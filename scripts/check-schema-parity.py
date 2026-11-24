#!/usr/bin/env python3
"""check-schema-parity.py — 声明式 Schema（contracts/schema/*.yaml）与产物 DDL 的结构一致性门禁。

为什么需要本门禁：`scripts/gen_schema.py` 只保证「生成器 → 产物」这一步可复现；
一旦有人手工改产物、或改生成器漏掉某类对象，YAML 与 DDL 就会静默漂移，
而 `check-sql-columns.py` 只反向校验 Go 仓储引用的列名，不校验声明侧。
本门禁把「声明 = 产物」变成可执行断言，解析器与生成器共用 `scripts/schema_common.py`，
避免两处解析逻辑各自演化。

用法：
    python3 scripts/check-schema-parity.py                 # 校验全部方言产物
    python3 scripts/check-schema-parity.py --file db/generated/postgresql/current.up.sql

输入：contracts/schema/*.yaml、db/generated/{postgresql,mysql,gaussdb,kingbasees}/current.up.sql
输出：PASS/FAIL + 表/列/约束/索引/RLS 差异
--check 行为：--file 只校验单个 DDL 文件
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from schema_common import (  # noqa: E402
    index_names_of,
    load_infra,
    load_tables,
    normalize_space,
    parse_ddl,
    pk_columns,
    short_name,
    strip_sql_comments,
)

import gen_schema  # noqa: E402

ROOT = Path(__file__).resolve().parent.parent
# 当前态全量渲染（每个方言一份）：与契约声明直接比对。
# 历史迁移链（db/postgresql/V{N}__*.sql）由 check-active-schema-history 与
# gen_schema --check 校验，不参与「声明 = 当前产物」的 parity。
PRODUCTS = {
    dialect: [f"db/generated/{dialect}/current.up.sql"]
    for dialect in ("postgresql", "mysql", "kingbasees", "gaussdb")
}

issues: list[str] = []


def fail(msg: str) -> None:
    issues.append(msg)


def expected_dialect_type(dialect: str, declared_type: str) -> str:
    """声明式逻辑类型 → 该方言产物中应出现的 DDL 类型。

    逻辑类型是方言中立的（`TEXT` = 无上界文本、`LONGTEXT` = 需 MEDIUMTEXT 承载的
    无上界内容）；映射表由生成器唯一持有，本门禁直接复用，避免两处各写一份。
    """
    logical = declared_type.strip()
    if dialect == "mysql":
        return gen_schema.mysql_type_of(logical)
    return gen_schema.PG_LOGICAL_ALIASES.get(logical.upper(), logical)


def type_matches(definition: str, want: str) -> bool:
    """产物列定义（列名之后的文本）的类型 token 是否等于期望类型。"""
    return definition == want or definition.startswith(want + " ")


def constraint_names(entry: dict) -> set[str]:
    names = set()
    for seg in entry["constraints"]:
        m = re.match(r'CONSTRAINT\s+"?([A-Za-z_][\w]*)"?', seg)
        if m:
            names.add(m.group(1))
    for m in re.finditer(r'CONSTRAINT\s+"?([A-Za-z_][\w]*)"?\s', " ".join(entry["constraints"])):
        names.add(m.group(1))
    return names


def inline_references(entry: dict) -> set[tuple[str, str]]:
    """从列定义里抽出内联 REFERENCES 的 (本表列, 目标表)。"""
    refs = set()
    for col, definition in entry["columns"].items():
        m = re.search(r"\bREFERENCES\s+([A-Za-z_][\w.]*)", definition)
        if m:
            refs.add((col, short_name(m.group(1))))
    return refs


def parse_ddl_policies(text: str) -> dict[str, set[str]]:
    """返回 {表: {策略名}}，兼容 DO $$ 守卫包裹的 CREATE POLICY。"""
    clean = strip_sql_comments(text).replace('"', "")
    out: dict[str, set[str]] = {}
    for m in re.finditer(
        r"CREATE\s+POLICY\s+([A-Za-z_][\w]*)\s+ON\s+(?:([A-Za-z_][\w]*)\.)?([A-Za-z_][\w]*)",
        clean,
    ):
        out.setdefault(m.group(3), set()).add(m.group(1))
    return out


def parse_ddl_rls(text: str) -> dict[str, bool]:
    clean = strip_sql_comments(text).replace('"', "")
    enabled: dict[str, bool] = {}
    for m in re.finditer(
        r"ALTER\s+TABLE\s+(?:([A-Za-z_][\w]*)\.)?([A-Za-z_][\w]*)\s+ENABLE\s+ROW\s+LEVEL\s+SECURITY",
        clean,
    ):
        enabled[m.group(2)] = True
    return enabled


def _find_balanced_paren(text: str, open_pattern: str) -> str | None:
    """在 text 里找 open_pattern 开头的最外层括号对，返回括号内的内容。

    解决的问题：PRIMARY KEY (`col1`, `col2`(255)) 里简单的 [^)]* 会在 (255) 的
    第一个右括号处截断，拿到残缺的列名列表。本函数用平衡括号栈匹配正确的边界。
    """
    m = re.search(open_pattern, text)
    if not m:
        return None
    start = m.end()  # open_pattern 匹配后的位置，即左括号之后
    depth = 1
    i = start
    while i < len(text) and depth > 0:
        if text[i] == "(":
            depth += 1
        elif text[i] == ")":
            depth -= 1
        i += 1
    if depth != 0:
        return None  # 括号不匹配
    return text[start : i - 1]  # 去掉最后一个右括号


def check_dialect(dialect: str, products: list[str]) -> None:
    tables = {short_name(t["table_name"]): t for t in load_tables()}
    infra = load_infra(dialect)

    # ── Dialect feature flags — 控制哪些 infra 项参与校验 ──
    # 与 internal/infra/dialect/*.go 的 Supports(Feature) 保持同态。
    # postgresql / gaussdb 全量；其他方言跳过 PG 专属特性（保守 false）。
    _pg_only = {"postgresql", "gaussdb"}
    _pg_skip = dialect not in _pg_only

    # MySQL 等方言的预期降级项：
    #   - gen_* 列 = 部分索引 predicate 降级为生成列（MySQL 无 WHERE 子句）
    #   - GIN 索引 = MySQL 无等价实现，gen_schema.py 里跳过并注释
    #   - 表达式索引 = metadata ->> 'key' 等操作符 MySQL 不支持
    _skip_col_prefixes = ("gen_",) if _pg_skip else ()

    def _is_expected_index_miss(tbl_decl: dict, idx_name: str) -> bool:
        """非 PG 方言下，GIN / 表达式索引跳过是预期行为。"""
        if not _pg_skip:
            return False
        for idx in tbl_decl.get("indexes") or []:
            if idx.get("name") == idx_name:
                if idx.get("using") == "gin":
                    return True
                # 表达式索引：列包含 ->、(、)、? 等操作符
                if any(any(op in c for op in ("->", "(", ")", "?"))
                       for c in (idx.get("columns") or [])):
                    return True
        return False

    for rel in products:
        path = ROOT / rel
        if not path.exists():
            fail(f"{rel}: 产物不存在")
            continue
        text = path.read_text(encoding="utf-8")
        ddl = {short_name(k): v for k, v in parse_ddl(text).items()}

        missing = sorted(set(tables) - set(ddl))
        extra = sorted(set(ddl) - set(tables))
        for name in missing:
            fail(f"{rel}: 表 {name} 在声明式 Schema 中存在但产物缺失")
        for name in extra:
            fail(f"{rel}: 表 {name} 出现在产物中但未声明于 contracts/schema/*.yaml")

        for name in sorted(set(tables) & set(ddl)):
            declared, actual = tables[name], ddl[name]

            dcols = {
                col["name"] if isinstance(col, dict) else col for col in declared["columns"]
            }
            acols = set(actual["columns"])
            for col in sorted(dcols - acols):
                fail(f"{rel}: {name}.{col} 已声明但产物缺失")
            for col in sorted(acols - dcols):
                # 非 PG 方言跳过 gen_* 生成列（MySQL 用生成列替代 PG 部分索引 predicate）
                if col.startswith(_skip_col_prefixes):
                    continue
                fail(f"{rel}: {name}.{col} 出现在产物中但未声明")

            # ── 列类型等价 ──
            # 仅比对列名存在性是不够的：MySQL 侧曾把逻辑 TEXT 映射为 VARCHAR(100)，
            # 566 个 TEXT 列中的 538 个在产物里被静默收窄成 100 字符，而列名一个不差，
            # 门禁长期无告警。此处断言「声明逻辑类型 → 方言期望类型 == 产物实际类型」。
            dtype_by_col = {
                (c["name"] if isinstance(c, dict) else c): (c.get("type") if isinstance(c, dict) else None)
                for c in declared["columns"]
            }
            for col in sorted(dcols & acols):
                dtype = dtype_by_col.get(col)
                if not dtype:
                    continue
                want_type = expected_dialect_type(dialect, dtype)
                definition = actual["columns"][col].strip()
                if not type_matches(definition, want_type):
                    got = definition.split()[0] if definition.split() else "(空)"
                    fail(
                        f"{rel}: {name}.{col} 类型不一致"
                        f"（声明 {dtype} → 期望 {want_type} / 产物 {got}）"
                    )

            declared_pk = pk_columns(declared)
            # PRIMARY KEY 列表可能嵌套前缀长度括号（TEXT(255)），简单 [^)]* 会在第一个
            # 右括号处截断。用平衡括号栈找最外层的括号对。
            actual_pk = _find_balanced_paren(" ".join(actual["constraints"]), r"PRIMARY\s+KEY\s*\(")
            if actual_pk is not None:
                got = [
                    re.sub(r"[`\"]", "", re.sub(r"\(\d+\)", "", c.strip()))
                    for c in actual_pk.split(",")
                ]
            else:
                got = [
                    c
                    for c, d in actual["columns"].items()
                    if "PRIMARY KEY" in d.upper()
                ]
            if sorted(declared_pk) != sorted(got):
                fail(f"{rel}: {name} 主键不一致（声明 {declared_pk} / 产物 {got}）")

            want = set()
            for c in declared.get("constraints") or []:
                if c.get("name"):
                    want.add(c["name"])
            for c in declared.get("fk_constraints") or []:
                if c.get("name"):
                    want.add(c["name"])
            have = constraint_names(actual) | set(actual.get("indexes") or {})
            for cname in sorted(want - have):
                fail(f"{rel}: {name} 约束 {cname} 已声明但产物缺失")

            declared_indexes = index_names_of(declared)
            actual_indexes = set(actual.get("indexes") or {}) | have
            for iname in sorted(declared_indexes - actual_indexes):
                # 非 PG 方言下，GIN / 表达式索引跳过是预期降级
                if _is_expected_index_miss(declared, iname):
                    continue
                fail(f"{rel}: {name} 索引 {iname} 已声明但产物缺失")

            # 内联 REFERENCES 只能指向已声明的表
            for col, target in inline_references(actual):
                if target not in ddl and target not in tables:
                    fail(f"{rel}: {name}.{col} 的 REFERENCES 指向未声明的表 {target}")

            # 唯一约束列集合必须与声明一致
            for c in declared.get("constraints") or []:
                if c.get("type") in ("unique", "unique_partial") and c.get("name"):
                    # 约束名必须整体匹配：`uq_x_code` 是 `uq_x_code_id` 的前缀，子串匹配会把
                    # 后者的列集当成前者的，于是新增一个名字带前缀关系的唯一约束必然误报
                    # 「缺少列」（qr_code_sessions.code 的唯一约束即因此被误判）。
                    name_re = re.compile(rf'CONSTRAINT\s+"?{re.escape(c["name"])}"?\s')
                    body = next((s for s in actual["constraints"] if name_re.search(s)), None)
                    if body:
                        cols = set(re.findall(r"[A-Za-z_][\w]*", body.split("(", 1)[1]))
                        for want_col in c.get("columns") or []:
                            if want_col not in cols:
                                fail(f"{rel}: {name} 约束 {c['name']} 缺少列 {want_col}")

        # ── RLS / Policies：按产物能力校验，不与运行期方言降级混用 ──
        # KingbaseES 当前 DDL 与 PG 一样包含 RLS，静态 parity 必须覆盖；
        # 运行期 FeatureRLS=false 所表达的支持边界由方言实现和集成测试另行验证。
        if dialect in {"postgresql", "gaussdb", "kingbasees"}:
            enabled = parse_ddl_rls(text)
            policies = parse_ddl_policies(text)
            for name, table in sorted(tables.items()):
                want_rls = bool(table.get("rls"))
                has_rls = name in enabled
                has_policy = "tenant_isolation" in policies.get(name, set())
                if want_rls and not has_rls:
                    fail(f"{rel}: {name} 声明 rls: true 但产物未 ENABLE ROW LEVEL SECURITY")
                if not want_rls and has_rls:
                    fail(f"{rel}: {name} 产物启用了 RLS 但未声明 rls: true")
                if want_rls != has_policy:
                    fail(
                        f"{rel}: {name} 的 tenant_isolation 策略与 rls 声明不一致"
                        f"（声明 {want_rls} / 策略 {has_policy}）"
                    )

            for table, names in sorted(policies.items()):
                for policy in sorted(names - {"tenant_isolation"}):
                    fail(f"{rel}: {table} 存在未声明的策略 {policy}")

        # ── Extensions / Functions / Triggers：仅 PostgreSQL/GaussDB ──
        if not _pg_skip:
            # 基础设施：扩展 / 函数 / 触发器
            for ext in infra.get("extensions") or []:
                name = ext["name"] if isinstance(ext, dict) else ext
                if not re.search(
                    rf"CREATE\s+EXTENSION\s+IF\s+NOT\s+EXISTS\s+{name}\b", text, re.I
                ):
                    fail(f"{rel}: 扩展 {name} 已声明但产物缺失")
            for fn in infra.get("functions") or []:
                if not re.search(
                    rf"CREATE\s+OR\s+REPLACE\s+FUNCTION\s+"
                    rf"(?:{infra.get('schema')}\.)?{fn['name']}\s*\(",
                    text,
                    re.I,
                ):
                    fail(f"{rel}: 函数 {fn['name']} 已声明但产物缺失")
            for trg in infra.get("triggers") or []:
                if not re.search(
                    rf"CREATE\s+OR\s+REPLACE\s+TRIGGER\s+{trg['name']}\b", text, re.I
                ):
                    fail(f"{rel}: 触发器 {trg['name']} 已声明但产物缺失")

            # 列默认值不得依赖库函数（ADR-022 §7）
            for m in re.finditer(r"^[ \t]+\w+[^\n]*DEFAULT\s+guliangiam\.", text, re.M):
                fail(f"{rel}: 列默认值依赖库函数 → {normalize_space(m.group(0))[:80]}")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--file", action="append", default=None)
    args = ap.parse_args()

    if args.file:
        check_dialect("postgresql", args.file)
    else:
        for dialect, products in PRODUCTS.items():
            check_dialect(dialect, products)

    if issues:
        print("Schema 一致性检查失败：", file=sys.stderr)
        for item in issues:
            print(f"  - {item}", file=sys.stderr)
        print(f"\n共 {len(issues)} 项不一致", file=sys.stderr)
        return 1

    tables = load_tables()
    print(
        "Schema 一致性检查通过：声明式 Schema 与产物 DDL 结构等价"
        f"（{len(tables)} 表，{len(PRODUCTS['postgresql'])} 个产物）"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

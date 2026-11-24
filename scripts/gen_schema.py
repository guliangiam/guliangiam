#!/usr/bin/env python3
"""Schema generator — contracts/schema/*.yaml → DDL 生成物（声明式 Schema 唯一权威输出）

唯一事实数据源：contracts/schema/*.yaml（表定义 + 00_infra.yaml 基础设施声明）。

生成物（全部由本脚本写出，CI 逐字节校验零差异）：
  1. db/generated/{dialect}/current.up.sql                 当前声明状态的全量渲染（parity/列检查输入）
  2. db/postgresql/V1__*.sql                              冻结基线（由 history/active/V1 渲染）
  3. db/{mysql,gaussdb,kingbasees}/V1__*.sql               方言当前态全量基线（不维护增量链）
  4. internal/bootstrap/schema_sql/V{N}__*.sql             go:embed 副本（V1 与 db/postgresql 逐字节一致）
  5. db/postgresql/V{N}__*.sql、db/rollback/V{N}__*.down.sql  V2+ 增量迁移与配对回滚（生成物）
  6. internal/domain/types/generated/schema_types.go        Go struct 类型（基于当前声明）
  7. web/src/shared/types/generated/schema-types.ts         TS 接口类型（基于当前声明）

迁移链模型（ADR-022）：V1 是冻结基线（= history/active/V1），V2+ 由
python3 scripts/gen_schema.py --version N 生成——以 history/active/V{N-1} 为 diff 起点，
产出 V{N} 正向+回滚，成功后冻结 history/active/V{N}。删表/删列变更被生成器拒绝；列改名
必须登记 contracts/schema/migrations/renames.yaml 才会生成 RENAME COLUMN（数据保留）。
禁止手写结构迁移。V1 不是手写冻结文件。

设计约束（由 scripts/check-*.sh 门禁强制）：
  * 幂等：CREATE TABLE/INDEX 全部 IF NOT EXISTS；约束、策略用 DO $$ 守卫
  * 无破坏性语句（DROP/TRUNCATE 只出现在回滚脚本中）
  * 命名：uq_/idx_/ck_/fk_ 前缀，全局索引名唯一；unique 索引必须落成 CREATE UNIQUE INDEX
  * 无 FK 前向引用：拓扑序不满足者延迟到文件尾部的 ALTER TABLE 块
  * RLS：rls: true 的表输出 ENABLE [+ FORCE] + 租户隔离策略（GUC 名唯一）
  * 隐式列：soft_delete → deleted_at；revision → revision/updated_at；所有表隐含 created_at
  * 主键：无方言函数默认值，ID 由应用层生成 UUIDv7（ADR-007 / ADR-022 §7）

Usage:
  python3 scripts/gen_schema.py --dialect postgresql --all
  python3 scripts/gen_schema.py --dialect postgresql --check
  python3 scripts/gen_schema.py --dialect postgresql --version 2   # 生成/校验下一个增量迁移

输入：contracts/schema/*.yaml（含 00_infra.yaml 基础设施声明）
输出：db/generated/{dialect}/current.up.sql、db/<dialect>/V1__*.sql、embed 副本、V2+ 迁移与回滚、schema_types.go、schema-types.ts
--check 行为：逐字节 diff 全部生成物（当前态/冻结基线/增量）与声明
消费方：check-generated-zero-diff.sh、verify-docs.sh(RUN_CODE_CHECKS=1, 四方言)、CI
"""
from __future__ import annotations

import argparse
import hashlib
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

import yaml  # noqa: E402

sys.path.insert(0, str(Path(__file__).resolve().parent))

from schema_common import (  # noqa: E402
    IMPLICIT_COLUMNS,
    SCHEMA_DIR,
    _apply_implicit_columns,
    index_names_of,
    load_infra,
    load_tables,
    normalize_space,
    pk_columns,
    short_name,
    split_top_level,
)

ROOT = Path(__file__).resolve().parent.parent
GENERATED_DIR = Path("db/generated")
DIALECTS = {"postgresql", "mysql", "kingbasees", "gaussdb"}

MYSQL_SCHEMA_PATH = "db/mysql/V1__guliangiam_schema_init.sql"
MYSQL_ROLLBACK_PATH = "db/mysql/V1__guliangiam_schema_init__down.sql"

RUNTIME_DDL = "internal/bootstrap/schema_sql/V1__guliangiam_schema_init.sql"
EMBED_DDL = "internal/bootstrap/schema_sql/V1__guliangiam_schema_init.sql"
ROLLBACK_DDL = "db/rollback/V1__guliangiam_schema_init__down.sql"  # forward-only 框架不自动生成；rollback 由部署流水线生成
ACTIVE_HISTORY_DIR = Path("contracts/schema/history/active")
# 当前状态的全量渲染产物（每个方言一份）：供 parity/列检查/文档消费的“当前态”。
# 与 db/<dialect>/V{N}__*.sql 的历史链分离：V1 是冻结基线，增量迁移只追加历史。
CURRENT_RENDER_NAME = "current.up.sql"
# 迁移无法用 diff 自动表达、但语义等价且必须数据保留的变更（列改名）显式登记在此。
SCHEMA_RENAMES = Path("contracts/schema/migrations/renames.yaml")

# 下游类型生成物路径
GO_SCHEMA_PATH = "internal/domain/types/generated/schema_types.go"
TS_SCHEMA_PATH = "web/src/shared/types/generated/schema-types.ts"

# 租户上下文 GUC —— 唯一事实来源（ADR-016 / ADR-022 §4）
GUC_ORG = "app.current_organization_id"
GUC_ROLE = "app.current_role"


# --------------------------------------------------------------------------- #
# 校验：生成前的声明式 Schema 自检                                            #
# --------------------------------------------------------------------------- #
class SchemaError(Exception):
    pass




# --------------------------------------------------------------------------- #
# 命名规范守卫：阻止生成器产出破坏蛇形命名的表名                              #
# --------------------------------------------------------------------------- #
def _warn_on_smushed_table_names(tables: list[dict]) -> None:
    """扫描 YAML table_name，警告非蛇形命名（captchaproviders / faceidproviders 等）。

    规则：schema.table_name 必须匹配 [a-z][a-z0-9_]* — 至少一个下划线或缩写边界。
    例外：单字表（records, sessions, entries）允许无下划线。
    """
    import sys as _sys
    pattern = re.compile(r'^[a-z][a-z0-9]*(_[a-z0-9]+)+$')  # 必须包含至少一个下划线
    single_word = re.compile(r'^[a-z]+$')
    bad = []
    for t in tables:
        full = t.get('table_name', '')
        short = full.split('.')[-1] if '.' in full else full
        if not pattern.match(short) and not single_word.match(short):
            bad.append(full)
    if bad:
        _sys.stderr.write(
            "⚠️  gen_schema.py 命名告警 — 以下 table_name 不符合蛇形命名规范：\n"
        )
        for b in bad:
            _sys.stderr.write(f"   • {b}\n")
        _sys.stderr.write(
            "   修复方法：在 contracts/schema/*.yaml 中把驼峰/连写改为蛇形（如 captchaproviders → captcha_providers）。\n"
        )


def index_column_errors(table: str, owner: str, col: str) -> list[str]:
    """校验索引/约束列项既可以是列名，也可以是括号表达式（表达式索引）。

    表达式侧只需两条可静态判定的性质：括号配平、且不以运算符结尾。缺失任一者，
    生成物就是一串无法解析的 SQL —— `columns: [(metadata ->>]` 曾产出
    `((metadata ->>))`，`ON DELETE SET_NULL` 同类：都只在真实执行时才暴露。
    """
    tok = str(col).strip()
    if not tok:
        return [f"{table}: {owner} 存在空的索引列项"]
    if tok.count("(") != tok.count(")"):
        return [f"{table}: {owner} 列项 {tok!r} 括号不配平"]
    if tok.rstrip().endswith(("->", "->>", "-", "+", "*", "/", "=", ",", "(")):
        return [f"{table}: {owner} 列项 {tok!r} 以运算符结尾，不是完整表达式"]
    return []


def validate(tables: list[dict], infra: dict) -> None:
    errors: list[str] = []
    tables_by_short = {short_name(t["table_name"]): t for t in tables}
    if len(tables_by_short) != len(tables):
        errors.append("存在重复的 table_name 声明")

    seen_index_names: dict[str, str] = {}
    for t in tables:
        full = t["table_name"]
        short = short_name(full)
        cols = [c["name"] for c in t.get("columns") or []]
        if not cols:
            errors.append(f"{full}: 未声明任何列")
        if len(cols) != len(set(cols)):
            errors.append(f"{full}: 存在重复列名")
        if not pk_columns(t):
            errors.append(f"{full}: 缺少 pk 声明")
        for col in pk_columns(t):
            if col not in cols:
                errors.append(f"{full}: pk {col} 未在 columns 中声明")
        for name in index_names_of(t):
            if not name.startswith(("idx_", "uq_")):
                errors.append(f"{full}: 索引/唯一约束名 {name} 前缀不合规")
            owner = seen_index_names.get(name)
            if owner and owner != full:
                errors.append(f"{full}: 索引名 {name} 与 {owner} 冲突（全局必须唯一）")
            seen_index_names[name] = full
        for idx in t.get("indexes") or []:
            for col in idx["columns"]:
                errors.extend(index_column_errors(full, idx["name"], col))
        for c in t.get("constraints") or []:
            if not c["name"].startswith(("uq_", "ck_")):
                errors.append(f"{full}: 约束名 {c['name']} 前缀不合规")
            if c.get("type") not in ("unique", "unique_partial", "check"):
                errors.append(f"{full}: 约束 {c['name']} 类型 {c.get('type')} 不支持")
            for col in c.get("columns") or []:
                errors.extend(index_column_errors(full, c["name"], col))
        for c in t.get("columns") or []:
            default = str(c.get("default") or "")
            if "guliangiam." in default:
                errors.append(f"{full}.{c['name']}: 默认值不得引用方言函数（{default}）")
        for fk in t.get("fk_constraints") or []:
            ref = short_name(fk["ref_table"])
            if ref not in tables_by_short:
                errors.append(f"{full}: 外键引用未声明的表 {fk['ref_table']}")
            if not fk.get("name", "").startswith("fk_") and fk.get("name"):
                errors.append(f"{full}: 外键名 {fk['name']} 前缀不合规")
            for kind in ("on_delete", "on_update"):
                action = fk.get(kind)
                if action and action not in FK_ACTIONS:
                    errors.append(
                        f"{full}.{fk.get('name') or fk_columns(fk)}: {kind} 记号 {action!r} 非法，"
                        f"规范记号：{'/'.join(sorted(FK_ACTIONS))}（写 `SET_NULL` 而非 `SET NULL`）"
                    )
        if t.get("rls"):
            if "organization_id" not in cols and short != "organizations" and not t.get("rls_predicate"):
                errors.append(f"{full}: 声明 RLS 但缺少 organization_id 列且未提供 rls_predicate")

    for fn in infra.get("functions") or []:
        if fn.get("phase") not in ("pre", "post"):
            errors.append(f"函数 {fn['name']}: phase 必须为 pre/post")
    if errors:
        raise SchemaError("\n".join("  - " + e for e in errors))


# --------------------------------------------------------------------------- #
# DDL 片段渲染                                                                #
# --------------------------------------------------------------------------- #
# 引用动作在声明层是**语义记号**，不是方言语法：记号一律用下划线形式，由生成器翻译成
# 目标方言的正确写词。历史上这里把 YAML 值原样内插，`SET_NULL` 于是被写成
# `ON DELETE SET_NULL` —— PostgreSQL 只接受两词形式 `SET NULL`，79 张表的建表脚本因此
# 从未在真实库上执行过（"生成物零差异"只证明字节稳定，不证明可执行）。
FK_ACTIONS = {
    "CASCADE": "CASCADE",
    "RESTRICT": "RESTRICT",
    "NO_ACTION": "NO ACTION",
    "SET_NULL": "SET NULL",
    "SET_DEFAULT": "SET DEFAULT",
}


# --------------------------------------------------------------------------- #
# 下游类型生成辅助（contracts/schema → Go / TypeScript）                             #
# --------------------------------------------------------------------------- #
PG_TYPE_TO_GO = {
    "UUID": "string", "TEXT": "string", "LONGTEXT": "string", "CIDR": "string", "INET": "string", "VARCHAR": "string",
    "BIGINT": "int64", "INT": "int", "INTEGER": "int", "BOOLEAN": "bool",
    "TIMESTAMPTZ": "time.Time", "DATE": "time.Time", "JSONB": "string", "BYTEA": "[]byte", "DECIMAL": "float64",
}
PG_TYPE_TO_TS = {
    "UUID": "string", "TEXT": "string", "LONGTEXT": "string", "CIDR": "string", "INET": "string", "VARCHAR": "string",
    "BIGINT": "number", "INT": "number", "INTEGER": "number", "DECIMAL": "number", "BOOLEAN": "boolean",
    "TIMESTAMPTZ": "string", "DATE": "string", "JSONB": "unknown", "BYTEA": "Uint8Array",
}


def _normalize_type(pg_type: str) -> tuple[str, bool]:
    t = pg_type.strip()
    is_array = False
    if t.endswith("[]"):
        is_array = True
        t = t[:-2]
    paren = t.find("(")
    if paren > 0:
        t = t[:paren]
    return t.upper(), is_array


def _go_type(col: dict) -> str:
    base, is_array = _normalize_type(col["type"])
    mapped = PG_TYPE_TO_GO.get(base, "string")
    if is_array:
        mapped = f"[]{mapped}"
    if col.get("nullable", True):
        if mapped.startswith("[]") or mapped in ("time.Time",):
            mapped = f"*{mapped}"
        else:
            mapped = f"*{mapped}"
    return mapped


def _ts_type(col: dict) -> str:
    base, is_array = _normalize_type(col["type"])
    mapped = PG_TYPE_TO_TS.get(base, "string")
    if is_array:
        mapped = f"{mapped}[]"
    if col.get("nullable", True):
        mapped = f"{mapped} | null"
    return mapped


def _snake_to_pascal(name: str) -> str:
    return "".join(p.capitalize() for p in name.split("_"))


# Go 惯用初始化缩写（golint / staticcheck ST1003）：导出标识符里的缩写必须全大写，
# 如 `id`→`ID`、`url`→`URL`、`ip`→`IP`。生成物是 Go 类型的唯一来源，若不在此归一化，
# 每个新增 `*_id` 列都会重新引入 `Id`/`Url` 这类非惯用名。
# TS 侧刻意不使用本表：Google TypeScript 风格指南要求避免全大写缩写（`ApiKeys` 而非
# `APIKeys`），两语言惯例不同，故 `render_ts_schema_types` 继续用 `_snake_to_pascal`。
_GO_INITIALISMS = frozenset({
    # golint 标准表
    "acl", "api", "ascii", "cpu", "css", "dns", "eof", "guid", "html", "http",
    "https", "id", "ip", "json", "lhs", "qps", "ram", "rhs", "rpc", "sla",
    "smtp", "sql", "ssh", "tcp", "tls", "ttl", "udp", "ui", "uid", "uri",
    "url", "utf8", "uuid", "vm", "xml", "xsrf", "xss",
    # IAM 领域全大写缩写（OAuth / DPoP / IdP 这类混合大小写拼写不纳入）
    "abac", "arn", "ca", "cors", "crl", "csrf", "der", "jwk", "jwks", "jwt",
    "ldap", "mfa", "oidc", "otp", "pem", "rbac", "saml", "scim", "sms",
    "sso", "totp",
})


def _go_exported_name(name: str) -> str:
    """snake_case → Go 导出标识符，缩写按 Go 惯例全大写（`organization_id`→`OrganizationID`）。"""
    return "".join(
        p.upper() if p.lower() in _GO_INITIALISMS else p.capitalize()
        for p in name.split("_")
    )


def _col_signature(col: dict) -> tuple:
    return (col.get("type", ""), col.get("nullable", True), col.get("default", None))


# 逻辑类型是方言中立的。`LONGTEXT` 表达「无上界内容」（用户撰写的 HTML/CSS/JS/法律文本），
# 在 PostgreSQL / GaussDB / KingBaseES 上仍是 TEXT，只有 MySQL 需要升到 MEDIUMTEXT。
# 逻辑类型一旦引入就必须在每个方言后端都有确定映射，否则会有一个方言把逻辑类型名
# 当 DDL 类型直接吐出去（PostgreSQL 不认 `LONGTEXT`，建表即失败）。
PG_LOGICAL_ALIASES = {"LONGTEXT": "TEXT"}


def pg_column_type(col: dict) -> str:
    """逻辑类型 → PostgreSQL 物理类型；增量迁移与全量渲染必须共用同一映射。"""
    return PG_LOGICAL_ALIASES.get(col["type"].strip().upper(), col["type"])


def render_column(col: dict) -> str:
    logical = pg_column_type(col)
    parts = [col["name"], logical]
    if not col.get("nullable", True):
        parts.append("NOT NULL")
    if col.get("default") is not None:
        parts.append(f"DEFAULT {col['default']}")
    return " ".join(parts)


def resolve_ref_columns(fk: dict, tables_by_short: dict[str, dict]) -> list[str]:
    if fk.get("ref_columns"):
        return list(fk["ref_columns"])
    if fk.get("ref_column"):
        return [fk["ref_column"]]
    if fk.get("columns"):
        return list(fk["columns"])
    ref = tables_by_short[short_name(fk["ref_table"])]
    return pk_columns(ref) or ["id"]


def fk_columns(fk: dict) -> list[str]:
    if fk.get("columns"):
        return list(fk["columns"])
    return [fk["column"]]


def fk_name(fk: dict, table_short: str) -> str:
    if fk.get("name"):
        return fk["name"]
    cols = "_".join(fk_columns(fk))
    name = f"fk_{table_short}_{cols}_{short_name(fk['ref_table'])}"
    # MySQL identifiers are capped at 64 characters (PostgreSQL at 63). Generated
    # FK names must fit both dialects without relying on silent truncation, which
    # would break rollback/drop order and dialect parity. Keep a deterministic
    # hash suffix so shortened names stay unique.
    if len(name) > 63:
        digest = hashlib.sha1(name.encode("utf-8")).hexdigest()[:8]
        name = name[:54] + "_" + digest
    return name


def render_fk_clause(fk: dict, tables_by_short: dict[str, dict]) -> str:
    full = fk.get("ref_table", "?")
    cols = ", ".join(fk_columns(fk))
    ref_cols = ", ".join(resolve_ref_columns(fk, tables_by_short))
    clause = f"FOREIGN KEY ({cols}) REFERENCES {fk['ref_table']}({ref_cols})"
    if fk.get("on_delete"):
        clause += f" ON DELETE {FK_ACTIONS[fk['on_delete']]}"
    if fk.get("on_update"):
        clause += f" ON UPDATE {FK_ACTIONS[fk['on_update']]}"
    return clause


def render_table(table: dict, tables_by_short: dict[str, dict], rls_force: bool) -> tuple[list[str], list[str]]:
    """渲染单表 DDL；返回 (语句块, 需延迟到文件尾的外键语句)。"""
    full = table["table_name"]
    short = short_name(full)
    pk = pk_columns(table)
    deferred: list[str] = []
    lines: list[str] = [f"CREATE TABLE IF NOT EXISTS {full} ("]

    defs: list[str] = []
    for col in table.get("columns") or []:
        cd = render_column(col)
        if pk == [col["name"]]:
            cd += " PRIMARY KEY"
        defs.append("    " + cd)

    if len(pk) > 1:
        defs.append(f"    PRIMARY KEY ({', '.join(pk)})")

    for c in table.get("constraints") or []:
        if c.get("type") == "check":
            defs.append(f"    CONSTRAINT {c['name']} CHECK ({normalize_space(c['expr'])})")
        elif c.get("type") == "unique":
            defs.append(f"    CONSTRAINT {c['name']} UNIQUE ({', '.join(c['columns'])})")

    # FK 目标表在同一文件中更晚出现 → 延迟到尾部（避免前向引用）
    ordered_shorts = [short_name(t["table_name"]) for t in tables_by_short["_order"]]
    here = ordered_shorts.index(short)
    for fk in table.get("fk_constraints") or []:
        target = short_name(fk["ref_table"])
        clause = render_fk_clause(fk, tables_by_short)
        name = fk_name(fk, short)
        if ordered_shorts.index(target) > here:
            deferred.append(f"ALTER TABLE {full}\n    ADD CONSTRAINT {name}\n    {clause};")
        else:
            defs.append(f"    CONSTRAINT {name} {clause}")

    lines.append(",\n".join(defs))

    # PARTITION BY（ADR-011）— 只有 schema YAML 显式声明 partition_by 才生成。
    # 示例：partition_by: {column: created_at, granularity: month}
    # 对现有表无影响（零差异门禁安全）。
    pb = table.get("partition_by")
    if pb and isinstance(pb, dict) and pb.get("column"):
        pb_col = pb["column"]
        lines.append(f") PARTITION BY RANGE ({pb_col});")
    else:
        lines.append(");")

    for c in table.get("constraints") or []:
        if c.get("type") == "unique_partial":
            where = f" WHERE {normalize_space(c['predicate'])}" if c.get("predicate") else ""
            lines.append(
                f"CREATE UNIQUE INDEX IF NOT EXISTS {c['name']} ON {full} "
                f"({', '.join(c['columns'])}){where};"
            )
    for idx in table.get("indexes") or []:
        using = f" USING {idx['using']}" if idx.get("using") else ""
        where = f" WHERE {normalize_space(idx['predicate'])}" if idx.get("predicate") else ""
        # unique 必须落成 CREATE UNIQUE INDEX：忽略该标志会静默把唯一约束降级为普通索引，
        # 数据库层不再阻止重复值（check-db-conventions 的门禁命名检查即因此报错）。
        unique = "UNIQUE " if idx.get("unique") else ""
        lines.append(
            f"CREATE {unique}INDEX IF NOT EXISTS {idx['name']} ON {full}"
            f"{using} ({', '.join(idx['columns'])}){where};"
        )

    if table.get("comment"):
        cmt = table["comment"].replace("'", "''")
        lines.append(f"COMMENT ON TABLE {full} IS '{cmt}';")
    # COMMENT ON COLUMN 只对 YAML 显式定义的列生成，跳过 _apply_implicit_columns
    # 追加的隐式列（created_at / deleted_at / updated_at / revision）。
    # 原因：V1 baseline 用 CREATE TABLE IF NOT EXISTS 做幂等 replay，
    # 已有旧表缺少隐式列时，COMMENT ON COLUMN 会触发 42703。
    implicit_names = set(IMPLICIT_COLUMNS.keys())
    for col in table.get("columns") or []:
        if col.get("comment") and col.get("name") not in implicit_names:
            cmt = col["comment"].replace("'", "''")
            lines.append(f"COMMENT ON COLUMN {full}.{col['name']} IS '{cmt}';")

    if table.get("rls") or table.get("rls_force"):
        cols = {c["name"] for c in table.get("columns") or []}
        lines.append(f"ALTER TABLE {full} ENABLE ROW LEVEL SECURITY;")
        if rls_force or table.get("rls_force"):
            lines.append(f"ALTER TABLE {full} FORCE ROW LEVEL SECURITY;")
        # 策略名按表唯一：统一为 tenant_isolation（全库命名一致，ADR-016）
        policy = "tenant_isolation"
        if table.get("rls_predicate"):
            # 间接关联租户的表（如 sessions 通过 user_id 关联 users.organization_id）
            # 使用自定义谓词，避免在表中冗余 organization_id 列
            predicate = normalize_space(table["rls_predicate"])
        else:
            tenant_col = "organization_id" if "organization_id" in cols else "id"
            predicate = (
                f"guliangiam.is_super_admin() OR {tenant_col} = guliangiam.current_organization_id()"
            )
        lines.append(
            "DO $$\n"
            "BEGIN\n"
            "    IF NOT EXISTS (\n"
            "        SELECT 1 FROM pg_policies\n"
            "        WHERE schemaname = 'guliangiam'\n"
            f"          AND tablename = '{short}'\n"
            f"          AND policyname = '{policy}'\n"
            "    ) THEN\n"
            f"        CREATE POLICY {policy} ON {full}\n"
            f"            USING ({predicate})\n"
            f"            WITH CHECK ({predicate});\n"
            "    END IF;\n"
            "END $$;"
        )
    return lines, deferred


def render_function(fn: dict) -> str:
    volatility = f" {fn['volatility']}" if fn.get("volatility") else ""
    body = "\n".join(fn["body"].rstrip("\n").split("\n"))
    return (
        f"CREATE OR REPLACE FUNCTION {fn['name']}{fn['signature']}\n"
        f"    LANGUAGE {fn['language']}{volatility} AS $$\n"
        f"{body}\n"
        "$$;"
    )


def render_role(role: dict) -> str:
    """渲染角色创建与权限授予。"""
    name = role["name"]
    opts = []
    if role.get("login"):
        opts.append("LOGIN")
    opt_str = " ".join(opts) if opts else "NOLOGIN"
    out = [
        f"DO $$\n"
        f"BEGIN\n"
        f"    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '{name}') THEN\n"
        f"        CREATE ROLE {name} {opt_str};\n"
        f"    END IF;\n"
        f"END $$;"
    ]
    for grant in role.get("grants") or []:
        priv = grant["privilege"]
        obj = grant["object"]
        target = grant["target"]
        if obj == "SCHEMA":
            out.append(f"GRANT {priv} ON SCHEMA {target} TO {name};")
        elif obj == "ALL_TABLES":
            out.append(f"GRANT {priv} ON ALL TABLES IN SCHEMA {target} TO {name};")
        elif obj == "ALL_FUNCTIONS":
            out.append(f"GRANT {priv} ON ALL FUNCTIONS IN SCHEMA {target} TO {name};")
        elif obj == "ALL_SEQUENCES":
            out.append(f"GRANT {priv} ON ALL SEQUENCES IN SCHEMA {target} TO {name};")
    return "\n".join(out)


def render_trigger(trg: dict) -> str:
    events = " OR ".join(trg["events"])
    return (
        f"CREATE OR REPLACE TRIGGER {trg['name']}\n"
        f"    {trg['timing']} {events} ON {trg['table']}\n"
        f"    FOR EACH {trg.get('for_each', 'ROW')} EXECUTE FUNCTION {trg['function']}();"
    )


def rollback_table_order(tables: list[dict]) -> list[dict]:
    """按「可安全删除」的顺序返回表：先删没有任何保留表引用它的表。

    建表顺序**不能取反**使用：生成器把前向引用的外键延迟到文件尾用 `ALTER TABLE` 补上，
    于是 `tables` 的拓扑序只描述建表那一刻的引用。反过来删会在真实库上撞上后加的约束
    （`DROP TABLE guliangiam.enforcers` 被 `fk_casbin_rules_enforcer_id_enforcers` 拒绝），
    即 ADR-022 结构门禁 3「回滚成功」事实上从未成立。

    这里对**完整**外键图反复剥离无入边表，等价于逆拓扑排序；自引用忽略，剩余表在存在
    环时按建表逆序兜底（当前声明集无环，剥离必然收敛）。
    """
    remaining = {short_name(t["table_name"]): t for t in tables}
    ordered: list[dict] = []
    while remaining:
        # 被其他保留表引用的表不能先删：先算出「入边」集合，再剥离无入边的表。
        referenced = {
            short_name(fk["ref_table"])
            for t in remaining.values()
            for fk in t.get("fk_constraints") or []
            if short_name(fk["ref_table"]) != short_name(t["table_name"])
        }
        droppable = [name for name in remaining if name not in referenced]
        if not droppable:
            ordered.extend(reversed(list(remaining.values())))
            break
        for name in droppable:
            ordered.append(remaining.pop(name))
    return ordered


def render_rollback(tables: list[dict], infra: dict) -> str:
    out = [
        "-- Auto-generated by scripts/gen_schema.py — DO NOT EDIT",
        "-- 回滚迁移：仅用于开发环境重置 / 灾难恢复演练；生产环境禁止自动执行。",
        "-- 对应 db/postgresql/V1__guliangiam_schema_init.sql",
        "",
    ]
    for trg in reversed(infra.get("triggers") or []):
        out.append(f"DROP TRIGGER IF EXISTS {trg['name']} ON {trg['table']};")
    # 表必须先于函数删除：租户 RLS 策略挂在表上并引用 guliangiam.is_super_admin() 等
    # 函数，函数先删会被 DROP FUNCTION 以「other objects depend on it」拒绝 —— 回滚脚本
    # 因此在真实库上根本执行不完（此处顺序即 ADR-022 结构门禁 3 的「回滚成功」）。
    out.append("")
    for t in rollback_table_order(tables):
        out.append(f"DROP TABLE IF EXISTS {t['table_name']};")
    out.append("")
    for fn in reversed(infra.get("functions") or []):
        out.append(f"DROP FUNCTION IF EXISTS {fn['name']}{fn['signature'].split(' RETURNS')[0]};")
    out.append("")
    # 必须以 CASCADE 收尾：schema 内除了声明式对象，还有执行器自建的迁移账本表（生成器无法声明它）。
    # 用 RESTRICT 时回滚在真实库上会以「schema guliangiam is not empty」失败，即回滚链名存实亡。
    out.append("-- CASCADE 仅用于移除执行器自建的迁移账本表；除此之外 schema 内不应存在任何其他对象。")
    out.append("DROP SCHEMA IF EXISTS guliangiam CASCADE;")
    return "\n".join(out) + "\n"


# --------------------------------------------------------------------------- #
# 主生成                                                                      #
# --------------------------------------------------------------------------- #
def generate_postgresql(tables: list[dict], infra: dict, dialect: str) -> tuple[str, str]:
    schema = infra.get("schema", "guliangiam")
    rls_force = bool((infra.get("rls") or {}).get("force"))
    tables_by_short = {short_name(t["table_name"]): t for t in tables}
    tables_by_short["_order"] = tables
    short_list = [short_name(t["table_name"]) for t in tables]

    out: list[str] = [
        "-- ============================================================",
        "-- GuliangIAM PostgreSQL Schema (V1)",
        "-- 生成物 — 请勿手工修改；唯一事实数据源为 contracts/schema/*.yaml，",
        "-- 由 scripts/gen_schema.py 生成，CI 通过 check-generated-zero-diff.sh 校验零差异。",
        "-- ============================================================",
        "",
        "-- ============================================================",
        "-- 1. 扩展与 Schema",
        "-- ============================================================",
        "",
    ]
    for ext in infra.get("extensions") or []:
        if ext.get("required", False):
            out.append(f"CREATE EXTENSION IF NOT EXISTS {ext['name']};")
        else:
            out.append(
                f"-- {ext['name']} 需 shared_preload_libraries；不可用时静默跳过\n"
                "DO $$ BEGIN\n"
                f"    CREATE EXTENSION IF NOT EXISTS {ext['name']};\n"
                "EXCEPTION WHEN OTHERS THEN\n"
                f"    RAISE NOTICE '{ext['name']} extension not available, skipping';\n"
                "END $$;"
            )
    out += ["", f"CREATE SCHEMA IF NOT EXISTS {schema};", ""]

    roles = infra.get("roles") or []
    if roles:
        out += [
            "-- ============================================================",
            "-- 1.5 应用角色与权限（RLS 强制隔离配套）",
            "-- ============================================================",
            "",
        ]
        out += [render_role(r) + "\n" for r in roles]

    pre_fns = [f for f in infra.get("functions") or [] if f.get("phase") == "pre"]
    post_fns = [f for f in infra.get("functions") or [] if f.get("phase") == "post"]

    if pre_fns:
        out += [
            "-- ============================================================",
            "-- 2. 基础函数（不依赖表结构）",
            "-- ============================================================",
            "",
        ]
        out += [render_function(f) + "\n" for f in pre_fns]

    out += [
        "-- ============================================================",
        "-- 3. 表结构（拓扑序：被引用表先创建）",
        "-- ============================================================",
        "",
    ]
    deferred_blocks: list[str] = []
    current_source = None
    for t in tables:
        if t["_source"] != current_source:
            current_source = t["_source"]
            out += [
                "",
                "-- ------------------------------------------------------------",
                f"-- 分组：{current_source}",
                "-- ------------------------------------------------------------",
                "",
            ]
        block, deferred = render_table(t, tables_by_short, rls_force)
        out += block + [""]
        deferred_blocks += deferred

    out += [
        "-- ============================================================",
        "-- 4. 依赖表结构的函数与触发器",
        "-- ============================================================",
        "",
    ]
    out += [render_function(f) + "\n" for f in post_fns]
    out += [render_trigger(t) + "\n" for t in infra.get("triggers") or []]

    if deferred_blocks:
        out += [
            "-- ============================================================",
            "-- 5. 延迟外键（目标表在声明序中更晚出现，避免 FK 前向引用）",
            "-- ============================================================",
            "",
            "DO $$",
            "BEGIN",
        ]
        body = []
        for stmt in deferred_blocks:
            name = stmt.split("ADD CONSTRAINT ")[1].split("\n")[0]
            table = stmt.split("ALTER TABLE ")[1].split("\n")[0]
            body.append(
                "    IF NOT EXISTS (\n"
                "        SELECT 1\n"
                "        FROM pg_constraint\n"
                f"        WHERE conname = '{name}'\n"
                f"          AND conrelid = '{table}'::regclass\n"
                "    ) THEN\n"
                + "\n".join("        " + ln for ln in stmt.split("\n"))
                + "\n    END IF;\n"
            )
        out.append("\n".join(body).rstrip("\n"))
        out.append("END $$;")

    up = "\n".join(out).rstrip("\n") + "\n"
    return up, render_rollback(tables, infra)


# =========================================================================== #
# MySQL 8.x 方言渲染后端（ADR-023 第二基线）                                 #
#                                                                             #
# 与 PostgreSQL 产物语义等价，但用 MySQL 可执行的 DDL 语法：                 #
#   - 类型映射（INET→VARCHAR(45), JSONB→JSON, UUID→CHAR(36), 数组→JSON）     #
#   - 软删除活跃唯一性（MySQL 无部分索引 WHERE，用生成列 is_active + 普通    #
#     UNIQUE INDEX 实现：IF(deleted_at IS NULL, 1, NULL)）                   #
#   - 无 RLS（MySQL 不支持；应用层 WHERE organization_id 过滤，ADR-023 决策6）#
#   - 延迟外键用独立 ALTER TABLE + 反查约束名（条件化执行，避免二次 DDL 失败）#
#   - COMMENT 内嵌到列/表定义（MySQL 无 COMMENT ON 语法）                    #
#   - uuidv7 默认值已在 V1 产物中移除（ADR-007 应用层生成，PostgreSQL 也没有）#
# =========================================================================== #
MYSQL_TYPE_MAP = {
    "UUID": "CHAR(36)",
    # 逻辑 TEXT 是「无上界文本」；MySQL 侧映射到 TEXT（65535 字节）而非有界 VARCHAR——
    # 曾经这里写的是 VARCHAR(100)，于是声明式 Schema 里 566 个 TEXT 列中的 538 个在
    # MySQL 产物里被静默收窄成 100 字符：声明与产物语义不等价，超长值在严格模式下报
    # 错、在非严格模式下被截断。真正无上界的内容（用户撰写的 HTML/CSS/JS/法律文本）
    # 用逻辑类型 LONGTEXT，MySQL 映射到 MEDIUMTEXT（16MB）。
    "TEXT": "TEXT",
    "LONGTEXT": "MEDIUMTEXT",
    "VARCHAR": "VARCHAR",
    "BIGINT": "BIGINT",
    "INT": "INT",
    "INTEGER": "INT",
    "BOOLEAN": "TINYINT(1)",
    "TIMESTAMPTZ": "TIMESTAMP",
    "TIMESTAMP": "TIMESTAMP",
    "DATE": "DATE",
    "JSONB": "JSON",
    "JSON": "JSON",
    "BYTEA": "BLOB",
    "DECIMAL": "DECIMAL",
    "INET": "VARCHAR(45)",
    "CIDR": "VARCHAR(45)",
    "CHAR": "CHAR",
    "SMALLINT": "SMALLINT",
}


def mysql_type_of(pg_type: str) -> str:
    t = pg_type.strip()
    is_array = t.endswith("[]")
    if is_array:
        t = t[:-2]
        return "JSON"  # MySQL 无原生数组，统一映射到 JSON 存数组
    paren = t.find("(")
    if paren > 0:
        base = t[:paren].upper()
        rest = t[paren:]
    else:
        base = t.upper()
        rest = ""
    mapped = MYSQL_TYPE_MAP.get(base, base)
    return mapped + rest


# TEXT/BLOB 家族类型集合 — MySQL 上这些列出现在 UNIQUE/PK/INDEX 中时必须指定前缀长度。
_MYSQL_TEXT_FAMILY = {"TEXT", "MEDIUMTEXT", "LONGTEXT", "JSON", "BLOB"}


def _mysql_is_text_family(col: dict) -> bool:
    """判断列在 MySQL 上是否属于 TEXT 家族（需要前缀长度）。"""
    ct = col.get("type") if isinstance(col, dict) else None
    if ct is None:
        return False
    return mysql_type_of(ct) in _MYSQL_TEXT_FAMILY


def _mysql_col_ref(col_name: str, cols_by_name: dict, dialect: str = "mysql") -> str:
    """返回列名在约束/索引中的引用形式。

    PostgreSQL 可直接用列名（UNIQUE (col1, col2)），但 MySQL 对 TEXT 家族列必须
    指定前缀长度，否则报错 ERROR 1170: BLOB/TEXT column ... used in key
    specification without a key length。

    典型调用方：PRIMARY KEY 列表、UNIQUE 约束、CREATE INDEX 列列表。
    """
    if dialect != "mysql":
        return f"`{col_name}`"
    col_def = cols_by_name.get(col_name)
    if col_def is None:
        return f"`{col_name}`"
    declared_type = col_def.get("type") if isinstance(col_def, dict) else None
    if declared_type and mysql_type_of(declared_type) in _MYSQL_TEXT_FAMILY:
        return f"`{col_name}`(255)"
    return f"`{col_name}`"


def _mysql_col_list(col_names: list[str], cols_by_name: dict) -> str:
    """把列名列表转为带反引号的约束/索引列引用串（逗号分隔）。

    TEXT 家族列自动加 (255) 前缀；当组合列超过 InnoDB utf8mb4 3072 字节限制时，
    自动给 VARCHAR 列加 (64) 前缀（Casbin UNIQUE 8×VARCHAR(255) 典型场景）。
    """
    MAX_INDEX_BYTES = 3072  # InnoDB utf8mb4 默认

    refs = []  # (col_ref_no_prefix, is_varchar, is_text, prefix_len)
    for c in col_names:
        col_def = cols_by_name.get(c)
        if col_def is None:
            refs.append((c, False, False, 255))
            continue
        declared = col_def.get("type", "")
        mt = mysql_type_of(declared)
        if mt in _MYSQL_TEXT_FAMILY:
            refs.append((c, False, True, 255))
        elif mt.startswith("VARCHAR"):
            m = re.search(r'VARCHAR\((\d+)\)', mt)
            refs.append((c, True, False, int(m.group(1)) if m else 255))
        elif mt.startswith("CHAR"):
            m = re.search(r'CHAR\((\d+)\)', mt)
            refs.append((c, False, False, int(m.group(1)) if m else 36))
        else:
            refs.append((c, False, False, 0))  # INT/BIGINT/TIMESTAMP 等固定宽度

    total = sum(r[3] * 4 for r in refs)  # utf8mb4: 每字符 4 字节
    need_shrink = total > MAX_INDEX_BYTES

    parts = []
    if need_shrink:
        # 自动计算每列前缀长度：3072 字节 ÷ 列数 ÷ 4 (utf8mb4)
        # 固定宽度列（INT/BIGINT）不计入前缀分配
        n_shrinkable = sum(1 for r in refs if r[3] > 0)
        if n_shrinkable > 0:
            safe_prefix = MAX_INDEX_BYTES // (n_shrinkable * 4)
            safe_prefix = max(safe_prefix, 16)  # 保底 16 字符
        else:
            safe_prefix = 191
        for name, is_vc, is_txt, plen in refs:
            if plen == 0:
                parts.append(f"`{name}`")
            else:
                prefix = min(plen, safe_prefix)
                parts.append(f"`{name}`({prefix})")
    else:
        for name, is_vc, is_txt, plen in refs:
            if is_txt:
                parts.append(f"`{name}`(255)")
            else:
                parts.append(f"`{name}`")
    return ", ".join(parts)


def _mysql_index_if_not_exists(tbl: str, idx_name: str, cols: str, unique: bool = False) -> list[str]:
    """MySQL 无 CREATE INDEX IF NOT EXISTS，用 PREPARE/EXECUTE 条件化执行。

    调用方传入已格式化好的 cols 字符串（由 _mysql_col_list 生成，带反引号和 TEXT 前缀）。
    """
    tbl_ref = f"`{tbl}`"
    idx_ref = f"`{idx_name}`"
    kind = "UNIQUE INDEX" if unique else "INDEX"
    safe_sql = f"CREATE {kind} {idx_ref} ON {tbl_ref} ({cols})"
    return [
        f"SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS "
        f"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '{tbl}' AND INDEX_NAME = '{idx_name}' LIMIT 1);",
        f"SET @_sql := IF(@_idx_exists IS NULL, '{safe_sql}', 'SELECT 1');",
        f"PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;",
    ]


def _mysql_unique_index_if_not_exists(tbl: str, idx_name: str, cols: str) -> list[str]:
    """同 _mysql_index_if_not_exists 的 UNIQUE 版。cols 必须由 _mysql_col_list 预格式化。"""
    tbl_ref = f"`{tbl}`"
    idx_ref = f"`{idx_name}`"
    safe_sql = f"CREATE UNIQUE INDEX {idx_ref} ON {tbl_ref} ({cols})"
    return [
        f"SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS "
        f"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '{tbl}' AND INDEX_NAME = '{idx_name}' LIMIT 1);",
        f"SET @_sql := IF(@_idx_exists IS NULL, '{safe_sql}', 'SELECT 1');",
        f"PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;",
    ]


def render_column_mysql(col: dict) -> str:
    mysql_type = mysql_type_of(col["type"])
    # MySQL 保留字转义：所有列名用反引号包裹
    col_name = f"`{col['name']}`"
    parts = [col_name, mysql_type]
    if not col.get("nullable", True):
        parts.append("NOT NULL")
    has_default = col.get("default") is not None
    # MySQL 严格模式不允许 BLOB/TEXT/JSON 系列类型有 DEFAULT。声明式为这些列写的默认值
    # 在 MySQL 上无法表达，生成器只能去掉——去掉是「响亮地少一个默认值」，比换成有界
    # VARCHAR 换取默认值安全：后者会静默改写列语义。
    if has_default and mysql_type in ("JSON", "LONGTEXT", "MEDIUMTEXT", "TEXT", "BLOB"):
        has_default = False
    # MySQL TIMESTAMP DEFAULT 不允许算术表达式（如 now() + INTERVAL），去掉
    if has_default and mysql_type.startswith("TIMESTAMP"):
        raw = str(col["default"]).strip()
        if any(op in raw for op in ("INTERVAL", "+", "-", "*", "/")):
            has_default = False
    if has_default:
        default = _mysql_clean_default(str(col["default"]))
        parts.append(f"DEFAULT {default}")
    if col.get("comment"):
        cmt = col["comment"].replace("'", "''")
        parts.append(f"COMMENT '{cmt}'")
    return " ".join(parts)


def _mysql_clean_default(val: str) -> str:
    """把 PostgreSQL default 值转为 MySQL 兼容形式。"""
    import re
    v = val.strip()
    # 去掉 PostgreSQL 类型转换：::jsonb, ::text, ::uuid, ::timestamptz 等
    while "::" in v:
        v = v.rsplit("::", 1)[0]
    # PostgreSQL now() → MySQL CURRENT_TIMESTAMP
    v = re.sub(r'\bnow\(\)', 'CURRENT_TIMESTAMP', v, flags=re.IGNORECASE)
    # PostgreSQL INTERVAL '24 hours' / '1 day' → MySQL INTERVAL 24 HOUR / INTERVAL 1 DAY
    def _fix_interval(m):
        num = m.group(1)
        unit = m.group(2).upper()
        # 复数单数都取单数形式
        unit = unit.replace("S", "")  # hours → HOUR, days → DAY
        return f"INTERVAL {num} {unit}"
    v = re.sub(r"INTERVAL\s+'(\d+)\s+(\w+)'", _fix_interval, v, flags=re.IGNORECASE)
    return v


def _mysql_check_expr(expr: str) -> str:
    """把 PostgreSQL 方言的 CHECK 表达式翻译成 MySQL 8 可接受的形式。"""
    import re

    out = normalize_space(expr)
    # PostgreSQL 类型转换：col::text / col::uuid
    out = re.sub(r"([A-Za-z_][A-Za-z0-9_.]*)\s*::\s*[A-Za-z_][A-Za-z0-9_\[\]]*", r"\1", out)
    # 时间加减：col + INTERVAL '1 year' → DATE_ADD(col, INTERVAL 1 YEAR)
    def _add(m):
        return f"DATE_ADD({m.group(1)}, INTERVAL {m.group(2)} {m.group(3).upper()})"

    def _sub(m):
        return f"DATE_SUB({m.group(1)}, INTERVAL {m.group(2)} {m.group(3).upper()})"

    out = re.sub(
        r"([A-Za-z_][A-Za-z0-9_.]*)\s*\+\s*INTERVAL\s+'(\d+)\s+([A-Za-z]+)'",
        _add,
        out,
        flags=re.IGNORECASE,
    )
    out = re.sub(
        r"([A-Za-z_][A-Za-z0-9_.]*)\s*-\s*INTERVAL\s+'(\d+)\s+([A-Za-z]+)'",
        _sub,
        out,
        flags=re.IGNORECASE,
    )
    # PostgreSQL cardinality() 用于 JSONB 数组长度 → MySQL JSON_LENGTH()
    out = re.sub(r"\bcardinality\s*\(", "JSON_LENGTH(", out, flags=re.IGNORECASE)
    return out


def _mysql_predicate_is_column_not_null_only(predicate: str, columns: list[str]) -> bool:
    """partial unique 谓词是否只由「索引列的 IS NOT NULL」组成。

    这种谓词在 MySQL 里无需生成列：唯一索引本身允许多个 NULL。
    """
    terms = [
        term.strip().strip("()").strip()
        for term in re.split(r"\s+AND\s+", predicate, flags=re.IGNORECASE)
        if term.strip()
    ]
    if not terms:
        return True
    for term in terms:
        match = re.fullmatch(
            r"([A-Za-z_][A-Za-z0-9_]*)\s+IS\s+NOT\s+NULL", term, flags=re.IGNORECASE
        )
        if not match or match.group(1) not in columns:
            return False
    return True


def render_table_mysql(table: dict, tables_by_short: dict[str, dict],
                       rls_force: bool) -> tuple[list[str], list[str]]:
    # PostgreSQL 用 schema-qualified 表名 (guliangiam.xxx)，MySQL 用裸表名
    full = short_name(table["table_name"])
    short = full  # MySQL 无 schema，short == full
    tbl_ref = f'`{full}`'  # DDL 语句里用带反引号的表名
    pk = pk_columns(table)
    cols = table.get("columns") or []
    cols_by_name = {c["name"]: c for c in cols}
    has_deleted_at = "deleted_at" in cols_by_name

    lines: list[str] = [f"CREATE TABLE IF NOT EXISTS {tbl_ref} ("]

    # 表级：判断是否用内联 PRIMARY KEY。
    #   - 复合 PK → 表末声明（所有列类型）
    #   - 单行 TEXT 家族 PK → 表末声明（内联会触发 ERROR 1170）
    #   - 单行非 TEXT PK → 内联声明（最简洁）
    inline_pk = False
    if len(pk) == 1:
        pk_col = cols_by_name[pk[0]]
        inline_pk = not _mysql_is_text_family(pk_col)

    for col in cols:
        cd = render_column_mysql(col)
        if inline_pk and col["name"] == pk[0]:
            cd += " PRIMARY KEY"
        lines.append(f"    {cd}")

    if not inline_pk:
        lines.append(f"    PRIMARY KEY ({_mysql_col_list(pk, cols_by_name)})")

    for c in table.get("constraints") or []:
        if c.get("type") == "check":
            lines.append(f"    CONSTRAINT {c['name']} CHECK ({_mysql_check_expr(c['expr'])})")
        elif c.get("type") == "unique":
            lines.append(f"    CONSTRAINT {c['name']} UNIQUE ({_mysql_col_list(c['columns'], cols_by_name)})")
        elif c.get("type") == "unique_partial":
            # MySQL 没有部分唯一索引，用「生成列 + 复合唯一键」模拟：
            # 仅当谓词成立时生成 1，否则生成 NULL（NULL 不参与唯一性冲突）。
            # 谓词只包含索引列的 IS NOT NULL 时无需生成列——MySQL 的唯一索引
            # 本身就允许多个 NULL。旧实现无条件下标 deleted_at，遇到没有该列
            # 的表（如 notification_providers）会直接建表失败。
            pred = normalize_space(c.get("predicate") or "").strip()
            idx_cols = c.get("columns") or []
            if not pred or _mysql_predicate_is_column_not_null_only(pred, idx_cols):
                lines.append(
                    f"    CONSTRAINT {c['name']} UNIQUE ({_mysql_col_list(idx_cols, cols_by_name)})"
                )
            else:
                safe = c["name"].replace("uq_", "gen_")  # 保留原后缀（如 _active），不硬加
                lines.append(
                    f"    {safe} TINYINT(1) AS "
                    f"(CASE WHEN ({pred}) THEN 1 ELSE NULL END) STORED, "
                    f"CONSTRAINT {c['name']} UNIQUE ({_mysql_col_list(idx_cols, cols_by_name)}, {safe})"
                )

    ordered_shorts = [short_name(t["table_name"]) for t in tables_by_short["_order"]]
    here = ordered_shorts.index(short)
    fk_alter: list[str] = []
    # 生成列表达式文本：InnoDB 拒绝「ON UPDATE CASCADE + 依赖该列的 STORED
    # 生成列」组合（级联更新会改写生成列的值，报 1215）。检测到这种依赖时
    # 把 ON UPDATE 降级为 RESTRICT；主键是 UUID 且不可变，语义差异可忽略。
    gen_text = " ".join(line for line in lines if line.strip().startswith("gen_"))
    for fk in table.get("fk_constraints") or []:
        target = short_name(fk["ref_table"])
        cols_list = ", ".join(fk_columns(fk))
        ref_cols = ", ".join(resolve_ref_columns(fk, tables_by_short))
        name = fk_name(fk, short)
        ref_tbl = short_name(fk["ref_table"])
        # 反引号转义保留字
        cols_list = ", ".join(f"`{c.strip()}`" for c in cols_list.split(","))
        ref_cols = ", ".join(f"`{c.strip()}`" for c in ref_cols.split(","))
        clause = (
            f"FOREIGN KEY ({cols_list}) REFERENCES `{ref_tbl}`({ref_cols})"
        )
        if fk.get("on_delete"):
            clause += f" ON DELETE {FK_ACTIONS[fk['on_delete']]}"
        if fk.get("on_update"):
            on_update = FK_ACTIONS[fk["on_update"]]
            if on_update == "CASCADE" and any(
                re.search(rf"\b{re.escape(col)}\b", gen_text) for col in fk_columns(fk)
            ):
                on_update = "RESTRICT"
            clause += f" ON UPDATE {on_update}"
        stmt = f"ALTER TABLE {tbl_ref} ADD CONSTRAINT {name} {clause};"
        if ordered_shorts.index(target) > here:
            fk_alter.append(stmt)
        else:
            lines.append(f"    CONSTRAINT {name} {clause}")

    # 把 CREATE TABLE (...) 的内部定义用逗号连接
    header = lines[0]
    inner = ",\n".join(lines[1:])
    tail = ") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci"
    if table.get("comment"):
        tbl_cmt = table["comment"].replace("'", "''")
        tail += f" COMMENT='{tbl_cmt}'"
    tail += ";"
    lines = [header, inner, tail]

    post_table: list[str] = []
    for idx in table.get("indexes") or []:
        where = idx.get("predicate")
        # PostgreSQL GIN 索引（JSONB @> 查询加速）MySQL 无等价能力，跳过
        if idx.get("using") == "gin":
            post_table.append(
                f"-- INDEX {idx['name']}: PostgreSQL GIN 索引，MySQL 无等价实现，已跳过"
            )
            continue
        # PostgreSQL 表达式索引（如 metadata ->> 'key'）MySQL 无法等价，跳过
        expr_cols = [c for c in idx["columns"] if any(op in c for op in ("->", "(", ")", "?"))]
        if expr_cols:
            post_table.append(
                f"-- INDEX {idx['name']}: 表达式索引 {expr_cols} MySQL 无等价实现，已跳过"
            )
            continue
        # PostgreSQL 索引列可能带操作类后缀，MySQL 不需要
        cleaned_cols = [c.split(" ")[0].strip() for c in idx["columns"]]
        cols_str = _mysql_col_list(cleaned_cols, cols_by_name)
        if where:
            if "deleted_at IS NULL" in where and has_deleted_at:
                gen = "gen_" + idx["name"].replace("idx_", "")  # 去掉 idx_ 前缀加 gen_ 前缀
                add_col_sql = (
                    f"ALTER TABLE {tbl_ref} ADD COLUMN {gen} TINYINT(1) "
                    f"GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED"
                )
                # 幂等保护：先查 information_schema.COLUMNS 判断生成列是否已存在
                # 注意：information_schema 里 TABLE_NAME 存储的是纯表名（不带反引号）
                post_table.append(
                    f"SET @_col_exists := (SELECT COLUMN_NAME "
                    f"FROM information_schema.COLUMNS "
                    f"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '{full}' "
                    f"AND COLUMN_NAME = '{gen}');\n"
                    f"SET @_sql := IF(@_col_exists IS NULL, "
                    f"  '{add_col_sql.replace(chr(39), chr(39)+chr(39))}', "
                    f"  'SELECT 1');\n"
                    f"PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;"
                )
                post_table.extend(
                    _mysql_index_if_not_exists(full, idx["name"],
                                               f"{cols_str}, `{gen}`", bool(idx.get("unique")))
                )
            else:
                post_table.append(
                    f"-- INDEX {idx['name']}: predicate {where} (MySQL 不支持 WHERE 子句，降级为普通索引)"
                )
                post_table.extend(
                _mysql_index_if_not_exists(full, idx["name"], cols_str, bool(idx.get("unique")))
            )
        else:
            post_table.extend(
                _mysql_index_if_not_exists(full, idx["name"], cols_str, bool(idx.get("unique")))
            )

    lines += post_table

    # MySQL 8 有 CREATE TABLE IF NOT EXISTS 但 ALTER TABLE ADD CONSTRAINT 没有 IF NOT EXISTS
    # 在延迟 FK 外包裹动态 SQL：从 information_schema 反查约束名是否存在
    deferred: list[str] = []
    for stmt in fk_alter:
        tbl = stmt.split("ALTER TABLE ")[1].split(" ")[0].strip("`")
        cname = stmt.split("ADD CONSTRAINT ")[1].split(" ")[0]
        deferred.append(
            f"SET @_cname := (SELECT CONSTRAINT_NAME "
            f"FROM information_schema.TABLE_CONSTRAINTS "
            f"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '{tbl}' "
            f"AND CONSTRAINT_NAME = '{cname}');\n"
            f"SET @_sql := IF(@_cname IS NULL, "
            f"  '{stmt.replace(chr(39), chr(39)+chr(39))}', "
            f"  'SELECT 1');\n"
            f"PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;"
        )

    return lines, deferred


def generate_mysql(tables: list[dict], infra: dict, dialect: str) -> tuple[str, str]:
    rls_force = bool((infra.get("rls") or {}).get("force"))
    tables_by_short = {short_name(t["table_name"]): t for t in tables}
    tables_by_short["_order"] = tables

    out: list[str] = [
        "-- ============================================================",
        "-- GuliangIAM MySQL 8.4+ Schema (V1)",
        "-- 生成物 — 请勿手工修改；唯一事实数据源为 contracts/schema/*.yaml，",
        "-- 由 scripts/gen_schema.py --dialect mysql 生成。",
        "-- PostgreSQL 与 MySQL 产物语义等价（ADR-023 第二基线）。",
        "-- ============================================================",
        "",
        "SET NAMES utf8mb4;",
        "SET FOREIGN_KEY_CHECKS = 0;",
        "",
    ]

    deferred_blocks: list[str] = []
    current_source = None
    for t in tables:
        if t["_source"] != current_source:
            current_source = t["_source"]
            out += ["", f"-- 分组：{current_source}", ""]
        block, deferred = render_table_mysql(t, tables_by_short, rls_force)
        out += block + [""]
        deferred_blocks += deferred

    if deferred_blocks:
        out += ["-- 延迟外键（条件化执行，避免前向引用）", ""]
        for d in deferred_blocks:
            out += d.split("\n") + [""]

    out += ["", "SET FOREIGN_KEY_CHECKS = 1;"]

    # MySQL rollback：DROP TABLE IF EXISTS CASCADE 语义
    rollback_lines: list[str] = [
        "-- MySQL V1 rollback — 删表 CASCADE",
        "",
        "SET FOREIGN_KEY_CHECKS = 0;",
        "",
    ]
    for t in reversed(tables):
        # MySQL 不用 schema-qualified 前缀，直接用裸表名（short_name 剥离 guliangiam.）
        rollback_lines.append(f"DROP TABLE IF EXISTS {short_name(t['table_name'])};")
    rollback_lines += ["", "SET FOREIGN_KEY_CHECKS = 1;"]

    return "\n".join(out).rstrip("\n") + "\n", "\n".join(rollback_lines) + "\n"


# --------------------------------------------------------------------------- #
# 下游类型渲染：Go struct + TypeScript export type                           #
# --------------------------------------------------------------------------- #
GO_SCHEMA_HEADER = """// Code generated by scripts/gen_schema.py; DO NOT EDIT.
// Source: contracts/schema/*.yaml (declarative schema, single source of truth)

package typesgen

import "time"

"""

TS_SCHEMA_HEADER = """// Code generated by scripts/gen_schema.py; DO NOT EDIT.
// Source: contracts/schema/*.yaml (declarative schema, single source of truth)

"""


def _gofmt(text: str) -> str:
    """把渲染出的 Go 源码交给 gofmt，使生成物本身就是 gofmt 结果。

    模板手工对齐（4 空格缩进 + 定宽列）永远不等于 gofmt 输出，此前只是没人
    跑过 `gofmt -l`，产物因此带格式债；门禁补上后必须由生成器负责格式化，
    否则「重新生成」与「格式化」会互相推翻。
    """
    try:
        proc = subprocess.run(["gofmt"], input=text, capture_output=True, text=True, check=False)
    except FileNotFoundError as exc:
        raise RuntimeError("未找到 gofmt：生成物必须等于 gofmt 输出，请安装 Go 工具链") from exc
    if proc.returncode != 0:
        raise RuntimeError(f"gofmt 失败：{proc.stderr.strip()}")
    return proc.stdout


def _oxfmt(text: str, rel_path: str) -> str:
    """把渲染出的 TS 交给 oxfmt，使生成物本身就是 `pnpm run format:check` 的结果。

    与 Go 侧同一理由：生成物若不带格式，`pnpm run format` 与「重新生成」会互相
    推翻，而 web 的格式门禁（oxfmt --check）不允许仓库里存在未格式化的文件。
    `--stdin-filepath` 让 oxfmt 只按该路径推断解析器，输出写到 stdout。
    """
    oxfmt = ROOT / "web" / "node_modules" / ".bin" / "oxfmt"
    if not oxfmt.exists():
        raise RuntimeError("未找到 oxfmt：请先在 web/ 下执行 pnpm install")
    rel_in_web = rel_path[len("web/") :] if rel_path.startswith("web/") else rel_path
    try:
        proc = subprocess.run(
            [str(oxfmt), "-c", "oxfmt.json", f"--stdin-filepath={rel_in_web}"],
            input=text,
            capture_output=True,
            text=True,
            check=False,
            cwd=ROOT / "web",
        )
    except FileNotFoundError as exc:
        raise RuntimeError("未找到 oxfmt：请先在 web/ 下执行 pnpm install") from exc
    if proc.returncode != 0:
        raise RuntimeError(f"oxfmt 失败：{proc.stderr.strip()}")
    return proc.stdout


def render_go_schema_types(tables: list[dict]) -> str:
    lines = [GO_SCHEMA_HEADER]
    for table in sorted(tables, key=lambda t: short_name(t["table_name"])):
        short = short_name(table["table_name"])
        pascal = _go_exported_name(short)
        cols = table.get("columns") or []
        lines.append(f"// {pascal} maps to guliangiam.{short}.")
        lines.append(f"type {pascal} struct {{")
        for col in cols:
            gt = _go_type(col)
            nullable_flag = " // nullable" if col.get("nullable", True) else " // NOT NULL"
            lines.append(f"    {_go_exported_name(col['name']):30s} {gt:20s} `db:\"{col['name']}\"`{nullable_flag}")
        lines.append("}")
        lines.append("")
    return _gofmt("\n".join(lines).rstrip("\n") + "\n")


def render_ts_schema_types(tables: list[dict]) -> str:
    lines = [TS_SCHEMA_HEADER]
    for table in sorted(tables, key=lambda t: short_name(t["table_name"])):
        short = short_name(table["table_name"])
        pascal = _snake_to_pascal(short)
        cols = table.get("columns") or []
        lines.append(f"// {pascal} — rows of guliangiam.{short}.")
        lines.append(f"export type {pascal} = {{")
        for col in cols:
            tt = _ts_type(col)
            nullable_flag = "" if col.get("nullable", True) else "  // NOT NULL"
            lines.append(f"  {col['name']}: {tt}{nullable_flag}")
        lines.append("};")
        lines.append("")
    return _oxfmt("\n".join(lines).rstrip("\n") + "\n", TS_SCHEMA_PATH)


# --------------------------------------------------------------------------- #
# 版本化 diff 迁移生成（用于 V2+，表达移除类变更）                            #
# --------------------------------------------------------------------------- #
# V1 是全量建表（CREATE TABLE IF NOT EXISTS）。V2+ 不再改动 V1，而是产出
# 增量 ALTER TABLE 脚本。每次 diff 从声明式 Schema 的 history 快照加载上一
# 个版本的 YAML，对比当前 YAML，产出 4 类变更：
#   新增表 / 删表 / 增列 / 删列 / 改列类型 / 改列 nullable / 改列默认值


def load_tables_from_dir(dir_path: str) -> list[dict]:
    """从指定目录（如 contracts/schema/history/V1/）加载全部表定义。"""
    import glob as _glob

    tables = []
    for yaml_path in sorted(_glob.glob(str(ROOT / dir_path / "*.yaml"))):
        if "infra" in yaml_path:
            continue
        with open(yaml_path) as f:
            docs = list(yaml.safe_load_all(f))
        for doc in docs:
            if doc and doc.get("table_name") and doc.get("columns"):
                doc["_source"] = Path(yaml_path).name
                _apply_implicit_columns(doc)
                tables.append(doc)
    return tables


def additive_table_items(old_table: dict, new_table: dict, key: str) -> list[dict] | None:
    """Return items appended to a table list, or None when its existing prefix changed."""
    old_items = old_table.get(key) or []
    new_items = new_table.get(key) or []
    if len(new_items) < len(old_items) or new_items[:len(old_items)] != old_items:
        return None
    return new_items[len(old_items):]


def _table_item_key(item: dict, key: str):
    """Stable identity for list items; naming is the contract for indexes/constraints."""
    if key == "fk_constraints":
        return (item.get("column"), item.get("ref_table"))
    return item.get("name")


def table_item_removals(old_table: dict, new_table: dict, key: str) -> list[dict] | None:
    """Return items removed from a table list.

    ``None`` means the edit is not a pure removal: either the surviving items were
    reordered, or an item kept its name but changed its definition in place. Both
    are unsafe to render as DROP+CREATE without an explicit decision, so the
    increment generator rejects them instead of guessing.
    """
    old_items = old_table.get(key) or []
    new_items = new_table.get(key) or []
    old_by_key = {_table_item_key(item, key): item for item in old_items}
    new_keys = {_table_item_key(item, key) for item in new_items}
    surviving_keys = [_table_item_key(item, key) for item in old_items if _table_item_key(item, key) in new_keys]
    if surviving_keys != [_table_item_key(item, key) for item in new_items[: len(surviving_keys)]]:
        return None
    for item in new_items[: len(surviving_keys)]:
        if old_by_key.get(_table_item_key(item, key)) != item:
            return None
    return [item for item in old_items if _table_item_key(item, key) not in new_keys]


def diff_schemas(prev_tables: list[dict], cur_tables: list[dict]) -> dict:
    prev = {short_name(t["table_name"]): t for t in prev_tables}
    cur = {short_name(t["table_name"]): t for t in cur_tables}

    added_tables: list[dict] = []
    dropped_tables: list[dict] = []
    changed_columns: dict[str, dict] = {}

    dropped = set(prev) - set(cur)
    added = set(cur) - set(prev)
    # set 迭代顺序随 PYTHONHASHSEED 改变；声明文件顺序本身是 FK 拓扑序，
    # 必须同时用于新增/删除表，避免连续生成内容漂移或产生前向外键。
    dropped_tables = [t for t in prev_tables if short_name(t["table_name"]) in dropped]
    added_tables = [t for t in cur_tables if short_name(t["table_name"]) in added]

    for short in set(prev) & set(cur):
        prev_cols = {c["name"]: c for c in prev[short].get("columns", [])}
        cur_cols = {c["name"]: c for c in cur[short].get("columns", [])}

        added_cols = [cur_cols[n] for n in sorted(set(cur_cols) - set(prev_cols))]
        dropped_cols = [prev_cols[n] for n in sorted(set(prev_cols) - set(cur_cols))]
        modified: list[tuple[dict, dict]] = []
        for n in sorted(set(prev_cols) & set(cur_cols)):
            if _col_signature(prev_cols[n]) != _col_signature(cur_cols[n]):
                modified.append((prev_cols[n], cur_cols[n]))

        if added_cols or dropped_cols or modified:
            changed_columns[short] = {
                "table": cur.get(short) or prev[short],
                "added": added_cols,
                "dropped": dropped_cols,
                "modified": modified,
            }

    added_checks: dict[str, dict] = {}
    added_uniques: dict[str, dict] = {}
    added_unique_partials: dict[str, dict] = {}
    added_indexes: dict[str, dict] = {}
    dropped_indexes: dict[str, dict] = {}
    added_fks: dict[str, dict] = {}
    # 同名 CHECK 的定义变更（典型场景：扩展 kind 闭集枚举）允许以
    # DROP + ADD 表达；UNIQUE/FK 的描述中改定义仍不支持（依赖索引/数据）。
    changed_checks: dict[str, dict] = {}
    for short in sorted(set(prev) & set(cur)):
        constraints = additive_table_items(prev[short], cur[short], "constraints")
        for kind, changes in (
            ("check", added_checks),
            ("unique", added_uniques),
            ("unique_partial", added_unique_partials),
        ):
            added = [c for c in constraints or [] if c.get("type") == kind]
            if added:
                changes[short] = {"table": cur[short], "added": added}

        replaced = replaced_checks(prev[short], cur[short])
        if replaced:
            changed_checks[short] = {"table": cur[short], "replaced": replaced}

        indexes = additive_table_items(prev[short], cur[short], "indexes")
        if indexes:
            added_indexes[short] = {"table": cur[short], "added": indexes}
        removed_indexes = table_item_removals(prev[short], cur[short], "indexes")
        if removed_indexes:
            dropped_indexes[short] = {"table": prev[short], "dropped": removed_indexes}

        fks = additive_table_items(prev[short], cur[short], "fk_constraints")
        if fks:
            added_fks[short] = {"table": cur[short], "added": fks}

    # 已有表从「无 RLS」切换为「有 RLS」是安全收紧，允许增量表达；
    # 反向（关闭 RLS）与已启用表的谓词改写仍需人工评审，不进生成器安全子集。
    added_rls: dict[str, dict] = {}
    for short in sorted(set(prev) & set(cur)):
        if not prev[short].get("rls") and cur[short].get("rls"):
            added_rls[short] = {"table": cur[short]}

    return {
        "added_tables": added_tables,
        "dropped_tables": dropped_tables,
        "changed_columns": changed_columns,
        "added_checks": added_checks,
        "changed_checks": changed_checks,
        "added_uniques": added_uniques,
        "added_unique_partials": added_unique_partials,
        "added_indexes": added_indexes,
        "dropped_indexes": dropped_indexes,
        "added_fks": added_fks,
        "added_rls": added_rls,
    }


def replaced_checks(old_table: dict, new_table: dict) -> list[dict]:
    """Return same-named CHECK constraints whose definition changed in place.

    Only ``type == check`` participates: a UNIQUE/FK/partial-unique whose
    definition moved stays in the unsupported bucket, because those need index
    or data semantics rather than a plain DROP + ADD.
    """
    old = {
        c["name"]: c
        for c in old_table.get("constraints") or []
        if c.get("type") == "check"
    }
    new = {
        c["name"]: c
        for c in new_table.get("constraints") or []
        if c.get("type") == "check"
    }
    replaced = []
    for name in sorted(set(old) & set(new)):
        if old[name] != new[name]:
            replaced.append({"name": name, "old": old[name], "new": new[name]})
    return replaced


def migration_diff_has_changes(diff: dict) -> bool:
    return any(diff.get(key) for key in (
        "added_tables",
        "dropped_tables",
        "changed_columns",
        "added_checks",
        "changed_checks",
        "added_uniques",
        "added_unique_partials",
        "added_indexes",
        "dropped_indexes",
        "added_fks",
        "added_rls",
    ))


def constraint_edits_supported(old_table: dict, new_table: dict) -> bool:
    """Whether the constraint-list edit stays inside the incremental subset.

    Supported: appending ``check``/``unique``/``unique_partial``; replacing a
    same-named ``check`` definition (rendered as DROP + ADD).
    Unsupported: removing a constraint (needs cascade semantics) and changing
    the definition of a same-named UNIQUE/partial-unique (index-backed).
    """
    old = {c["name"]: c for c in old_table.get("constraints") or []}
    new = {c["name"]: c for c in new_table.get("constraints") or []}
    if set(old) - set(new):
        return False
    for name in set(new) - set(old):
        if new[name].get("type") not in ("check", "unique", "unique_partial"):
            return False
    for name in set(old) & set(new):
        if old[name].get("type") != new[name].get("type"):
            return False
        if old[name] != new[name] and new[name].get("type") != "check":
            return False
    return True


def unsupported_migration_changes(
    prev_tables: list[dict], cur_tables: list[dict]
) -> list[str]:
    """Find table/column edits outside the migration renderer's supported subset."""
    prev = {short_name(t["table_name"]): t for t in prev_tables}
    cur = {short_name(t["table_name"]): t for t in cur_tables}
    unsupported: list[str] = []

    for short in sorted(set(prev) & set(cur)):
        old_table, new_table = prev[short], cur[short]
        metadata_exclusions = {"columns", "constraints", "indexes", "fk_constraints", "_source"}
        rls_keys = {"rls", "rls_force", "rls_predicate"}
        old_metadata = {k: v for k, v in old_table.items() if k not in metadata_exclusions}
        new_metadata = {k: v for k, v in new_table.items() if k not in metadata_exclusions}
        old_rest = {k: v for k, v in old_metadata.items() if k not in rls_keys}
        new_rest = {k: v for k, v in new_metadata.items() if k not in rls_keys}
        # 仅允许「从未启用 → 启用」的单向 RLS 收紧；关闭 RLS、已启用表改谓词
        # 或改 rls_force 均视为不安全变更，要求人工评审。
        rls_enable_only = (
            not old_table.get("rls")
            and bool(new_table.get("rls"))
            and old_table.get("rls_force") == new_table.get("rls_force")
        )
        rls_regression = (
            bool(old_table.get("rls"))
            and old_table.get("rls_predicate") != new_table.get("rls_predicate")
        )
        # 约束只接受「纯追加」或「同名 CHECK 定义替换」：删除约束需要级联语义
        # （unique → 依赖索引、FK → 依赖数据），不在增量渲染器的安全子集内；索引
        # 允许纯删除（消除被唯一约束覆盖的冗余索引），但重排或就地改定义仍不支持。
        invalid_list_edit = (
            not constraint_edits_supported(old_table, new_table)
            or additive_table_items(old_table, new_table, "fk_constraints") is None
            or table_item_removals(old_table, new_table, "indexes") is None
        )
        if old_rest != new_rest or rls_regression or not (old_metadata == new_metadata or rls_enable_only) or invalid_list_edit:
            unsupported.append(
                f"{short}: primary key, indexes, constraints, foreign keys, RLS, "
                "or other table metadata changed"
            )

        old_columns = {c["name"]: c for c in old_table.get("columns", [])}
        new_columns = {c["name"]: c for c in new_table.get("columns", [])}
        for name in sorted(set(old_columns) & set(new_columns)):
            old_col, new_col = old_columns[name], new_columns[name]
            old_extra = {k: v for k, v in old_col.items() if k not in ("name", "type", "nullable", "default", "comment")}
            new_extra = {k: v for k, v in new_col.items() if k not in ("name", "type", "nullable", "default", "comment")}
            if old_extra != new_extra:
                unsupported.append(
                    f"{short}.{name}: column metadata outside type/nullability/default changed"
                )

    return unsupported


def freeze_snapshot(snapshot_path: Path, source_files: list[Path]) -> None:
    """Atomically freeze schema YAML files without replacing an existing snapshot."""
    if snapshot_path.exists():
        raise FileExistsError(f"refusing to overwrite frozen schema snapshot {snapshot_path}")
    if not source_files:
        raise ValueError("no schema YAML files found to freeze")

    snapshot_path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=f".{snapshot_path.name}-", dir=snapshot_path.parent) as tmp:
        tmp_path = Path(tmp)
        for source in source_files:
            shutil.copyfile(source, tmp_path / source.name)
        tmp_path.replace(snapshot_path)


def replace_snapshot(snapshot_path: Path, source_files: list[Path]) -> None:
    """用当前声明文件原子替换最新活动快照（仅用于纯组织重构：表级 diff 必须为空）。

    声明层文件分组/拆分/改名属于「组织重构」而非结构变更：表、列、约束的 SQL 语义不变，
    因此不产生迁移版本；但活动快照必须重新冻结以反映最新文件布局（check-active-schema-history
    比较的是快照文件字节与 contracts/schema 目录）。调用方必须先证明表级 diff 为空。
    """
    if not snapshot_path.is_dir():
        raise FileNotFoundError(f"active snapshot does not exist: {snapshot_path}")
    if not source_files:
        raise ValueError("no schema YAML files found to freeze")
    staging = snapshot_path.parent / (snapshot_path.name + ".rebase")
    if staging.exists():
        shutil.rmtree(staging)
    staging.mkdir(parents=True)
    for source in source_files:
        shutil.copyfile(source, staging / source.name)
    backup = snapshot_path.parent / (snapshot_path.name + ".bak")
    if backup.exists():
        shutil.rmtree(backup)
    snapshot_path.rename(backup)
    staging.rename(snapshot_path)
    shutil.rmtree(backup)


def render_migration_constraint(full: str, name: str, clause: str) -> str:
    return (
        "DO $$\n"
        "BEGIN\n"
        "    IF NOT EXISTS (\n"
        "        SELECT 1 FROM pg_constraint\n"
        f"        WHERE conname = '{name}'\n"
        f"          AND conrelid = '{full}'::regclass\n"
        "    ) THEN\n"
        f"        ALTER TABLE {full} ADD CONSTRAINT {name} {clause};\n"
        "    END IF;\n"
        "END $$;"
    )


def render_migration_index(full: str, index: dict, force_unique: bool = False) -> str:
    unique = "UNIQUE " if force_unique or index.get("unique") else ""
    using = f" USING {index['using']}" if index.get("using") else ""
    where = f" WHERE {normalize_space(index['predicate'])}" if index.get("predicate") else ""
    return (
        f"CREATE {unique}INDEX IF NOT EXISTS {index['name']} ON {full}"
        f"{using} ({', '.join(index['columns'])}){where};"
    )


def render_migration_up(diff: dict, version: int, prev_version: int,
                        tables: list[dict], infra: dict,
                        renames: list[tuple[str, str, str]] | None = None) -> str:
    tables_by_short = {short_name(t["table_name"]): t for t in tables}
    tables_by_short["_order"] = tables  # render_table 需要 _order 做 FK 前向引用延迟
    lines = [
        f"-- Auto-generated by scripts/gen_schema.py -- DO NOT EDIT",
        f"-- 版本化迁移 V{version}：上版本 V{prev_version}",
        f"-- 变更内容：",
        f"--   新增表 {len(diff['added_tables'])} 张",
        f"--   删表 {len(diff['dropped_tables'])} 张",
        f"--   {len(diff['changed_columns'])} 张表有列变更（ADD/DROP/ALTER）",
        f"--   列改名 {len(renames or [])} 处（数据保留的原地 RENAME COLUMN）",
        f"-- 事务内执行，失败自动回滚。",
        "", "BEGIN;", "",
    ]
    if renames:
        lines.append("-- === 重命名列（原地改名，数据保留）===")
        lines.append("")
        for table_full, old_name, new_name in renames:
            lines.append(f"ALTER TABLE {table_full} RENAME COLUMN {old_name} TO {new_name};")
        lines.append("")

    lines.append("-- === 删表（按 FK 逆拓扑序）===")
    lines.append("-- ⚠️  自动 DROP TABLE 已禁用 — 破坏性迁移必须手动编写")
    lines.append(f"--   检测到 {len(diff['dropped_tables'])} 张待删表，全部跳过：")
    for _t in reversed(list(diff["dropped_tables"])):
        lines.append(f"--   ❌ SKIP DROP TABLE {_t['table_name']}")

    # 自动 DROP COLUMN 已禁用 — 破坏性迁移必须手动编写
    dropped_col_count = sum(len(c["dropped"]) for c in diff["changed_columns"].values())
    if dropped_col_count > 0:
        lines.append("-- === 删列 ===")
        lines.append(f"-- ⚠️  自动 DROP COLUMN 已禁用 — 检测到 {dropped_col_count} 列待删，全部跳过")
        for short in sorted(diff["changed_columns"]):
            change = diff["changed_columns"][short]
            for col in change["dropped"]:
                lines.append(f"--   ❌ SKIP DROP COLUMN {change['table']['table_name']}.{col['name']}")
        lines.append("")

    if diff.get("dropped_indexes"):
        lines.append("-- === 删除冗余/失效索引 ===")
        lines.append("")
        for short in sorted(diff["dropped_indexes"]):
            change = diff["dropped_indexes"][short]
            for index in reversed(change["dropped"]):
                lines.append(f"DROP INDEX IF EXISTS {index['name']};")
        lines.append("")

    if diff["added_tables"]:
        lines.append("")
        lines.append("-- === 新增表 ===")
        lines.append("")
        for t in diff["added_tables"]:
            body, _delayed = render_table(t, tables_by_short, rls_force=True)
            lines.extend(body)
            lines.append("")

    if diff["changed_columns"]:
        lines.append("-- === 增列 & 改列 ===")
        lines.append("")
    for short in sorted(diff["changed_columns"]):
        change = diff["changed_columns"][short]
        full = change["table"]["table_name"]
        for col in change["added"]:
            parts = [full, f"ADD COLUMN IF NOT EXISTS {col['name']}", pg_column_type(col)]
            if not col.get("nullable", True):
                parts.append("NOT NULL")
            if col.get("default") is not None:
                parts.append(f"DEFAULT {col['default']}")
            lines.append("ALTER TABLE " + " ".join(parts) + ";")
            # 新增列携带声明层注释：迁移本身即记录该列的语义边界（与隐式列一致跳过）。
            if col.get("comment") and col.get("name") not in IMPLICIT_COLUMNS:
                cmt = col["comment"].replace("'", "''")
                lines.append(f"COMMENT ON COLUMN {full}.{col['name']} IS '{cmt}';")
        for old, new in change["modified"]:
            if old.get("default") is not None and new.get("default") is None:
                lines.append(f"ALTER TABLE {full} ALTER COLUMN {new['name']} DROP DEFAULT;")
            elif old.get("default") is None and new.get("default") is not None:
                lines.append(f"ALTER TABLE {full} ALTER COLUMN {new['name']} SET DEFAULT {new['default']};")
            elif old.get("default") != new.get("default"):
                lines.append(f"ALTER TABLE {full} ALTER COLUMN {new['name']} SET DEFAULT {new['default']};")
            if old.get("nullable", True) != new.get("nullable", True):
                if not new.get("nullable", True):
                    lines.append(f"ALTER TABLE {full} ALTER COLUMN {new['name']} SET NOT NULL;")
                else:
                    lines.append(f"ALTER TABLE {full} ALTER COLUMN {new['name']} DROP NOT NULL;")
            if old.get("type", "") != new.get("type", ""):
                target_type = pg_column_type(new)
                lines.append(
                    f"ALTER TABLE {full} ALTER COLUMN {new['name']} "
                    f"TYPE {target_type} USING ({new['name']}::{target_type.split('(')[0]});"
                )

    lines.append("")
    if diff["added_checks"]:
        lines.append("-- === 新增 CHECK 约束 ===")
        lines.append("")
        for short in sorted(diff["added_checks"]):
            change = diff["added_checks"][short]
            full = change["table"]["table_name"]
            for constraint in change["added"]:
                clause = f"CHECK ({normalize_space(constraint['expr'])})"
                lines.append(render_migration_constraint(full, constraint["name"], clause))
        lines.append("")

    if diff["changed_checks"]:
        lines.append("-- === 替换同名 CHECK 约束（定义变更）===")
        lines.append("")
        for short in sorted(diff["changed_checks"]):
            change = diff["changed_checks"][short]
            full = change["table"]["table_name"]
            for item in change["replaced"]:
                lines.append(f"ALTER TABLE {full} DROP CONSTRAINT IF EXISTS {item['name']};")
                lines.append(
                    f"ALTER TABLE {full} ADD CONSTRAINT {item['name']} "
                    f"CHECK ({normalize_space(item['new']['expr'])});"
                )
        lines.append("")

    if diff["added_uniques"]:
        lines.append("-- === 新增 UNIQUE 约束 ===")
        lines.append("")
        for short in sorted(diff["added_uniques"]):
            change = diff["added_uniques"][short]
            full = change["table"]["table_name"]
            for constraint in change["added"]:
                clause = f"UNIQUE ({', '.join(constraint['columns'])})"
                lines.append(render_migration_constraint(full, constraint["name"], clause))
        lines.append("")

    if diff["added_unique_partials"]:
        lines.append("-- === 新增部分 UNIQUE 索引 ===")
        lines.append("")
        for short in sorted(diff["added_unique_partials"]):
            change = diff["added_unique_partials"][short]
            full = change["table"]["table_name"]
            for constraint in change["added"]:
                lines.append(render_migration_index(full, constraint, force_unique=True))
        lines.append("")

    if diff["added_indexes"]:
        lines.append("-- === 新增索引 ===")
        lines.append("")
        for short in sorted(diff["added_indexes"]):
            change = diff["added_indexes"][short]
            full = change["table"]["table_name"]
            for index in change["added"]:
                lines.append(render_migration_index(full, index))
        lines.append("")

    if diff["added_fks"]:
        lines.append("-- === 新增外键约束 ===")
        lines.append("")
        for short in sorted(diff["added_fks"]):
            change = diff["added_fks"][short]
            full = change["table"]["table_name"]
            for fk in change["added"]:
                name = fk_name(fk, short)
                clause = render_fk_clause(fk, tables_by_short)
                lines.append(render_migration_constraint(full, name, clause))
        lines.append("")

    if diff.get("added_rls"):
        lines.append("-- === 启用租户行级安全（本版本新增 RLS）===")
        lines.append("")
        for short in sorted(diff["added_rls"]):
            table = diff["added_rls"][short]["table"]
            full = table["table_name"]
            lines.append(f"ALTER TABLE {full} ENABLE ROW LEVEL SECURITY;")
            lines.append(f"ALTER TABLE {full} FORCE ROW LEVEL SECURITY;")
            if table.get("rls_predicate"):
                predicate = normalize_space(table["rls_predicate"])
            else:
                cols = {c["name"] for c in table.get("columns") or []}
                tenant_col = "organization_id" if "organization_id" in cols else "id"
                predicate = (
                    f"guliangiam.is_super_admin() OR {tenant_col} = guliangiam.current_organization_id()"
                )
            lines.append(
                "DO $$\n"
                "BEGIN\n"
                "    IF NOT EXISTS (\n"
                "        SELECT 1 FROM pg_policies\n"
                "        WHERE schemaname = 'guliangiam'\n"
                f"          AND tablename = '{short}'\n"
                "          AND policyname = 'tenant_isolation'\n"
                "    ) THEN\n"
                f"        CREATE POLICY tenant_isolation ON {full}\n"
                f"            USING ({predicate})\n"
                f"            WITH CHECK ({predicate});\n"
                "    END IF;\nEND $$;"
            )
        lines.append("")

    lines.append("COMMIT;")
    return "\n".join(lines).rstrip("\n") + "\n"


def render_migration_down(diff: dict, version: int, prev_version: int,
                          renames: list[tuple[str, str, str]] | None = None) -> str:
    """渲染增量回滚脚本 —— 正脚本的逆运算。

    正脚本顺序：DROP TABLE → DROP COLUMN → CREATE TABLE → ADD COLUMN → ALTER COLUMN
    本函数逆序撤销：ALTER→DROP(ADD)→DROP(CREATE)→ADD(DROP)
    """
    lines = [
        f"-- Auto-generated by scripts/gen_schema.py -- DO NOT EDIT",
        f"-- 版本化回滚 V{version} → V{prev_version}",
        f"-- 正脚本逆运算：ALTER→DROP(ADD)→DROP(CREATE)→ADD(DROP)",
        "", "BEGIN;", "",
    ]

    if diff.get("added_rls"):
        lines.append("-- === 关闭本版本启用的 RLS（撤销 ENABLE/FORCE/POLICY）===")
        lines.append("")
        for short in sorted(diff["added_rls"], reverse=True):
            full = diff["added_rls"][short]["table"]["table_name"]
            lines.append(f"DROP POLICY IF EXISTS tenant_isolation ON {full};")
            lines.append(f"ALTER TABLE {full} NO FORCE ROW LEVEL SECURITY;")
            lines.append(f"ALTER TABLE {full} DISABLE ROW LEVEL SECURITY;")
        lines.append("")

    if diff["added_fks"]:
        lines.append("-- === 删除本版本新增外键约束 ===")
        lines.append("")
        for short in sorted(diff["added_fks"], reverse=True):
            change = diff["added_fks"][short]
            full = change["table"]["table_name"]
            for fk in reversed(change["added"]):
                lines.append(f"ALTER TABLE {full} DROP CONSTRAINT IF EXISTS {fk_name(fk, short)};")
        lines.append("")

    if diff.get("dropped_indexes"):
        lines.append("-- === 恢复本版本删除的索引 ===")
        lines.append("")
        for short in sorted(diff["dropped_indexes"]):
            change = diff["dropped_indexes"][short]
            full = change["table"]["table_name"]
            for index in change["dropped"]:
                using = f" USING {index['using']}" if index.get("using") else ""
                where = f" WHERE {normalize_space(index['predicate'])}" if index.get("predicate") else ""
                unique = "UNIQUE " if index.get("unique") else ""
                lines.append(
                    f"CREATE {unique}INDEX IF NOT EXISTS {index['name']} ON {full}"
                    f"{using} ({', '.join(index['columns'])}){where};"
                )
        lines.append("")

    if diff["added_indexes"]:
        lines.append("-- === 删除本版本新增索引 ===")
        lines.append("")
        for short in sorted(diff["added_indexes"], reverse=True):
            change = diff["added_indexes"][short]
            for index in reversed(change["added"]):
                lines.append(f"DROP INDEX IF EXISTS {index['name']};")
        lines.append("")

    if diff["added_unique_partials"]:
        lines.append("-- === 删除本版本新增部分 UNIQUE 索引 ===")
        lines.append("")
        for short in sorted(diff["added_unique_partials"], reverse=True):
            change = diff["added_unique_partials"][short]
            for constraint in reversed(change["added"]):
                lines.append(f"DROP INDEX IF EXISTS {constraint['name']};")
        lines.append("")

    if diff["added_uniques"]:
        lines.append("-- === 删除本版本新增 UNIQUE 约束 ===")
        lines.append("")
        for short in sorted(diff["added_uniques"], reverse=True):
            change = diff["added_uniques"][short]
            full = change["table"]["table_name"]
            for constraint in reversed(change["added"]):
                lines.append(f"ALTER TABLE {full} DROP CONSTRAINT IF EXISTS {constraint['name']};")
        lines.append("")

    if diff["added_checks"]:
        lines.append("-- === 删除本版本新增 CHECK 约束（先于列回滚）===")
        lines.append("")
        for short in sorted(diff["added_checks"], reverse=True):
            change = diff["added_checks"][short]
            full = change["table"]["table_name"]
            for constraint in reversed(change["added"]):
                lines.append(f"ALTER TABLE {full} DROP CONSTRAINT IF EXISTS {constraint['name']};")
        lines.append("")

    if diff["changed_checks"]:
        lines.append("-- === 还原被替换的 CHECK 约束定义（回到上版本）===")
        lines.append("")
        for short in sorted(diff["changed_checks"], reverse=True):
            change = diff["changed_checks"][short]
            full = change["table"]["table_name"]
            for item in reversed(change["replaced"]):
                lines.append(f"ALTER TABLE {full} DROP CONSTRAINT IF EXISTS {item['name']};")
                lines.append(
                    f"ALTER TABLE {full} ADD CONSTRAINT {item['name']} "
                    f"CHECK ({normalize_space(item['old']['expr'])});"
                )
        lines.append("")

    changed = [diff["changed_columns"][k] for k in sorted(diff["changed_columns"])]

    # 1. 回退改列 —— 全部恢复到 old（prev_version）状态
    #    顺序：先 TYPE（避免 new 类型不接受 old 的 DEFAULT 值）
    #          再 nullable（NOT NULL 前必须已有 DEFAULT 值填充空行）
    #          最后 DEFAULT
    if changed:
        lines.append("-- === 回退 ALTER COLUMN（恢复到上版本状态）===")
        lines.append("")
    for change in reversed(changed):
        full = change["table"]["table_name"]
        for old, new in change["modified"]:
            if old.get("type", "") != new.get("type", ""):
                lines.append(
                    f"ALTER TABLE {full} ALTER COLUMN {old['name']} "
                    f"TYPE {old['type']} USING ({old['name']}::{old['type'].split('(')[0]});"
                )
            if old.get("default") is not None and new.get("default") != old.get("default"):
                lines.append(f"ALTER TABLE {full} ALTER COLUMN {old['name']} SET DEFAULT {old['default']};")
            elif old.get("default") is None and new.get("default") is not None:
                lines.append(f"ALTER TABLE {full} ALTER COLUMN {old['name']} DROP DEFAULT;")
            if old.get("nullable", True) != new.get("nullable", True):
                if not old.get("nullable", True):
                    lines.append(f"ALTER TABLE {full} ALTER COLUMN {old['name']} SET NOT NULL;")
                else:
                    lines.append(f"ALTER TABLE {full} ALTER COLUMN {old['name']} DROP NOT NULL;")

    # 2. DROP COLUMN —— 撤销正脚本 ADD COLUMN（正脚本最后执行，回滚最先撤销）
    if changed and any(c["added"] for c in changed):
        lines.append("")
        lines.append("-- === DROP COLUMN（撤销正脚本 ADD COLUMN）===")
        lines.append("")
    for change in reversed(changed):
        full = change["table"]["table_name"]
        for col in reversed(change["added"]):
            lines.append(f"ALTER TABLE {full} DROP COLUMN IF EXISTS {col['name']};")

    # 3. DROP TABLE —— 撤销正脚本 CREATE TABLE
    if diff["added_tables"]:
        lines.append("")
        lines.append("-- === 删除本版本新增表（撤销 CREATE TABLE）===")
        lines.append("")
        for t in reversed(list(diff["added_tables"])):
            lines.append(f"DROP TABLE IF EXISTS {t['table_name']} CASCADE;")

    # 4. ADD COLUMN —— 恢复正脚本中被 DROP 的列
    if changed and any(c["dropped"] for c in changed):
        lines.append("")
        lines.append("-- === 恢复被删除的列（正脚本 DROP COLUMN 的逆运算）===")
        lines.append("")
    for change in reversed(changed):
        full = change["table"]["table_name"]
        for col in reversed(change["dropped"]):
            parts = [full, f"ADD COLUMN IF NOT EXISTS {col['name']}", col["type"]]
            if not col.get("nullable", True):
                parts.append("NOT NULL")
            if col.get("default") is not None:
                parts.append(f"DEFAULT {col['default']}")
            lines.append("ALTER TABLE " + " ".join(parts) + ";")

    lines.append("")
    # 5. 反向重命名 —— 撤销正脚本的 RENAME COLUMN（回滚后列名恢复为 V{N-1} 形态）
    if renames:
        lines.append("")
        lines.append("-- === 反向重命名（撤销正脚本 RENAME COLUMN）===")
        lines.append("")
        for table_full, old_name, new_name in reversed(renames):
            lines.append(f"ALTER TABLE {table_full} RENAME COLUMN {new_name} TO {old_name};")

    lines.append("COMMIT;")
    return "\n".join(lines).rstrip("\n") + "\n"


def render_dialect(tables: list[dict], dialect: str) -> tuple[str, str]:
    """按方言渲染当前状态的全量 DDL。"""
    if dialect == "mysql":
        return generate_mysql(tables, load_infra(dialect), dialect)
    return generate_postgresql(tables, load_infra(dialect), dialect)


def active_versions() -> list[int]:
    root = ROOT / ACTIVE_HISTORY_DIR
    if not root.is_dir():
        return []
    versions = []
    for path in root.iterdir():
        match = re.fullmatch(r"V([1-9][0-9]*)", path.name)
        if path.is_dir() and match:
            versions.append(int(match.group(1)))
    return sorted(versions)


def load_version_renames(version: int) -> list[dict]:
    """读取 contracts/schema/migrations/renames.yaml 中属于 V{version} 的改名登记。"""
    path = ROOT / SCHEMA_RENAMES
    if not path.exists():
        return []
    data = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    entries = data.get("renames") or []
    selected = []
    for entry in entries:
        required = ("version", "table", "from", "to")
        if not isinstance(entry, dict) or any(key not in entry for key in required):
            raise SchemaError(
                f"{SCHEMA_RENAMES}: 每条 renames 记录必须包含 version/table/from/to"
            )
        try:
            entry_version = int(entry["version"])
        except (TypeError, ValueError):
            raise SchemaError(f"{SCHEMA_RENAMES}: version 必须是整数：{entry!r}")
        if entry_version == version:
            selected.append(entry)
    return selected


def apply_column_renames(prev_tables: list[dict], version: int) -> tuple[list[dict], list[tuple[str, str, str]]]:
    """把 V{N-1} 快照中的旧列名按登记改名为新列名，使 diff 识别为 rename 而非 drop+add。"""
    entries = load_version_renames(version)
    if not entries:
        return prev_tables, []
    import copy

    tables = copy.deepcopy(prev_tables)
    by_short = {short_name(t["table_name"]): t for t in tables}
    rendered: list[tuple[str, str, str]] = []
    for entry in entries:
        table = by_short.get(entry["table"])
        if table is None:
            raise SchemaError(f"renames V{version}: 表 {entry['table']} 在上版本快照中不存在")
        columns = {c["name"]: c for c in table.get("columns") or []}
        old_name, new_name = entry["from"], entry["to"]
        if old_name not in columns:
            raise SchemaError(f"renames V{version}: {entry['table']}.{old_name} 在上版本快照中不存在")
        if new_name in columns:
            raise SchemaError(f"renames V{version}: {entry['table']}.{new_name} 已存在，不能作为改名目标")
        columns[old_name]["name"] = new_name
        rendered.append((table["table_name"], old_name, new_name))
    return tables, rendered


def render_increment(version: int) -> tuple[str, str, str]:
    """按活动快照 + 改名登记渲染 V{version} 正向/回滚脚本（fail-closed）。"""
    prev_dir = ACTIVE_HISTORY_DIR / f"V{version - 1}"
    cur_dir = ACTIVE_HISTORY_DIR / f"V{version}"
    if not (ROOT / prev_dir).is_dir():
        raise SchemaError(f"活动迁移上版本快照不存在 —— 预期 {prev_dir}")
    if not (ROOT / cur_dir).is_dir():
        raise SchemaError(f"活动迁移快照缺失 —— 预期 {cur_dir}")
    prev_tables = load_tables_from_dir(str(prev_dir))
    cur_tables = load_tables_from_dir(str(cur_dir))
    if not prev_tables or not cur_tables:
        raise SchemaError(f"活动快照为空：{prev_dir} 或 {cur_dir}")
    prev_norm, renames = apply_column_renames(prev_tables, version)
    unsupported = unsupported_migration_changes(prev_norm, cur_tables)
    if unsupported:
        raise SchemaError(
            "检测到当前增量生成器不支持的结构变更："
                + "\n  - ".join(unsupported)
        )
    diff = diff_schemas(prev_norm, cur_tables)
    if not migration_diff_has_changes(diff) and not renames:
        raise SchemaError(f"V{version} 快照与 V{version - 1} 无差异（不应存在的空迁移）")
    dropped_columns = sum(len(change["dropped"]) for change in diff["changed_columns"].values())
    if diff["dropped_tables"] or dropped_columns:
        raise SchemaError(
            "检测到删表或删列变更；生成器不会生成可能丢失数据且无法由结构回滚恢复的迁移。"
            "请先制定数据保留/备份与显式迁移方案。"
        )
    tags: list[str] = []
    for table_full, _old_name, _new_name in renames:
        tags.append(short_name(table_full) + "_rename")
    for key in (
        "changed_columns",
        "added_checks",
        "added_uniques",
        "added_unique_partials",
        "added_indexes",
        "dropped_indexes",
        "added_fks",
    ):
        tags.extend(sorted(diff[key].keys()))
    if diff["changed_checks"]:
        # 同定义替换用约束名而非表名命名迁移文件，文件名直接说明改了什么
        tags.extend(
            sorted(
                item["name"]
                for change in diff["changed_checks"].values()
                for item in change["replaced"]
            )
        )
    if diff["added_tables"]:
        tags.extend(sorted("add_" + short_name(t["table_name"]) for t in diff["added_tables"]))
    tag = "_".join(tags[:2])[:50]
    name = f"V{version}__{tag}.sql"
    up_sql = render_migration_up(
        diff, version, version - 1, cur_tables, load_infra("postgresql"), renames
    )
    down_sql = render_migration_down(diff, version, version - 1, renames)
    return name, up_sql, down_sql


def build_outputs(tables: list[dict], dialect: str) -> dict[Path, str]:
    """收集该方言全部生成物：当前态渲染 + 冻结基线 + 增量迁移 + 下游类型。"""
    targets: dict[Path, str] = {}
    current_up, _ = render_dialect(tables, dialect)
    targets[ROOT / GENERATED_DIR / dialect / CURRENT_RENDER_NAME] = current_up

    if dialect == "postgresql":
        if not (ROOT / ACTIVE_HISTORY_DIR / "V1").is_dir():
            raise SchemaError("活动基线 V1 缺失；先运行 --initialize-active-baseline 冻结基线")
        v1_tables = load_tables_from_dir(str(ACTIVE_HISTORY_DIR / "V1"))
        v1_up, _ = render_dialect(v1_tables, "postgresql")
        targets[ROOT / "db/postgresql/V1__guliangiam_schema_init.sql"] = v1_up
        targets[ROOT / "internal/bootstrap/schema_sql/V1__guliangiam_schema_init.sql"] = v1_up
        for version in active_versions():
            if version < 2:
                continue
            name, up_sql, down_sql = render_increment(version)
            targets[ROOT / "db/postgresql" / name] = up_sql
            targets[ROOT / "internal/bootstrap/schema_sql" / name] = up_sql
            targets[ROOT / "db/rollback" / name.replace(".sql", ".down.sql")] = down_sql
    else:
        # 非 PostgreSQL 方言不维护增量链（ADR-023）：V1 即当前结构终态的重渲染。
        targets[ROOT / "db" / dialect / "V1__guliangiam_schema_init.sql"] = current_up

    targets[ROOT / GO_SCHEMA_PATH] = render_go_schema_types(tables)
    targets[ROOT / TS_SCHEMA_PATH] = render_ts_schema_types(tables)
    return targets


def write_outputs(tables: list[dict], dialect: str, root: Path, check_only: bool) -> int:
    try:
        targets = build_outputs(tables, dialect)
    except SchemaError as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1
    drift = []
    for path, content in targets.items():
        existing = path.read_text(encoding="utf-8") if path.exists() else None
        if existing != content:
            drift.append(path)
    if check_only:
        if drift:
            for path in drift:
                print(f"FAIL: 生成物与声明式 Schema 不一致 → {path}", file=sys.stderr)
            return 1
        print("PASS: 全部生成物（当前态/冻结基线/增量迁移/下游类型）零差异")
        return 0
    for path in drift:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(targets[path], encoding="utf-8")
        print(f"wrote {path.relative_to(root)}")
    if not drift:
        print("no changes — 生成物已是最新")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Schema generator (contracts/schema -> DDL)")
    parser.add_argument("--dialect", default="postgresql", choices=sorted(DIALECTS))
    parser.add_argument("--all", action="store_true", help="生成当前状态全部产物（兼容旧 flag）")
    parser.add_argument("--file", default=None, help="按文件名片段过滤（如 20_identity）；仅预览")
    parser.add_argument("--check", action="store_true", help="仅校验生成物零差异（CI 模式）")
    parser.add_argument("--version", type=int, default=1,
                        help="版本号：1=当前状态全量；2..N=生成/校验增量迁移（需活动快照 V{N-1}）")
    parser.add_argument(
        "--rebase-active-snapshot",
        action="store_true",
        help="纯组织重构（文件分组/拆分/改名）后重冻结最新活动快照：要求表级 diff 为空，不产生迁移",
    )
    parser.add_argument(
        "--initialize-active-baseline",
        action="store_true",
        help="把当前声明冻结为活动迁移链起点 V1（仅一次）",
    )
    args = parser.parse_args()

    tables = load_tables(args.file)
    if not tables:
        print("警告：未找到任何表定义", file=sys.stderr)
        return 1
    infra = load_infra(args.dialect)
    try:
        validate(tables, infra)
    except SchemaError as exc:
        print("FAIL: 声明式 Schema 自检未通过：", file=sys.stderr)
        print(exc, file=sys.stderr)
        return 1

    if args.file:
        up, _down = render_dialect(tables, args.dialect)
        print(up)
        return 0

    if args.rebase_active_snapshot:
        if args.check or args.file or args.version != 1 or args.all or args.initialize_active_baseline:
            print(
                "FAIL: --rebase-active-snapshot 必须单独使用，不能与 --check/--file/"
                "--version/--all/--initialize-active-baseline 组合。",
                file=sys.stderr,
            )
            return 1
        versions = active_versions()
        if not versions:
            print("FAIL: 活动快照链为空；先运行 --initialize-active-baseline。", file=sys.stderr)
            return 1
        latest = versions[-1]
        prev_tables = load_tables_from_dir(str(ACTIVE_HISTORY_DIR / f"V{latest}"))
        try:
            structural = unsupported_migration_changes(prev_tables, tables)
            diff = diff_schemas(prev_tables, tables)
        except SchemaError as exc:
            print(f"FAIL: {exc}", file=sys.stderr)
            return 1
        table_changes = migration_diff_has_changes(diff)
        if structural or table_changes:
            print(
                "FAIL: 活动快照与当前声明存在结构差异，rebase 只允许纯组织重构（文件分组/改名）。"
                "结构变更请使用 --version N 生成迁移。",
                file=sys.stderr,
            )
            for change in structural:
                print(f"  - {change}", file=sys.stderr)
            return 1
        snapshot_path = ROOT / ACTIVE_HISTORY_DIR / f"V{latest}"
        try:
            replace_snapshot(snapshot_path, sorted((ROOT / SCHEMA_DIR).glob("*.yaml")))
        except (OSError, ValueError) as exc:
            print(f"FAIL: 无法重冻结活动快照：{exc}", file=sys.stderr)
            return 1
        print(f"PASS: 活动快照 V{latest} 已按当前声明文件布局重冻结（表级 diff 为空，无新迁移）")
        return write_outputs(tables, "postgresql", ROOT, check_only=False)

    if args.initialize_active_baseline:
        if args.dialect != "postgresql" or args.check or args.file or args.version != 1 or args.all:
            print(
                "FAIL: --initialize-active-baseline 必须单独用于 PostgreSQL，且不能与 --check、"
                "--file、--all 或非默认 --version 组合。",
                file=sys.stderr,
            )
            return 1
        snapshot_path = ROOT / ACTIVE_HISTORY_DIR / "V1"
        if snapshot_path.exists():
            print(f"FAIL: 活动基线已存在，拒绝覆盖：{snapshot_path}", file=sys.stderr)
            return 1
        source_files = sorted((ROOT / SCHEMA_DIR).glob("*.yaml"))
        try:
            freeze_snapshot(snapshot_path, source_files)
        except (OSError, ValueError) as exc:
            print(f"FAIL: 无法冻结活动迁移基线：{exc}", file=sys.stderr)
            return 1
        print(f"PASS: 当前 Schema 已冻结为活动迁移基线 {snapshot_path.relative_to(ROOT)}")
        return write_outputs(tables, "postgresql", ROOT, check_only=False)

    if args.version >= 2:
        if args.dialect != "postgresql":
            print("FAIL: 增量迁移只支持 PostgreSQL；其他方言只维护当前态全量渲染。", file=sys.stderr)
            return 1
        latest = active_versions()[-1] if active_versions() else 0
        if args.check:
            if args.version < 2 or args.version > latest:
                print(
                    f"FAIL: --check 只能校验已存在的增量迁移（当前 V2..V{latest}，"
                    f"请求 V{args.version}）。",
                    file=sys.stderr,
                )
                return 1
        elif args.version != latest + 1:
            print(
                f"FAIL: 版本号必须连续：当前活动链最新为 V{latest}，"
                f"下一个只能是 V{latest + 1}（请求 V{args.version}）。",
                file=sys.stderr,
            )
            return 1
        if not (ROOT / ACTIVE_HISTORY_DIR / f"V{args.version - 1}").is_dir():
            print(f"FAIL: 上版本快照不存在：{ACTIVE_HISTORY_DIR}/V{args.version - 1}", file=sys.stderr)
            return 1
        if args.check:
            try:
                name, up_sql, down_sql = render_increment(args.version)
            except SchemaError as exc:
                print(f"FAIL: {exc}", file=sys.stderr)
                return 1
            up_paths = [
                ROOT / "db/postgresql" / name,
                ROOT / "internal/bootstrap/schema_sql" / name,
            ]
            down_path = ROOT / "db/rollback" / name.replace(".sql", ".down.sql")
            missing = [str(path) for path in [*up_paths, down_path] if not path.exists()]
            if missing:
                for path in missing:
                    print(f"FAIL: 缺少增量迁移生成物 → {path}", file=sys.stderr)
                return 1
            drift = [str(path) for path in up_paths if path.read_text() != up_sql]
            if down_path.read_text() != down_sql:
                drift.append(str(down_path))
            if drift:
                for path in drift:
                    print(f"FAIL: 增量迁移生成物与声明式 Schema 不一致 → {path}", file=sys.stderr)
                return 1
            print(f"PASS: 增量迁移 V{args.version} 零差异（{name} + 回滚）")
            return 0

        # 生成：先按 prev 快照 vs 当前声明验证/渲染，再冻结 V{N} 快照并落全部生成物。
        prev_tables = load_tables_from_dir(str(ACTIVE_HISTORY_DIR / f"V{args.version - 1}"))
        try:
            prev_norm, renames = apply_column_renames(prev_tables, args.version)
            unsupported = unsupported_migration_changes(prev_norm, tables)
            if unsupported:
                raise SchemaError(
                    "检测到当前增量生成器不支持的结构变更："
                + "\n  - ".join(unsupported)
                )
            diff = diff_schemas(prev_norm, tables)
            dropped_columns = sum(len(change["dropped"]) for change in diff["changed_columns"].values())
            if diff["dropped_tables"] or dropped_columns:
                raise SchemaError(
                    "检测到删表或删列变更；生成器不会生成可能丢失数据且无法由结构回滚恢复的迁移。"
                )
            if not migration_diff_has_changes(diff) and not renames:
                print(f"没有任何 schema 变更，V{args.version} 增量迁移不需要生成。")
                return 0
        except SchemaError as exc:
            print(f"FAIL: {exc}", file=sys.stderr)
            return 1
        cur_dir = ROOT / ACTIVE_HISTORY_DIR / f"V{args.version}"
        try:
            freeze_snapshot(cur_dir, sorted((ROOT / SCHEMA_DIR).glob("*.yaml")))
        except (OSError, ValueError) as exc:
            print(f"FAIL: 无法冻结 V{args.version} Schema 快照：{exc}", file=sys.stderr)
            return 1
        rc = write_outputs(tables, "postgresql", ROOT, check_only=False)
        if rc == 0:
            print(f"frozen {cur_dir.relative_to(ROOT)}")
        return rc

    rc = write_outputs(tables, args.dialect, ROOT, args.check)
    if rc == 0 and not args.check:
        print(
            f"schema={len(tables)} tables, "
            f"{sum(len(t['columns']) for t in tables)} columns, "
            f"active versions={active_versions()}"
        )
    return rc


if __name__ == "__main__":
    sys.exit(main())

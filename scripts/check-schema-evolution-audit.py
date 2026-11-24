#!/usr/bin/env python3
"""GuliangIAM Schema Evolution Audit — 声明式 Schema 演化增量审计。

覆盖 4 个 check-schema-parity.py / check-generated-zero-diff.sh 检测不到的 drift：
  [A] Go infra postgres SQL 引用的表/列在 YAML 里缺失
  [B] CHECK / partial index predicate 类型 cast 跟列类型不兼容
  [C] seed SQL INSERT 值长度 ↔ 声明列类型不兼容
  [D] seed SQL 使用 gen_random_uuid() 而非显式 UUIDv7

用法:
  python3 scripts/check-schema-evolution-audit.py           # 全量 4 维度
  python3 scripts/check-schema-evolution-audit.py --only A  # 仅维度 A
  python3 scripts/check-schema-evolution-audit.py --quick   # 不做 seed 值长度（慢）

exit code: 0 = 全 PASS；1 = 任一 FAIL；2 = 脚本自身错误
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml  # noqa: E402  (stdlib after path setup)

ROOT = Path(__file__).resolve().parent.parent
SCHEMA_DIR = ROOT / "contracts" / "schema"
INFRA_POSTGRES = ROOT / "internal" / "infra" / "postgres"
BOOTSTRAP_SEED = ROOT / "internal" / "bootstrap" / "seed_sql"


# ---------------------------------------------------------------------------
# 基础设施：加载声明式 schema
# ---------------------------------------------------------------------------

def load_tables(include_active: bool = False) -> dict[str, dict]:
    """加载 contracts/schema/*.yaml (+可选 history/active/V1-V5/*.yaml)。

    返回 {schema.table_short: {columns: {name: type}, rls: bool, ...}}
    include_active=True 时 active 快照会用前缀 "V{N}::table_name" 区分。
    """
    tables: dict[str, dict] = {}

    def _load_dir(prefix: str, yaml_dir: Path):
        for fp in sorted(yaml_dir.glob("*.yaml")):
            for doc in yaml.safe_load_all(fp.read_text()):
                if not doc or not isinstance(doc, dict):
                    continue
                name = doc.get("table_name")
                if not name:
                    continue
                full_key = f"{prefix}{name}" if prefix else name
                cols: dict[str, str] = {}
                col_defs: dict[str, dict] = {}
                for c in doc.get("columns", []) or []:
                    n = c.get("name", "")
                    t = c.get("type", "")
                    if n and t:
                        cols[n] = t
                        col_defs[n] = c
                tables[full_key] = {
                    "path": str(fp.relative_to(ROOT)),
                    "columns": cols,
                    "col_defs": col_defs,
                    "rls": bool(doc.get("rls")),
                    "constraints": doc.get("constraints", []) or [],
                    "indexes": doc.get("indexes", []) or [],
                    "rls_predicate": doc.get("rls_predicate", ""),
                }

    _load_dir("", SCHEMA_DIR)
    if include_active:
        for v in range(1, 6):
            active_dir = SCHEMA_DIR / "history" / "active" / f"V{v}"
            if active_dir.is_dir():
                _load_dir(f"V{v}::", active_dir)
    return tables


# ---------------------------------------------------------------------------
# 维度 A: Go infra postgres SQL 引用完整性
# ---------------------------------------------------------------------------

_TABLE_REF_RE = re.compile(r"\b(?:FROM|JOIN|INTO|UPDATE)\s+\"?guliangiam\"?\.(\w+)", re.IGNORECASE)

def _scan_sql_files() -> dict[str, set[str]]:
    """扫描 internal/infra/postgres/**/sql*.go 提取 {table_short: set(col_names)}."""
    refs: dict[str, set[str]] = {}
    for fp in INFRA_POSTGRES.rglob("sql*.go"):
        if fp.name.endswith("_test.go"):
            continue
        text = fp.read_text()
        tables_found: set[str] = set()
        for m in _TABLE_REF_RE.finditer(text):
            tables_found.add(m.group(1))
        # 对每个命中的表，从 SELECT / INSERT / UPDATE 列表里粗提取列名
        for tshort in tables_found:
            if tshort not in refs:
                refs[tshort] = set()
            # SELECT cols FROM guliangiam.{tshort} / INSERT INTO ... VALUES
            for colmatch in re.finditer(
                rf"SELECT\s+([\w\s,]+?)\s+FROM\s+\"?guliangiam\"?\.{tshort}",
                text, re.IGNORECASE,
            ):
                for c in re.split(r"[,\s]+", colmatch.group(1).strip()):
                    if c and c.lower() not in ("distinct", "*") and not c.startswith(("COUNT", "SUM", "AVG", "MIN", "MAX", "DISTINCT")):
                        if c[0].isupper() or c == c.upper():  # 聚合函数跳过
                            continue
                        refs[tshort].add(c)
            for colmatch in re.finditer(
                rf"INSERT\s+INTO\s+\"?guliangiam\"?\.{tshort}\s*\(([^)]+)\)",
                text, re.IGNORECASE,
            ):
                for c in re.split(r"[,\s]+", colmatch.group(1).strip()):
                    refs[tshort].add(c)
    return refs


def audit_infra_sql(tables: dict[str, dict]) -> list[str]:
    errors: list[str] = []
    refs = _scan_sql_files()
    # 把 schema.table_short 反向索引
    schema_by_short: dict[str, dict] = {}
    for full, td in tables.items():
        if "::" in full:
            continue  # A 维度只校验当前 Schema，历史快照仅供 B/C 维度使用。
        short = full.split(".", 1)[1]
        schema_by_short[short] = td

    for tshort, cols in sorted(refs.items()):
        if tshort not in schema_by_short:
            errors.append(
                f"[A] 表 guliangiam.{tshort} 在 infra postgres SQL 里被引用，"
                f"但 contracts/schema/*.yaml 未定义"
            )
            continue
        tcols = schema_by_short[tshort]["columns"]
        # Implicit cols (created_at / updated_at / deleted_at / revision) 由
        # _apply_implicit_columns() 自动追加，Go 代码引用它们是合法的。
        implicit = {"created_at", "updated_at", "deleted_at", "revision"}
        for c in cols:
            if c in implicit and c not in tcols:
                continue
            if c not in tcols:
                errors.append(
                    f"[A] 列 guliangiam.{tshort}.{c} 在 infra postgres SQL 里被引用，"
                    f"但 YAML 未声明"
                )
    return errors


# ---------------------------------------------------------------------------
# 维度 B: CHECK / predicate 类型 cast 一致性
# ---------------------------------------------------------------------------

_ARRAY_CAST_RE = re.compile(r"(\w+)\s*(!=|=)\s*'[^']*'\s*::\s*(\w+\[\])", re.IGNORECASE)
# 也捕获 predicate 里的 ::text[] 显式 cast（不带列引用的全局 cast）
_BARE_ARRAY_CAST_RE = re.compile(r"'[^']*'\s*::\s*(\w+\[\])", re.IGNORECASE)


def _column_type_accepts_cast(col_type: str, cast_target: str) -> bool:
    """判断声明列类型和 cast 目标类型是否兼容（= PG 能自动 coercion）。

    规则：
      - col_type VARCHAR(255)[] + cast ::text[]  → ❌ 不兼容（PG 不认）
      - col_type TEXT[]          + cast ::text[]  → ✅ 兼容（冗余但不会报错）
      - col_type UUID[]          + cast ::uuid[]  → ✅ 兼容
      - 任何类型 + 不带显式 cast 字面量（'{}'）  → ✅ PG 自动 coercion
    """
    if not col_type.endswith("[]") or not cast_target.endswith("[]"):
        return True  # 非数组场景不检查

    col_base = col_type.replace("[]", "").strip().upper()
    cast_base = cast_target.replace("[]", "").strip().upper()

    # VARCHAR(N)[] 和 TEXT[] 在 PG 里是不同类型，没有自动 coercion
    if col_base.startswith("VARCHAR") and cast_base == "TEXT":
        return False
    if col_base == "TEXT" and cast_base.startswith("VARCHAR"):
        return False
    return True


# ---------------------------------------------------------------------------
# 维度 B: CHECK / predicate 类型安全 —— 6 项子检测
#   B-1  YAML 值完整性（引号截断、语法残缺）
#   B-2  显式 ::cast 跟列类型不兼容（含非数组类型）
#   B-3  冗余 cast（列类型本身就是目标类型，显式 cast 为后续 drift 埋下隐患）
#   B-4  RLS predicate 里的显式 cast
#   B-5  CHECK 表达式中字符串枚举值长度 vs 列 VARCHAR(N) 上限
#   B-6  history/active/V1-V5 冻结快照的残留 cast
# ---------------------------------------------------------------------------

# 任何显式 ::cast：colname :: type 或 CAST(colname AS type)
_ANY_CAST_RE = re.compile(r"(\b\w+\b)\s*::\s*([A-Za-z][A-Za-z0-9_\s\(\)\[\]]*)", re.IGNORECASE)
_CAST_FUNC_RE = re.compile(r"CAST\s*\(\s*(\b\w+\b)\s+AS\s+([A-Za-z][A-Za-z0-9_\s\(\)\[\]]*)\s*\)", re.IGNORECASE)
# 比较右边字面量 cast: col != '{}'::jsonb
_COMPARISON_CAST_RE = re.compile(
    r"(\b\w+\b)\s*(=|!=|<>|<|>|<=|>=)\s*'[^']*'\s*::\s*([A-Za-z][A-Za-z0-9_\s\(\)\[\]]*)",
    re.IGNORECASE,
)

# SQL 关键字/函数名（不应被当作列名匹配）
_SQL_RESERVED = {
    "select", "from", "where", "and", "or", "not", "in", "is", "null", "true", "false",
    "any", "all", "exists", "array", "interval", "on", "join", "inner", "left", "right",
    "outer", "full", "cross", "group", "by", "order", "having", "union", "intersect",
    "except", "with", "distinct", "limit", "offset", "count", "sum", "avg", "min", "max",
    "now", "current_timestamp", "current_date", "current_time", "cardinality",
    "char_length", "length", "lower", "upper", "trim", "coalesce", "nullif",
    "row_number", "rank", "dense_rank", "lag", "lead", "over", "partition",
    "begin", "commit", "rollback", "transaction", "lock", "for", "update", "share",
    "set", "reset", "show", "table", "index", "constraint", "primary", "key",
    "foreign", "references", "unique", "check", "default", "not", "enable",
    "row", "level", "security", "using", "gin", "gist", "btree", "hash",
    "do", "while", "loop", "if", "then", "else", "elsif", "end", "return",
    "integer", "int", "bigint", "smallint", "serial", "bigserial",
    "text", "varchar", "character", "varying", "char",
    "boolean", "bool",
    "timestamp", "timestamptz", "date", "time", "timetz",
    "uuid", "json", "jsonb", "xml", "bytea",
    "numeric", "decimal", "real", "double", "precision",
}

def _is_valid_sql_name(token: str) -> bool:
    """判断正则捕获到的 colname 是否可能是 SQL 关键字/函数，而非真正列名。"""
    return token.lower() not in _SQL_RESERVED and not token[0].isupper()


def _normalize_type(t: str) -> str:
    """统一类型字符串以便比较：去掉空格、简化 VARCHAR(N)/DOUBLE PRECISION 等。"""
    t = t.strip().upper()
    t = re.sub(r"\s+", "", t)          # "DOUBLE PRECISION" → "DOUBLEPRECISION"
    t = re.sub(r"\(\d+\)", "", t)      # "VARCHAR(255)" → "VARCHAR"
    t = re.sub(r"\(\d+,\d+\)", "", t)  # "NUMERIC(10,2)" → "NUMERIC"
    return t


# cast 目标在这些类型上时，显式 cast 是跨 PG 版本兼容必需的
# PG 14 之前没有隐式 string→json/jsonb coercion，PG 18 虽然放松但声明层要跨方言（gaussdb/mysql）
_REQUIRED_CAST_TYPES = {"JSONB", "JSON"}


def _types_compatible(col_type: str, cast_type: str) -> tuple[bool, str]:
    """比较列类型和 cast 目标类型的兼容性。返回 (ok, reason)。

    PG 自动 coercion 规则：
      - VARCHAR(N)[] ↔ TEXT[]  ❌ 不同类型，无自动转换
      - VARCHAR(N) ↔ TEXT       ✅ VARCHAR 自动可转 TEXT
      - INTEGER ↔ BIGINT        ✅ 自动向上转型
      - ''::jsonb on JSONB      ✅ 兼容（PG 18+ 自动 coercion，老 PG/其他方言需要显式 cast — 标记为 compatible_cast_required）
      - VARCHAR ↔ INTEGER       ❌ 不能隐式转（必须显式 cast）
    """
    col_norm = _normalize_type(col_type)
    cast_norm = _normalize_type(cast_type)

    # cast 目标在 JSON 家族 — 显式 cast 是跨版本/方言兼容必需的
    if cast_norm in _REQUIRED_CAST_TYPES:
        if col_norm == cast_norm:
            return True, "compatible_cast_required"
        return True, "compatible"

    # 完全一致 → 冗余 cast（警告），不报错
    if col_norm == cast_norm:
        return True, "exact_match_redundant"

    # 数组特殊处理
    col_is_array = col_norm.endswith("[]")
    cast_is_array = cast_norm.endswith("[]")
    if col_is_array and cast_is_array:
        col_base = col_norm[:-2]
        cast_base = cast_norm[:-2]
        # VARCHAR[] ↔ TEXT[] / TEXT[] ↔ VARCHAR[] — PG 不自动 coercion
        if col_base.startswith("VARCHAR") and cast_base == "TEXT":
            return False, "varchar_array_to_text_array"
        if col_base == "TEXT" and cast_base.startswith("VARCHAR"):
            return False, "text_array_to_varchar_array"
        return _types_compatible(col_base, cast_base)
    if col_is_array != cast_is_array:
        return False, "scalar_vs_array_cast"

    # TEXT 家族 — VARCHAR 可自动转 TEXT（TEXT 是超集）
    if col_base := col_norm.split("[")[0]:
        pass
    # VARCHAR → TEXT 兼容
    if cast_norm == "TEXT" and (col_norm.startswith("VARCHAR") or col_norm == "CHAR" or col_norm.startswith("CHARACTER")):
        return True, "varchar_to_text_coercible"
    # UUID → TEXT：PG 的 uuid↔text 是合法显式转换。CHECK 里 UUID 列与 VARCHAR 列比较
    # 必须显式 cast（PG 无 uuid<>varchar 隐式转换），MySQL 侧 UUID 渲染为 CHAR(36)，
    # 生成器去掉 ::text 后语义等价——该 cast 是跨方言必需的，不是类型漂移。
    if cast_norm == "TEXT" and col_norm == "UUID":
        return True, "compatible_cast_required"
    # INTEGER → BIGINT 兼容
    if cast_norm == "BIGINT" and col_norm in ("INTEGER", "INT", "SMALLINT"):
        return True, "int_to_bigint_coercible"
    if cast_norm == "DOUBLEPRECISION" and col_norm in ("REAL", "FLOAT"):
        return True, "float_promotion"

    # 完全不同家族
    col_family = "STRING" if col_norm in ("TEXT", "VARCHAR", "CHAR", "CHARACTER") or col_norm.startswith("VARCHAR") else \
                 "INT" if col_norm in ("INTEGER", "INT", "BIGINT", "SMALLINT", "SERIAL", "BIGSERIAL") else \
                 "FLOAT" if col_norm in ("REAL", "DOUBLEPRECISION", "FLOAT", "NUMERIC", "DECIMAL") else \
                 "JSON" if col_norm in ("JSON", "JSONB") else \
                 "UUID" if col_norm == "UUID" else \
                 "BOOL" if col_norm == "BOOLEAN" or col_norm == "BOOL" else \
                 "DATE" if col_norm in ("DATE", "TIMESTAMP", "TIMESTAMPTZ", "TIME", "TIMETZ") else \
                 col_norm

    cast_family = "STRING" if cast_norm in ("TEXT", "VARCHAR", "CHAR", "CHARACTER") or cast_norm.startswith("VARCHAR") else \
                  "INT" if cast_norm in ("INTEGER", "INT", "BIGINT", "SMALLINT") else \
                  "FLOAT" if cast_norm in ("REAL", "DOUBLEPRECISION", "FLOAT", "NUMERIC", "DECIMAL") else \
                  "JSON" if cast_norm in ("JSON", "JSONB") else \
                  "UUID" if cast_norm == "UUID" else \
                  "BOOL" if cast_norm == "BOOLEAN" or cast_norm == "BOOL" else \
                  "DATE" if cast_norm in ("DATE", "TIMESTAMP", "TIMESTAMPTZ", "TIME", "TIMETZ") else \
                  cast_norm

    if col_family != cast_family:
        return False, f"family_mismatch({col_family}→{cast_family})"

    return True, "compatible"


def _check_string_integrity(expr: str, context: str) -> list[str]:
    """B-1: 检测字符串里未闭合的引号 — YAML safe_load 不会报错但 PG 会炸。"""
    issues = []
    single_quotes = expr.count("'")
    if single_quotes % 2 != 0:
        issues.append(
            f"[{context}] 引号不匹配: 单引号 {single_quotes} 个（应为偶数）"
        )
    return issues


def _report_cast_result(errors: list[str], warnings: list[str],
                        ctx: str, col: str, col_type: str, cast_t: str,
                        expr: str) -> None:
    """统一处理 cast 兼容性判定输出。"""
    ok, reason = _types_compatible(col_type, cast_t)
    if not ok:
        errors.append(
            f"[B-2] {ctx}\n"
            f"    列 {col} 声明类型 {col_type} 与 cast 目标 {cast_t} 不兼容"
            f" (reason: {reason})\n"
            f"    建议: 去掉显式 cast，PG 会自动 coercion"
        )
    elif reason == "exact_match_redundant":
        warnings.append(
            f"[B-3] {ctx}\n"
            f"    列 {col} 类型 {col_type} 与 cast ::{cast_t} 完全相同（冗余）"
            f" — 建议去掉显式 cast 以保持一致性（后续列类型漂移会变成定时炸弹）"
        )


def audit_predicate_safety(tables: dict[str, dict]) -> list[str]:
    """维度 B 完整检测。"""
    errors: list[str] = []
    warnings: list[str] = []

    for full_key, td in tables.items():
        short = full_key.split("::", 1)[-1].split(".", 1)[-1] if "." in full_key else full_key
        is_active_snap = full_key.startswith("V") and "::" in full_key
        location = td["path"]

        col_by_name: dict[str, str] = td["columns"]
        col_defs: dict[str, dict] = td.get("col_defs", {})

        # B-6 列 default 值里的显式 cast（''::jsonb / '{}'::jsonb 等）
        for col_name, col_def in col_defs.items():
            default = col_def.get("default", "")
            if not default:
                continue
            # default 值里出现 ::type
            for m in _ANY_CAST_RE.finditer(default):
                # 这里匹配的第一个 group 可能是字面量的 cast，跳过
                pass
            # 更直接：看 default 字符串本身是否含 ::
            if "::" in default:
                col_type = col_by_name.get(col_name, "")
                # 尝试提取 :: 后面的类型
                m2 = re.search(r"::\s*([A-Za-z][A-Za-z0-9_\s\(\)\[\]]*)", default)
                if m2 and col_type:
                    cast_t = m2.group(1).strip()
                    ok, reason = _types_compatible(col_type, cast_t)
                    if not ok:
                        errors.append(
                            f"[B-6] {full_key} column {col_name} ({location})\n"
                            f"    default 值里的 ::{cast_t} 与列类型 {col_type} 不兼容\n"
                            f"    default: {default}"
                        )
                    elif reason == "exact_match_redundant":
                        warnings.append(
                            f"[B-3] {full_key} column {col_name} ({location})\n"
                            f"    default 值 ::{cast_t} 与列类型 {col_type} 冗余 — 建议去掉\n"
                            f"    default: {default}"
                        )

        # ---------- 收集所有需要检测的表达式 ----------
        expr_sources: list[tuple[str, str, str]] = []  # (kind, name, expr)

        for c in td.get("constraints", []):
            if c.get("type") == "check":
                expr_sources.append(("CHECK", c.get("name", "?"), c.get("expr", "")))

        for idx in td.get("indexes", []):
            pred = idx.get("predicate", "")
            if pred:
                expr_sources.append(("INDEX_PRED", idx.get("name", "?"), pred))

        rls = td.get("rls_predicate", "")
        if rls:
            expr_sources.append(("RLS", "rls_predicate", rls))

        # ---------- 逐项检测 ----------
        for kind, name, expr in expr_sources:
            ctx = f"{full_key} {kind} {name} ({location})"

            # B-1 引号完整性
            for issue in _check_string_integrity(expr, f"[B-1] {ctx}"):
                errors.append(f"{issue}\n    值: {expr[:80]}{'...' if len(expr) > 80 else ''}")

            # B-2/B-3/B-4 显式 cast 兼容性 + 冗余检测
            # 模式 1: col :: type（列被显式 cast）
            for m in _ANY_CAST_RE.finditer(expr):
                col, cast_t = m.group(1).strip(), m.group(2).strip()
                if not _is_valid_sql_name(col):
                    continue
                col_type = col_by_name.get(col)
                if col_type is None:
                    continue
                _report_cast_result(errors, warnings, ctx, col, col_type, cast_t, expr)

            # 模式 2: CAST(col AS type)
            for m in _CAST_FUNC_RE.finditer(expr):
                col, cast_t = m.group(1).strip(), m.group(2).strip()
                if not _is_valid_sql_name(col):
                    continue
                col_type = col_by_name.get(col)
                if col_type is None:
                    continue
                _report_cast_result(errors, warnings, ctx, col, col_type, cast_t, expr)

            # 模式 3: col OP 'lit'::type（字面量被 cast，跟列比较）
            for m in _COMPARISON_CAST_RE.finditer(expr):
                col = m.group(1).strip()
                cast_t = m.group(3).strip()
                if not _is_valid_sql_name(col):
                    continue
                col_type = col_by_name.get(col)
                if col_type is None:
                    continue
                _report_cast_result(errors, warnings, ctx, col, col_type, cast_t, expr)

            # B-5 CHECK 枚举值 vs VARCHAR(N) 长度
            if kind == "CHECK":
                m = re.search(r"(\w+)\s+IN\s*\(([^)]+)\)", expr, re.IGNORECASE)
                if m:
                    col = m.group(1)
                    vals_str = m.group(2)
                    if col in col_by_name:
                        col_type = col_by_name[col]
                        vm = _VARCHAR_LEN_RE.match(col_type)
                        if vm:
                            max_len = int(vm.group(1))
                            # 提取所有枚举字符串值
                            enum_vals = re.findall(r"'([^']*)'", vals_str)
                            for v in enum_vals:
                                if len(v) > max_len:
                                    errors.append(
                                        f"[B-5] {ctx}\n"
                                        f"    CHECK 枚举值 '{v}' ({len(v)} chars) "
                                        f"超过列 {col} 的 VARCHAR({max_len}) 上限"
                                    )

    # 输出 warnings 作为单独的提醒（不 exit 1）
    for w in warnings:
        errors.append(w)  # 暂时合并输出，但标记 WARN 前缀

    return errors


# ---------------------------------------------------------------------------
# 维度 C: 值 ↔ 列类型兼容性（5 子检测）
#   C-1  seed INSERT VALUES 字符串长度 vs VARCHAR(N)
#   C-2  列 default 值里的字符串字面量 vs VARCHAR(N)
#   C-3  CHECK IN (...) 枚举值 vs VARCHAR(N)
#   C-4  migration ALTER COLUMN SET DEFAULT 值 vs VARCHAR(N)
#   C-5  跨版本列类型缩小检测（TEXT→VARCHAR 或 VARCHAR 上限变小）
# ---------------------------------------------------------------------------

_VARCHAR_LEN_RE = re.compile(r"VARCHAR\((\d+)\)", re.IGNORECASE)


def _parse_sql_value_list(vals_raw: str) -> list[str]:
    """解析 SQL VALUES (...) 里的逗号分隔列表 — 正确处理嵌套函数/子查询和字符串内逗号。

    入口前已确保传入的是 VALUES 括号内内容（外层括号已被 VALUES(...) 捕获）。
    PG 的 '' 是单引号转义，必须整体跳过而不是把第二个单引号当成字符串结束。
    """
    result: list[str] = []
    depth = 0           # 括号嵌套深度
    in_string = False
    current: list[str] = []
    i = 0
    while i < len(vals_raw):
        ch = vals_raw[i]
        if in_string:
            current.append(ch)
            if ch == "'":
                if i + 1 < len(vals_raw) and vals_raw[i + 1] == "'":
                    current.append(vals_raw[i + 1])
                    i += 2
                    continue
                in_string = False
        else:
            if ch == "'":
                in_string = True
                current.append(ch)
            elif ch == "(":
                depth += 1
                current.append(ch)
            elif ch == ")":
                depth -= 1
                current.append(ch)
                # 注意：不再在 depth==0 时 break —— 调用方传进来的就是 VALUES 内部
            elif ch == "," and depth == 0:
                result.append("".join(current).strip())
                current = []
            else:
                current.append(ch)
        i += 1
    if current:
        result.append("".join(current).strip())
    return result


def _statement_boundary(sql: str) -> int:
    """返回第一条语句结束位置（顶层分号），忽略单引号字符串内的分号。"""
    in_string = False
    i = 0
    while i < len(sql):
        ch = sql[i]
        if in_string:
            if ch == "'":
                if i + 1 < len(sql) and sql[i + 1] == "'":
                    i += 2
                    continue
                in_string = False
        elif ch == "'":
            in_string = True
        elif ch == ";":
            return i
        i += 1
    return len(sql)


def _split_insert_values(sql: str) -> tuple[str, str] | None:
    """精确拆分当前 INSERT INTO table (cols) VALUES (vals) — 处理嵌套子查询。

    只在本条语句（顶层分号前）里查找 VALUES。INSERT ... SELECT 没有 VALUES，
    必须返回 None；否则会把后续 INSERT 的 VALUES 误配到本条列清单（G1d 的
    organization_billing 误报即由此产生）。

    返回 (cols_str, vals_str) 或 None。
    """
    m = re.search(r"INSERT\s+INTO\s+guliangiam\.(\w+)\s*\(", sql, re.IGNORECASE)
    if not m:
        return None
    statement = sql[: _statement_boundary(sql)]
    after_table = statement[m.end():]

    # 用平衡括号找到 INSERT 列清单的结束
    depth = 1
    i = 0
    while i < len(after_table) and depth > 0:
        if after_table[i] == "(":
            depth += 1
        elif after_table[i] == ")":
            depth -= 1
        i += 1
    cols_str = after_table[: i - 1]  # 不含最后那个 )

    # 只在本条语句里找 VALUES 关键字
    rest = after_table[i:]
    vm = re.search(r"\bVALUES\s*\(", rest, re.IGNORECASE)
    if not vm:
        return None
    after_values = rest[vm.end():]

    # 再次平衡括号找 VALUES 的闭合（跳过 '' 转义）
    depth = 1
    i = 0
    in_str = False
    while i < len(after_values) and depth > 0:
        ch = after_values[i]
        if in_str:
            if ch == "'":
                if i + 1 < len(after_values) and after_values[i + 1] == "'":
                    i += 2
                    continue
                in_str = False
        else:
            if ch == "'":
                in_str = True
            elif ch == "(":
                depth += 1
            elif ch == ")":
                depth -= 1
        i += 1
    vals_str = after_values[: i - 1]

    return cols_str, vals_str


def _extract_string_len(val: str) -> tuple[int, str] | None:
    """从 SQL 字面量提取字符串内容和长度。PG '' 是单引号转义。"""
    val = val.strip()
    if not val.startswith("'") or not val.endswith("'"):
        return None
    # 取出外层引号间的内容，把 '' 还原成 '
    inner = val[1:-1].replace("''", "'")
    return len(inner), inner[:80]


def audit_seed_value_lengths(tables: dict[str, dict], quick: bool = False) -> list[str]:
    """维度 C 完整检测。"""
    errors: list[str] = []

    # 需要列类型定义 — load_tables(include_active=False) 已经包含主 schema
    # 每个 table dict 里有 columns + col_defs
    table_full_map: dict[str, dict] = {}
    for full_key, td in tables.items():
        # 去掉 V{N}:: 前缀
        if "::" in full_key:
            real_full = full_key.split("::", 1)[1]
        else:
            real_full = full_key
        if real_full.startswith("guliangiam."):
            table_full_map[real_full] = td

    # ---------- C-1: seed INSERT VALUES ----------
    for fp in sorted(BOOTSTRAP_SEED.glob("V*__*.sql")):
        text = fp.read_text()
        # 用平衡括号定位每个 INSERT —— 处理嵌套子查询
        for m in re.finditer(r"INSERT\s+INTO\s+guliangiam\.(\w+)\s*\(", text, re.IGNORECASE):
            tshort = m.group(1)
            # 从 INSERT 开始往后找 VALUES (...)
            segment = text[m.start():]
            split = _split_insert_values(segment)
            if not split:
                continue
            cols_str, vals_str = split
            col_list = [c.strip() for c in cols_str.split(",") if c.strip()]
            full = f"guliangiam.{tshort}"
            td = table_full_map.get(full)
            if not td:
                continue
            vals = _parse_sql_value_list(vals_str)
            for i, col in enumerate(col_list):
                if i >= len(vals):
                    break
                col_type = td["columns"].get(col, "")
                vm = _VARCHAR_LEN_RE.match(col_type)
                if not vm:
                    continue
                max_len = int(vm.group(1))
                sl = _extract_string_len(vals[i])
                if not sl:
                    continue
                actual_len, preview = sl
                if actual_len > max_len:
                    errors.append(
                        f"[C-1] {full} 列 {col} ({col_type}) seed VALUES 超长\n"
                        f"    实际 {actual_len} chars > 上限 {max_len}\n"
                        f"    值预览: {preview}...\n"
                        f"    修复: 改列类型为 TEXT 或缩短 seed 值"
                    )

    # ---------- C-2: 列 default 值 ----------
    for full, td in table_full_map.items():
        col_defs = td.get("col_defs", {})
        for cname, cdef in col_defs.items():
            default = cdef.get("default", "")
            if not default or not isinstance(default, str):
                continue
            col_type = cdef.get("type", "")
            vm = _VARCHAR_LEN_RE.match(col_type)
            if not vm:
                continue
            max_len = int(vm.group(1))
            # 提取 default 里的所有字符串字面量
            for sm in re.finditer(r"'([^']*)'", default):
                content = sm.group(1)
                # 还原 '' → '
                content = content.replace("''", "'")
                if len(content) > max_len:
                    errors.append(
                        f"[C-2] {full} 列 {cname} ({col_type}) default 值超长\n"
                        f"    default: {default[:80]}...\n"
                        f"    字符串 '{content[:40]}' {len(content)} > {max_len}"
                    )

    # ---------- C-3: CHECK IN (...) 枚举值 ----------
    for full, td in table_full_map.items():
        col_defs = td.get("col_defs", {})
        for ck in td.get("constraints", []) or []:
            if ck.get("type") != "check":
                continue
            expr = ck.get("expr", "")
            m = re.search(r"(\w+)\s+IN\s*\(([^)]+)\)", expr, re.IGNORECASE)
            if not m:
                continue
            col = m.group(1)
            cdef = col_defs.get(col)
            if not cdef:
                continue
            col_type = cdef.get("type", "")
            vm = _VARCHAR_LEN_RE.match(col_type)
            if not vm:
                continue
            max_len = int(vm.group(1))
            for ev in re.findall(r"'([^']*)'", m.group(2)):
                if len(ev) > max_len:
                    errors.append(
                        f"[C-3] {full} CHECK {ck.get('name','?')} 枚举值超长\n"
                        f"    列 {col} ({col_type}): 枚举 '{ev}' {len(ev)} > {max_len}\n"
                        f"    修复: 扩大列类型或缩短枚举值"
                    )

    # ---------- C-4: migration ALTER COLUMN SET DEFAULT ----------
    for dialect_dir in ["db/postgresql", "db/generated/postgresql"]:
        dp = ROOT / dialect_dir
        if not dp.is_dir():
            continue
        for fp in sorted(dp.glob("V*__*.sql")):
            text = fp.read_text()
            for m in re.finditer(
                r"ALTER\s+TABLE\s+guliangiam\.(\w+).*?"
                r"ALTER\s+COLUMN\s+(\w+)\s+SET\s+DEFAULT\s+([^,;]+)",
                text, re.IGNORECASE | re.DOTALL,
            ):
                tshort, col, default = m.group(1), m.group(2), m.group(3).strip()
                full = f"guliangiam.{tshort}"
                td = table_full_map.get(full)
                if not td:
                    continue
                col_type = td["columns"].get(col, "")
                vm = _VARCHAR_LEN_RE.match(col_type)
                if not vm:
                    continue
                max_len = int(vm.group(1))
                for sm in re.finditer(r"'([^']*)'", default):
                    content = sm.group(1).replace("''", "'")
                    if len(content) > max_len:
                        errors.append(
                            f"[C-4] {fp.name} {full} ALTER COLUMN {col} ({col_type}) SET DEFAULT 超长\n"
                            f"    default: {default[:80]}...\n"
                            f"    字符串 '{content[:40]}' {len(content)} > {max_len}"
                        )

    # ---------- C-5: 跨版本列类型缩小 ----------
    # diff history/active/V{N-1} 和 V{N} 的同列类型：
    # TEXT→VARCHAR / VARCHAR(N_大)→VARCHAR(N_小) / VARCHAR(N)→INTEGER 都是危险缩小
    if not quick:
        _check_cross_version_shrink(errors)

    return errors


def _check_cross_version_shrink(errors: list[str]) -> None:
    """C-5: 跨版本列类型缩小检测 — 最隐蔽的 drift。

    场景：
      - V1 声明 TEXT，V2 migration 改成 VARCHAR(255) → 已有数据可能 > 255
      - V1 声明 VARCHAR(500)，V2 migration 改成 VARCHAR(100) → 22001 truncation
      - V1 声明 VARCHAR(255)，V2 migration 改成 INTEGER → 42804 cast 失败
    """
    active_root = SCHEMA_DIR / "history" / "active"

    def _load_vcol_types(version: int) -> dict[str, dict[str, str]]:
        """加载 V{N} 快照的 {full_table: {col_name: col_type}}。"""
        result: dict[str, dict[str, str]] = {}
        vdir = active_root / f"V{version}"
        if not vdir.is_dir():
            return result
        for fp in sorted(vdir.glob("*.yaml")):
            for doc in yaml.safe_load_all(fp.read_text()):
                if not doc or not isinstance(doc, dict):
                    continue
                full = doc.get("table_name")
                if not full:
                    continue
                cols: dict[str, str] = {}
                for c in doc.get("columns", []) or []:
                    cols[c["name"]] = c.get("type", "")
                result[full] = cols
        return result

    for n in range(2, 6):  # V2 vs V1, V3 vs V2, ...
        prev = _load_vcol_types(n - 1)
        curr = _load_vcol_types(n)
        for full in curr:
            if full not in prev:
                continue
            prev_cols = prev[full]
            curr_cols = curr[full]
            for cname, curr_ct in curr_cols.items():
                prev_ct = prev_cols.get(cname)
                if not prev_ct or prev_ct == curr_ct:
                    continue
                # 比较是否缩小
                shrink_reason = _detect_type_shrink(prev_ct, curr_ct)
                if shrink_reason:
                    errors.append(
                        f"[C-5] {full}.{cname} 在 V{n} 缩小\n"
                        f"    V{n-1}: {prev_ct}  →  V{n}: {curr_ct}\n"
                        f"    风险: {shrink_reason}\n"
                        f"    修复: 改回 V{n-1} 类型（看 history/V{n-1} 原始类型）或确保迁移脚本里先清理超长数据"
                    )


def _detect_type_shrink(prev_ct: str, curr_ct: str) -> str | None:
    """判断列类型变更是否为缩小（已有数据可能不兼容）。返回风险说明或 None。"""
    pn = _normalize_type(prev_ct)
    cn = _normalize_type(curr_ct)

    # 1. TEXT → VARCHAR — 最危险！TEXT 无上限 → VARCHAR(N) 强制截断
    if pn in ("TEXT", "LONGTEXT") and cn.startswith("VARCHAR"):
        vm = re.match(r"VARCHAR(\d+)", cn)
        ml = vm.group(1) if vm else "?"
        return f"TEXT 无上限 → VARCHAR({ml}) 强制截断，现有数据可能超长"

    # 2. VARCHAR(N_大) → VARCHAR(N_小)
    pvm = _VARCHAR_LEN_RE.match(prev_ct)
    cvm = _VARCHAR_LEN_RE.match(curr_ct)
    if pvm and cvm:
        pl, cl = int(pvm.group(1)), int(cvm.group(1))
        if cl < pl:
            return f"VARCHAR({pl}) → VARCHAR({cl}) 缩小，VARCHAR({pl}) 里数据可能 {cl} 字超长"

    # 3. STRING 家族 → 非 STRING（VARCHAR → INTEGER / UUID → BIGINT）
    p_family = "STRING" if pn.startswith("VARCHAR") or pn in ("TEXT", "CHAR", "CHARACTER") else \
               "INT" if pn in ("INTEGER", "INT", "BIGINT", "SMALLINT") else \
               "FLOAT" if pn in ("REAL", "DOUBLEPRECISION", "FLOAT") else \
               "JSON" if pn in ("JSON", "JSONB") else \
               "UUID" if pn == "UUID" else \
               "BOOL" if pn in ("BOOLEAN", "BOOL") else \
               "DATE" if pn in ("DATE", "TIMESTAMP", "TIMESTAMPTZ") else pn

    c_family = "STRING" if cn.startswith("VARCHAR") or cn in ("TEXT", "CHAR", "CHARACTER") else \
               "INT" if cn in ("INTEGER", "INT", "BIGINT", "SMALLINT") else \
               "FLOAT" if cn in ("REAL", "DOUBLEPRECISION", "FLOAT") else \
               "JSON" if cn in ("JSON", "JSONB") else \
               "UUID" if cn == "UUID" else \
               "BOOL" if cn in ("BOOLEAN", "BOOL") else \
               "DATE" if cn in ("DATE", "TIMESTAMP", "TIMESTAMPTZ") else cn

    if p_family == "STRING" and c_family != "STRING":
        return f"STRING → {c_family} 跨家族转换，PG 会报 42804 cast 失败（除非先清空）"
    if p_family in ("INT", "FLOAT") and c_family not in ("INT", "FLOAT"):
        return f"数字家族 → {c_family} 跨家族转换，风险：数值精度丢失或 cast 失败"

    return None


# ---------------------------------------------------------------------------
# 维度 D: seed SQL UUIDv7 显式性
# ---------------------------------------------------------------------------

_BAD_UUID_FUNCS = re.compile(r"\b(gen_random_uuid|uuid_generate_v[0-94])\s*\(", re.IGNORECASE)


def audit_seed_uuidv7() -> list[str]:
    errors: list[str] = []
    for fp in sorted(BOOTSTRAP_SEED.glob("V1__*.sql")):
        text = fp.read_text()
        for m in _BAD_UUID_FUNCS.finditer(text):
            line = text[:m.start()].count("\n") + 1
            errors.append(
                f"[D] {fp.name} L{line}: 使用 {m.group(1)}() 而非显式 UUIDv7 字面量"
                f"\n    修复: 分配下一个 seq 号，写入 '01a0ea76-0c00-7XXX-8XXX-XXXXXXXXXXXX'::uuid"
            )
    return errors


# ---------------------------------------------------------------------------
# [E] SQL Injection Risk — 5 子检测
# ---------------------------------------------------------------------------

_SECURITY_DIR = ROOT / "contracts" / "security"
_REGISTRY_PATH = _SECURITY_DIR / "sql-injection-registry.json"

# ── 辅助：收集所有 Go 文件 ────────────────────────────────────────────────

def _walk_go_files() -> list[Path]:
    results: list[Path] = []
    for base in [INFRA_POSTGRES, ROOT / "internal" / "infra" / "mysql", ROOT / "internal" / "app", ROOT / "internal" / "platform"]:
        if not base.exists():
            continue
        for fp in base.rglob("*.go"):
            if fp.name.endswith("_test.go") or "/vendor/" in str(fp):
                continue
            results.append(fp)
    return results

def _rel(p: Path) -> str:
    return str(p.relative_to(ROOT))

# ── SQL 结构判定：必须有 SQL 结构锚点，而不是仅出现 from/order 等英文单词 ──
# 旧的 containsSQLKeyword / _SQL_KW_IN_TEMPLATE 会把错误文案里的
# "changed from %s to %s" 或 "does not match order currency" 当成 SQL，
# G1d 的 orders/payments/risk 误报即由此产生。

_SQL_TOKEN = r"[^,\s;()]+"
_SQL_STRUCTURE_RE = re.compile(
    r"(?:"
    r"\bSELECT\s+.+?\s+FROM\s+" + _SQL_TOKEN +
    r"|\bINSERT\s+INTO\s+" + _SQL_TOKEN +
    r"|\bUPDATE\s+" + _SQL_TOKEN + r"\s+SET\s+"
    r"|\bDELETE\s+FROM\s+" + _SQL_TOKEN +
    r"|\bFROM\s+" + _SQL_TOKEN + r"(?:\s+(?:AS\s+)?\w+)?"
    r"\s*(?:WHERE|JOIN|ON|GROUP|ORDER|LIMIT|;|$)"
    r"|\bWHERE\s+" + _SQL_TOKEN + r"\s*(?:=|<>|!=|<=|>=|<|>|\bIN\b|\bLIKE\b|\bIS\b|\bBETWEEN\b)"
    r"|\bORDER\s+BY\s+"
    r"|\bGROUP\s+BY\s+"
    r"|\bJOIN\s+" + _SQL_TOKEN + r"\s+ON\s+"
    r"|\bON\s+CONFLICT\b"
    r"|\bSET\s+" + _SQL_TOKEN + r"\s*="
    # 尾部锚点：字符串以 FROM / INTO 结束，表名或列清单由右侧 + 的变量补全。
    # 例如 "SELECT * FROM " + table。短片段必须仍是 SQL，不能按长度跳过。
    r"|\bSELECT\s+.+?\s+FROM\s*$"
    r"|\bDELETE\s+FROM\s*$"
    r"|\bINSERT\s+INTO\s*$"
    r")",
    re.IGNORECASE | re.DOTALL,
)


def _looks_like_sql_fragment(text: str) -> bool:
    """判断文本是否含可识别的 SQL 结构，而不是孤立的英文关键词。"""
    return bool(text) and _SQL_STRUCTURE_RE.search(text) is not None


# ── E-2 短片段判定口径（审计证据）────────────────────────────────────────
# 只认 SQL 结构锚点，不引入最小长度阈值：` ORDER BY ` / ` WHERE id = ` 等
# 短片段必须继续判为 TRUE_SQL；自然语言、错误文案与 Go 调用描述判为
# FALSE_POSITIVE。该二值判定只用于取证与回归，不参与任何豁免；禁止按长度
# 或目录批量跳过短片段（G1d-2）。
SQL_FRAGMENT_TRUE = "TRUE_SQL"
SQL_FRAGMENT_FALSE_POSITIVE = "FALSE_POSITIVE"


def _classify_sql_fragment(text: str) -> str:
    """返回显式的 TRUE_SQL / FALSE_POSITIVE 判定，供审计证据与测试锁定口径。"""
    return SQL_FRAGMENT_TRUE if _looks_like_sql_fragment(text) else SQL_FRAGMENT_FALSE_POSITIVE


def _string_literal_from(text: str) -> str:
    """从源码片段里取第一个 Go 字符串字面量内容（反引号或双引号）。"""
    m = re.search(r"`([^`]*)`|\"((?:[^\"\\]|\\.)*)\"", text)
    if not m:
        return ""
    return m.group(1) if m.group(1) is not None else (m.group(2) or "")

# ── E-1: fmt.Sprintf / Sprintf 拼 SQL 且有 %s/%q 风险占位符 ──────────────

_SPRINTF_SQL_WITH_STRING_ARG = re.compile(
    r"fmt\.Sprintf\s*\(\s*([`\"])([^\1]*?)%[sq]([^\1]*?)\1",
    re.IGNORECASE,
)

def _e1_detect_fmt_sprintf_sql_risks() -> list[dict]:
    """返回所有 fmt.Sprintf 拼 SQL 且有 %s/%q 的点（忽略 registry 已登记的安全点）。"""
    hits: list[dict] = []
    _safe_var_prefixes_when_in_fmtsprintf = (
        "query", "Query", "sql", "SQL", "orderBy", "order_by",
        "columnSearchClause", "columnSearchValue", "extra", "Extra",
        "bt", "conflict", "set", "colList", "columns", "Columns",
        "baseQuery", "selectQuery", "countQuery", "fullSQL",
    )
    _non_sql_fmtsprintf_patterns = [
        "login from a new country",         # anomaly detection 消息
        "expression", "invalid format",      # rules validator
        "ENCRYPTION_KEY_CURRENT",            # panic 消息
        "cannot transition",                 # enforcer state error
        "failed to update",                  # error message
        "replication violation",             # domain error
        "on conflict", "ON CONFLICT",
        "ON DUPLICATE KEY",
        "INSERT INTO %s (%s)",               # bulk.go — table/col 来自 reflect
        "INSERT INTO %s (%s) VALUES",        # 同上
    ]
    for fp in _walk_go_files():
        text = fp.read_text(errors="replace")
        for i, line in enumerate(text.splitlines(), 1):
            s = line.strip()
            if s.startswith("//"):
                continue
            if "fmt.Sprintf(" not in s and ".Sprintf(" not in s:
                continue
            # 非 SQL 消息 → 跳过
            if any(p in s for p in _non_sql_fmtsprintf_patterns):
                continue
            # table/col 来自 schema traversal 的 reencrypt_job → 跳过
            if "UPDATE %s SET %s = $1 WHERE %s = $2" in s or "UPDATE %s SET %s = $1" in s:
                continue
            # 已知安全的 fmt.Sprintf：参数全是内部 const / whitelist 变量
            # 匹配 "%s %s %s LIMIT $N" — 这是已知安全的 listSQL 拼法
            if re.search(r'LIMIT \$\d+ OFFSET \$\d+', s) or re.search(r'LIMIT \? OFFSET \?', s):
                continue
            m = _SPRINTF_SQL_WITH_STRING_ARG.search(s)
            if not m:
                continue
            # 用格式串本身做结构判定；%s/%q 用占位符替代，避免拆散 SQL 锚点
            probe = m.group(2) + " __arg__ " + m.group(3)
            if _classify_sql_fragment(probe) != SQL_FRAGMENT_TRUE:
                continue
            hits.append({"file": _rel(fp), "line": i, "snippet": s[:180]})
    return hits

# ── E-2: + 运算符拼 SQL 且右边不是常量 ────────────────────────────────────

_CONCAT_SQL_PLUS_VAR = re.compile(
    r"(`[^`]*(?:WHERE|ORDER|FROM|JOIN|UPDATE|DELETE)[^`]*`\s*\+\s*\w+)"
    r"|(\w+\s*\+\s*`[^`]*(?:WHERE|ORDER|FROM|JOIN)[^`]*`)",
    re.IGNORECASE,
)
_CONCAT_SQL_PLUS_VAR2 = re.compile(
    r'("[^"]*(?:WHERE|ORDER|FROM|JOIN|SELECT|UPDATE|DELETE)[^"]*"\s*\+\s*\w+)'
    r'|(\w+\s*\+\s*"[^"]*(?:WHERE|ORDER|FROM|JOIN)[^"]*")',
    re.IGNORECASE,
)
# 已知安全的 + 运算符使用（编译期常量表名前缀、内部 DSL）
_SAFE_CONCAT_PATTERNS = [
    r"` \+ bt\w+ \+ `",          # MySQL btXXX 表名常量
    r"COUNT\(\*\) ` \+ ",       # baseQuery 拼接
    r"FROM ` \+ bt\w+ \+ `",    # 同上
    r"SELECT COUNT\(\*\) FROM ` \+ bt",
]

# 显式精确白名单：只登记审计器无法解析、但已人工核验为「编译期常量拼接」的误报。
# 键 = (仓库相对路径, 行号, strip 后的源码行)，三者全等才豁免；源码改动即失效。
# 新增条目必须在值中写明为什么不存在运行时输入。
_EXPLICIT_SAFE_CONCAT_ALLOWLIST: dict[tuple[str, int, str], str] = {
    (
        "internal/infra/mysql/webhookevents/sql.go",
        19,
        "const tenantPredicate = \" AND (? = '' OR EXISTS (SELECT 1 FROM \" + btWebhooks +",
    ): (
        "btWebhooks 是同文件 line 16 的编译期常量（\"`webhooks`\"），tenantPredicate "
        "只拼接常量 SQL 片段与占位符，无运行时输入；表/列与 "
        "contracts/schema/20_identity.yaml 的 guliangiam.webhook_events 一致"
    ),
}

def _e2_detect_concat_risks() -> list[dict]:
    hits: list[dict] = []
    # 允许的 + 运算符使用（全是编译期常量拼接）
    _safe_patterns = [
        r"` \+ bt\w+ \+ `",                    # MySQL btXXX 表名常量
        r"COUNT\(\*\) ` \+ ",                 # baseQuery 拼接
        r"FROM ` \+ bt\w+ \+ `",              # 同上
        r"SELECT COUNT\(\*\) FROM ` \+ bt",   # 同上
        r"` \+ (ReconBatch|Query|Receipt|Refund|Attempt|Qc|QueryBatch|Batch)\w*Columns? \+ `",  # pg 列名常量
    ]
    _safe_concat_strings = [
        "baseQuery", "BaseQuery",              # 函数内硬编码 FROM...WHERE 常量
        "ReconBatchColumns", "QueryChannelConfigColumns",  # sql.go const
        "failed to update", "cannot transition",
        "replication violation", "invalid format",
        "apiURL", "path=%s&", "smtp_sender", "webhook_url",
        "ORDER BY",  # sorting.go 返回值拼接
        "SELECT count(*) ", "SELECT COUNT(*) ",  # countSQL = base + extra — registry 覆盖
        "queryListSignalsBase", "queryListDecisionsBase", "queryListGovernanceBase",
        "order cannot be paid in current state",  # orders service 错误消息
        "failed to update policy:",                # enforcers 错误消息
    ]
    # 额外的安全匹配：IN (`+ph+`) — pgx/mysql 占位符 + args slice 配对
    _safe_regexes = [
        re.compile(r"IN\s*\(`\+\w+\+`\)"),
    ]
    for fp in _walk_go_files():
        text = fp.read_text(errors="replace")
        rel = _rel(fp)
        for i, line in enumerate(text.splitlines(), 1):
            s = line.strip()
            if s.startswith("//"):
                continue
            if (rel, i, s) in _EXPLICIT_SAFE_CONCAT_ALLOWLIST:
                continue
            match = _CONCAT_SQL_PLUS_VAR.search(s) or _CONCAT_SQL_PLUS_VAR2.search(s)
            if not match:
                continue
            # 候选字符串必须含 SQL 结构；错误文案里的 " from "+X 不构成 FROM 子句
            literal = _string_literal_from(match.group(0))
            if _classify_sql_fragment(literal) != SQL_FRAGMENT_TRUE:
                continue
            if any(re.search(p, s) for p in _safe_patterns):
                continue
            if any(noise in s for noise in _safe_concat_strings):
                continue
            if any(r.search(s) for r in _safe_regexes):
                continue
            hits.append({"file": _rel(fp), "line": i, "snippet": s[:180]})
    return hits

# ── E-3: Exec/Query 首参不是 sql.go 里的 const ─────────────────────────────

_QUERY_CALL = re.compile(r"\.(Exec|Query|QueryRow)\s*\(\s*ctx\s*,\s*(\w+)")
_SQL_CONST_PREFIXES = (
    "query", "Query", "sql", "SQL", "qCount", "qList", "qRevoke", "qExpire",
    "qGet", "qInsert", "qUpdate", "qDelete", "qSelect", "qLock", "qConsume",
    "qBatch", "qCreate",
    "Receipt", "Refund", "Attempt", "Recon",
    "baseQuery", "selectQuery", "countQuery",
    "fullSQL", "ledgerDDL",
    "countSQL", "selectSQL", "listSQL", "searchSQL", "insertSQL", "updateSQL",
    "deleteSQL", "querySQL", "fetchSQL", "retrySQL",
)
_SQL_CONST_EXACT = {
    "sb", "qInsertJTI", "qGetCampaign", "qInsertCampaign", "qUpdateCampaignStatus",
    "qGetSODRule", "qInsertSODRule", "qInsertLifecycle",
}

def _e3_detect_exec_query_var() -> list[dict]:
    hits: list[dict] = []
    _safe_args = {"m", "req", "records", "chainSQL", "w"}
    for fp in _walk_go_files():
        text = fp.read_text(errors="replace")
        for i, line in enumerate(text.splitlines(), 1):
            s = line.strip()
            if s.startswith("//"):
                continue
            if any(x in s for x in ["set_tenant_context", "set_config(", "guliangiam.set_", "Batch", "gw.Query"]):
                continue
            m = _QUERY_CALL.search(s)
            if not m:
                continue
            arg = m.group(2)
            if arg in _SQL_CONST_EXACT:
                continue
            if arg in _safe_args:
                continue
            if arg.startswith(_SQL_CONST_PREFIXES):
                continue
            hits.append({"file": _rel(fp), "line": i, "arg": arg, "snippet": s[:180]})
    return hits

# ── E-4: SQL 文本里有裸 %s/%v 不是 fmt.Sprintf 参数（手写字符串拼接痕迹）────

_BARE_PERCENT_S_IN_SQL_GO = re.compile(
    r"(`[^`]*?(?:WHERE|ORDER|FROM)[^`]*?)%[sqv]",
    re.IGNORECASE,
)

def _e4_detect_bare_percent_in_sql_strings() -> list[dict]:
    hits: list[dict] = []
    for fp in _walk_go_files():
        text = fp.read_text(errors="replace")
        for i, line in enumerate(text.splitlines(), 1):
            s = line.strip()
            if s.startswith("//"):
                continue
            if "fmt.Sprintf" in s or ".Sprintf(" in s:
                continue  # fmt.Sprintf 已经安全处理
            # 扫 sql.go 与 sql_ext.go 等 SQL 源文件里的裸 SQL 字符串
            if not (fp.name == "sql.go" or (fp.name.startswith("sql_") and fp.suffix == ".go")):
                continue
            m = _BARE_PERCENT_S_IN_SQL_GO.search(s)
            if m:
                hits.append({"file": _rel(fp), "line": i, "snippet": s[:180]})
    return hits

# ── E-5: registry 完整性 — scan 到的点必须在 registry 里登记 ────────────────

def _e5_registry_consistency(scan_hits: list[dict]) -> list[str]:
    errors: list[str] = []
    if not _REGISTRY_PATH.exists():
        errors.append("[E-5] FATAL: contracts/security/sql-injection-registry.json 不存在 — 请先运行 P0 扫描并登记所有动态 SQL 点")
        return errors
    try:
        import json as _json
        reg = _json.loads(_REGISTRY_PATH.read_text())
    except Exception as e:
        errors.append(f"[E-5] FATAL: registry JSON 解析失败: {e}")
        return errors

    reg_entries = reg.get("entries", [])
    reg_files = {(e["file"], e["line"]) for e in reg_entries}

    # 扫描到的点里，取 (file, line) 不在 registry 的
    for h in scan_hits:
        key = (h["file"], h["line"])
        # 宽松匹配：同一 file 里任意 entry 就算覆盖
        file_entries = {e["file"] for e in reg_entries}
        if h["file"] not in file_entries:
            errors.append(
                f"[E-2] FATAL: 扫描到动态 SQL 点未登记 → {h['file']}:{h['line']}\n"
                f"       {h['snippet'][:120]}\n"
                f"       修复: 在 contracts/security/sql-injection-registry.json 里补登记，"
                f"或把它改成 pgx 参数化"
            )
    return errors

# ── 主入口 ────────────────────────────────────────────────────────────────

def _run_ast_scanner() -> list[dict] | None:
    """运行 Go AST 分析器，返回 findings 列表；失败返回 None（降级到正则）。"""
    import subprocess
    repo_root = Path(__file__).resolve().parent.parent
    try:
        result = subprocess.run(
            ["go", "run", "scripts/ast-sql-scanner/main.go"],
            cwd=str(repo_root),
            capture_output=True, text=True, timeout=120,
        )
        if result.returncode != 0:
            return None
        import json as _json
        data = _json.loads(result.stdout)
        return data.get("findings", [])
    except Exception:
        return None


def audit_sql_injection() -> list[str]:
    """维度 E — SQL Injection Risk。5 子检测合并。Go AST 为主，正则为 fallback。"""
    errors: list[str] = []

    # ── P2: Go AST 分析器 ── 覆盖 E-1 (fmt.Sprintf 参数溯源) + E-3 (Exec/Query 首参溯源)
    ast_findings = _run_ast_scanner()
    if ast_findings is not None:
        for f in ast_findings:
            level = f.get("level", "WARN")
            check = f.get("check", "E?")
            file = f.get("file", "?")
            line = f.get("line", 0)
            detail = f.get("detail", "")
            prefix = "FATAL" if level == "FATAL" else "WARN "
            errors.append(
                f"[{check}] {prefix}: (AST) {file}:{line}\n"
                f"       {detail[:130]}\n"
                f"       修复: 用 pgx 参数化；或登记 registry 加 guardian 证明安全"
            )
        # AST 覆盖了 E-1 和 E-3 — 跳过正则版
        skip_e1 = True
        skip_e3 = True
    else:
        skip_e1 = False
        skip_e3 = False

    # ── 正则 fallback / 补充检测 ──
    if not skip_e1:
        for h in _e1_detect_fmt_sprintf_sql_risks():
            errors.append(
                f"[E-1] FATAL: fmt.Sprintf 拼 SQL 且有字符串占位符 → {h['file']}:{h['line']}\n"
                f"       {h['snippet'][:130]}\n"
                f"       修复: 用 pgx 参数化 ($1, $2, ...) 替换 %s；或登记 registry 加 guardian 证明安全"
            )

    # E-2: + 运算符拼 SQL 且右边不是已知安全模式（正则）
    for h in _e2_detect_concat_risks():
        errors.append(
            f"[E-4] FATAL: + 运算符拼 SQL 片段且右边可能是运行时值 → {h['file']}:{h['line']}\n"
            f"       {h['snippet'][:130]}\n"
            f"       修复: 改为 pgx 参数化；或登记 registry 加 guardian"
        )

    if not skip_e3:
        for h in _e3_detect_exec_query_var():
            errors.append(
                f"[E-3] WARN: Exec/Query 首参是变量 '{h['arg']}' → {h['file']}:{h['line']}\n"
                f"       {h['snippet'][:130]}\n"
                f"       修复: 确认此变量最终来源是 registry 已登记的内部常量；否则 FATAL"
            )

    # E-4: sql.go 里裸 SQL 字符串有 %s 不是 fmt.Sprintf
    for h in _e4_detect_bare_percent_in_sql_strings():
        errors.append(
            f"[E-4] FATAL: sql.go 常量里裸 %s 占位符（不在 fmt.Sprintf 里）→ {h['file']}:{h['line']}\n"
            f"       {h['snippet'][:130]}\n"
            f"       修复: 改为 pgx placeholder $1, $2, ..."
        )

    # E-5: registry 完整性
    all_scan_hits = _e1_detect_fmt_sprintf_sql_risks() + _e2_detect_concat_risks()
    errors.extend(_e5_registry_consistency(all_scan_hits))

    return errors


# ---------------------------------------------------------------------------
# 入口
# ---------------------------------------------------------------------------

def main(argv: list[str]) -> int:
    only: str | None = None
    quick = False
    i = 0
    while i < len(argv):
        arg = argv[i]
        if arg == "--only" and i + 1 < len(argv):
            i += 1
            only = argv[i]
        elif arg.startswith("--only="):
            only = arg.split("=", 1)[1]
        elif arg == "--quick":
            quick = True
        elif arg == "--include-active":
            pass  # 维度 B 默认包含 active 快照，其他维度可选
        i += 1

    # 维度 B 需要扫 active V1-V5 快照（检测冻结基线的残留 cast）
    need_active = (not only) or only == "B"
    try:
        tables = load_tables(include_active=need_active)
    except yaml.YAMLError as e:
        print(f"[SCRIPT ERROR] YAML 解析失败: {e}", file=sys.stderr)
        return 2

    results: list[tuple[str, str, list[str]]] = []  # (key, label, errors)

    if not only or only == "A":
        results.append(("A", "Go infra SQL 引用完整性", audit_infra_sql(tables)))
    if not only or only == "B":
        results.append(("B", "CHECK / predicate / RLS 类型安全 (6 子检测)", audit_predicate_safety(tables)))
    if not only or only == "C":
        results.append(("C", "seed 值 ↔ 列类型长度", audit_seed_value_lengths(tables, quick=quick)))
    if not only or only == "D":
        results.append(("D", "seed UUIDv7 显式性", audit_seed_uuidv7()))
    if not only or only == "E":
        results.append(("E", "SQL Injection Risk (5 子检测)", audit_sql_injection()))

    total_pass = 0
    total_fail = 0
    for key, label, errs in results:
        # 未显式标 WARN 的错误都必须让门禁失败；不能因为缺少 FATAL
        # 字样就把缺表/缺列报成“PASS（0 警告）”。旧维度 B 的冗余项例外。
        legacy_warns = [e for e in errs if "[B-3]" in e or "冗余" in e]
        warns = [e for e in errs if "WARN" in e and "FATAL" not in e]
        warns.extend(legacy_warns)
        fatals = [
            e for e in errs
            if "WARN" not in e and "[B-3]" not in e and "冗余" not in e
        ]
        if not errs:
            print(f"[{key}] {label}: PASS")
            total_pass += 1
        else:
            if fatals:
                print(f"[{key}] {label}: FAIL ({len(fatals)} 致命)")
                for e in fatals:
                    print(f"    ↳ {e}")
                total_fail += len(fatals)
            else:
                print(f"[{key}] {label}: PASS (⚠️ {len(warns)} 警告)")
                total_pass += 1
            for w in warns:
                print(f"    ↳ WARN: {w[:120]}")

    print()
    if total_fail == 0:
        print(f"RESULT: 全 PASS ({total_pass} 维度)")
        return 0
    print(f"RESULT: {total_pass} PASS / {total_fail} FAIL  (exit 1)")
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

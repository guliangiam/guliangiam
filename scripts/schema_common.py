#!/usr/bin/env python3
"""schema_common.py — 声明式 Schema（contracts/schema/*.yaml）与 DDL 的共享解析工具。

唯一事实数据源：contracts/schema/*.yaml（含 00_infra.yaml 基础设施声明）。
DDL（internal/bootstrap/schema_sql/*.sql）是生成物，本模块同时提供其健壮解析能力，供
gen_schema.py（生成）与 check-schema-parity.py（校验）复用，避免两套解析逻辑漂移。

输入：contracts/schema/*.yaml、internal/bootstrap/schema_sql/*.sql
输出：暴露 parse_yaml_schema()、parse_ddl()、index_names_of() 等函数
--check 行为：无 --check；库模块
消费方：被 gen_schema.py 与 check-schema-parity.py import；不纳入 verify-docs.sh
"""
from __future__ import annotations

import glob
import re
from pathlib import Path

import yaml

SCHEMA_DIR = Path("contracts/schema")
INFRA_FILE = SCHEMA_DIR / "00_infra.yaml"

NON_COLUMN_PREFIXES = (
    "CONSTRAINT",
    "PRIMARY KEY",
    "UNIQUE",
    "CHECK",
    "FOREIGN KEY",
    "EXCLUDE",
    "LIKE",
)


# --------------------------------------------------------------------------- #
# 词法工具：剥离注释 / 字符串，括号配平                                    #
# --------------------------------------------------------------------------- #
def strip_sql_comments(text: str) -> str:
    """剥离 `--` 行注释与 /* */ 块注释，保留字符串字面量内部内容。"""
    out = []
    i, n = 0, len(text)
    while i < n:
        ch = text[i]
        if ch == "'":
            j = i + 1
            while j < n:
                if text[j] == "'":
                    if j + 1 < n and text[j + 1] == "'":
                        j += 2
                        continue
                    break
                j += 1
            out.append(text[i : j + 1])
            i = j + 1
        elif ch == "-" and text.startswith("--", i):
            j = text.find("\n", i)
            i = n if j == -1 else j
        elif ch == "/" and text.startswith("/*", i):
            j = text.find("*/", i + 2)
            i = n if j == -1 else j + 2
        else:
            out.append(ch)
            i += 1
    return "".join(out)


def extract_balanced(text: str, open_index: int) -> str:
    """返回 text[open_index] 所指 '(' 起始的配对括号体（不含最外层括号）。"""
    depth = 0
    i, n = open_index, len(text)
    while i < n:
        ch = text[i]
        if ch == "'":
            j = i + 1
            while j < n and text[j] != "'":
                j += 2 if text[j] == "'" and j + 1 < n and text[j + 1] == "'" else 1
            i = j + 1
            continue
        if text.startswith("--", i):
            j = text.find("\n", i)
            i = n if j == -1 else j
            continue
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
            if depth == 0:
                return text[open_index + 1 : i]
        i += 1
    raise ValueError("unbalanced parentheses")


def split_top_level(body: str, sep: str = ",") -> list[str]:
    """按 depth-0 的分隔符切分（字符串字面量中的分隔符与括号内分隔符不受影响）。"""
    parts, depth, current = [], 0, []
    i, n = 0, len(body)
    while i < n:
        ch = body[i]
        if ch == "'":
            j = i + 1
            while j < n:
                if body[j] == "'":
                    if j + 1 < n and body[j + 1] == "'":
                        j += 2
                        continue
                    break
                j += 1
            current.append(body[i : j + 1])
            i = j + 1
            continue
        if ch in "([":
            depth += 1
        elif ch in ")]":
            depth -= 1
        if ch == sep and depth == 0:
            parts.append("".join(current))
            current = []
        else:
            current.append(ch)
        i += 1
    if "".join(current).strip():
        parts.append("".join(current))
    return parts


def normalize_space(text: str) -> str:
    return re.sub(r"\s+", " ", text).strip()


# --------------------------------------------------------------------------- #
# 声明式 Schema 加载                                                          #
# --------------------------------------------------------------------------- #
def load_infra(dialect: str = "postgresql") -> dict:
    """加载 00_infra.yaml；不存在时返回最小空结构。"""
    if not INFRA_FILE.exists():
        return {"schema": "guliangiam", "extensions": [], "functions": [], "triggers": [], "rls": {}, "roles": []}
    with open(INFRA_FILE, "r", encoding="utf-8") as fh:
        docs = [d for d in yaml.safe_load_all(fh) if d]
    infra = {"schema": "guliangiam", "extensions": [], "functions": [], "triggers": [], "rls": {}, "roles": []}
    for doc in docs:
        for key in ("schema", "extensions", "functions", "triggers", "rls", "roles"):
            if key in doc:
                infra[key] = doc[key]
    return infra


# soft_delete / revision 是「结构能力标志」，其语义必须落到列：声明式 Schema 只写标志、
# 列由生成器补齐，避免同一语义在 30+ 张表里各写一遍（漏写即声明缺列，代码按
# updated_at/deleted_at/revision 读写时只会在运行期报错）。此处统一补齐，生成器与
# check-schema-parity 共用本加载器，因此声明与产物不会分叉。
IMPLICIT_COLUMNS: dict[str, dict] = {
    "created_at": {
        "name": "created_at",
        "type": "TIMESTAMPTZ",
        "nullable": False,
        "default": "now()",
        "comment": "Creation timestamp (schema convention: every table has created_at)",
    },
    "deleted_at": {
        "name": "deleted_at",
        "type": "TIMESTAMPTZ",
        "nullable": True,
        "comment": "Soft-delete timestamp (implied by soft_delete: true)",
    },
    "updated_at": {
        "name": "updated_at",
        "type": "TIMESTAMPTZ",
        "nullable": False,
        "default": "now()",
        "comment": "Last update timestamp (implied by revision / soft_delete)",
    },
    "revision": {
        "name": "revision",
        "type": "BIGINT",
        "nullable": False,
        "default": "1",
        "comment": "Optimistic lock version (implied by revision: true)",
    },
}


def _apply_implicit_columns(table: dict) -> None:
    columns = table.setdefault("columns", [])
    names = {column.get("name") for column in columns if isinstance(column, dict)}
    implied: list[str] = ["created_at"]
    if table.get("soft_delete"):
        implied.append("deleted_at")
    if table.get("revision") or table.get("soft_delete"):
        implied.append("updated_at")
    if table.get("revision"):
        implied.append("revision")
    for name in implied:
        if name in names:
            continue
        columns.append(dict(IMPLICIT_COLUMNS[name]))
        names.add(name)


def load_tables(specific: str | None = None) -> list[dict]:
    """按文件名序号 + 文档顺序加载表定义（拓扑顺序由 generate 阶段校验）。"""
    tables: list[dict] = []
    for fp in sorted(glob.glob(str(SCHEMA_DIR / "*.yaml"))):
        if specific and specific not in fp:
            continue
        with open(fp, "r", encoding="utf-8") as fh:
            for doc in yaml.safe_load_all(fh):
                if doc and "table_name" in doc:
                    doc["_source"] = Path(fp).name
                    _apply_implicit_columns(doc)
                    tables.append(doc)
    return tables


def short_name(table: str) -> str:
    return table.split(".")[-1]


def pk_columns(table: dict) -> list[str]:
    pk = table.get("pk") or []
    return list(pk) if isinstance(pk, list) else [pk]


# --------------------------------------------------------------------------- #
# DDL 解析（生成物侧，用于零漂移校验 / 旧 DDL 取证）                          #
# --------------------------------------------------------------------------- #
def _unquote_ident(s: str) -> str:
    """去除反引号/双引号包裹的标识符外层引号。"""
    s = s.strip()
    if len(s) >= 2 and s[0] in ("`", '"') and s[-1] == s[0]:
        return s[1:-1]
    return s


def parse_ddl(text: str) -> dict[str, dict]:
    """解析 DDL 文本，返回 {table: {"columns": {col: def}, "constraints": [def], "indexes": {...}}}。"""
    clean = strip_sql_comments(text)
    tables: dict[str, dict] = {}
    pos = 0
    # 表名支持反引号(MySQL)、双引号(PG)、无引号三种格式
    ident = r"(?:`[^`]+`|\"[^\"]+\"|[A-Za-z_][\w.]*)"
    while True:
        m = re.search(rf"CREATE TABLE IF NOT EXISTS\s+({ident})\s*\(", clean[pos:])
        if not m:
            break
        name = _unquote_ident(m.group(1).split(".")[-1])  # 取最后一段（去 schema 前缀）
        open_index = pos + m.end() - 1
        body = extract_balanced(clean, open_index)
        entry = {"columns": {}, "constraints": []}
        for segment in split_top_level(body):
            seg = normalize_space(segment)
            if not seg:
                continue
            upper = seg.upper()
            if upper.startswith(NON_COLUMN_PREFIXES):
                entry["constraints"].append(seg)
                continue
            # 列名支持反引号/双引号
            cm = re.match(rf"^({ident})\s+(.*)$", seg, re.S)
            if cm:
                entry["columns"][_unquote_ident(cm.group(1))] = cm.group(2)
        tables[name] = entry
        pos = open_index + len(body) + 2

    for m in re.finditer(
        rf"ALTER TABLE\s+({ident})\s+ADD\s+(?:COLUMN\s+)?(?:IF NOT EXISTS\s+)?([^;]+);",
        clean,
    ):
        table = _unquote_ident(m.group(1).split(".")[-1])
        rest = m.group(2)
        if " ADD CONSTRAINT " in f" {rest} " or rest.upper().lstrip().startswith("CONSTRAINT"):
            tables.setdefault(table, {"columns": {}, "constraints": []})["constraints"].append(
                normalize_space(rest)
            )
            continue
        cm = re.match(rf"^({ident})\s+(.*)$", normalize_space(rest), re.S)
        if cm:
            tables.setdefault(table, {"columns": {}, "constraints": []})[
                "columns"
            ][_unquote_ident(cm.group(1))] = cm.group(2)

    for m in re.finditer(
        rf"CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF NOT EXISTS\s+)?({ident})\s+ON\s+({ident})",
        clean,
    ):
        unique = bool(m.group(1))
        index_name = _unquote_ident(m.group(2))
        table = _unquote_ident(m.group(3).split(".")[-1])
        tables.setdefault(table, {"columns": {}, "constraints": []}).setdefault("indexes", {})[
            index_name
        ] = unique
    return tables


def index_names_of(table: dict) -> set[str]:
    """声明式中该表将生成的全部索引/唯一约束名（用于全局唯一性校验）。"""
    names = set()
    for c in table.get("constraints") or []:
        if c.get("type") in ("unique", "unique_partial"):
            names.add(c["name"])
    for idx in table.get("indexes") or []:
        names.add(idx["name"])
    return names

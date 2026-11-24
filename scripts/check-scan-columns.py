
#!/usr/bin/env python3
"""check-scan-columns.py — SQL SELECT 列数 vs Go Scan 目标数一致性门禁。

背景：repository 的 SELECT 列与 rows/row.Scan 目标数量不一致时 Go 编译器
不会发现，运行时才报 "expected N destination arguments in Scan, not M"。
V4.45 的 WebAuthn 凭据读取缺陷正是此类。

覆盖两种写法（同一函数作用域内匹配，避免把 mapper/helper 的 Scan 误配）：
  1. 链式：Query/QueryRow(...).Scan(...)
  2. 列表：rows, err := Query(...); ...; rows.Scan(...)
不覆盖把 rows 传给 helper（scanX(rows)）的形态——helper 的列契约由单元测试
与审查覆盖，避免静态误报。

范围：仅检查以 SELECT 开头的 SQL 常量（INSERT/UPDATE/WITH…RETURNING 的
RETURNING 列不在本门禁）。
"""

from __future__ import annotations

import glob
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
FUNC_SPLIT = re.compile(r"(?m)^func\s")


def parse_go_strings(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    pattern = re.compile(
        r"\b(\w+)\s*=\s*((?:`.*?`|\"(?:[^\"\\]|\\.)*\")"
        r"(?:\s*\+\s*(?:`.*?`|\"(?:[^\"\\]|\\.)*\"))*)",
        re.S,
    )
    for m in pattern.finditer(text):
        parts = re.findall(r"`(.*?)`|\"((?:[^\"\\]|\\.)*)\"", m.group(2), re.S)
        out[m.group(1)] = "".join(
            a if a is not None else b.replace("\\n", "\n") for a, b in parts
        )
    return out


def split_top(text: str) -> list[str]:
    parts: list[str] = []
    depth = 0
    cur = ""
    for ch in text:
        if ch in "([":
            depth += 1
        elif ch in ")]":
            depth = max(0, depth - 1)
        if ch == "," and depth == 0:
            parts.append(cur)
            cur = ""
        else:
            cur += ch
    if cur.strip():
        parts.append(cur)
    return parts


def select_columns(sql: str) -> int | None:
    stripped = sql.strip()
    if not stripped.upper().startswith("SELECT"):
        return None
    # Find the top-level FROM: EXTRACT(... FROM ...) / substring(...) contain
    # FROM inside parentheses and must not truncate the select list.
    depth = 0
    upper = stripped.upper()
    for i, ch in enumerate(stripped):
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth = max(0, depth - 1)
        elif depth == 0 and upper.startswith(" FROM", i) and (
            i + 5 >= len(stripped) or stripped[i + 5].isspace() or stripped[i + 5] in "\n("
        ):
            return len(split_top(stripped[6:i]))
    return None


QUERY_CALL = re.compile(
    r"(?:(\w+)\s*,\s*err\s*:?=\s*)?[A-Za-z_][\w.]*\.Query(?:Context)?\(\s*ctx,\s*(\w+)",
    re.S,
)
CHAINED_SCAN = re.compile(r"\.Scan\((.*?)\)", re.S)
ROWS_SCAN = re.compile(r"\b(\w+)\.Scan\((.*?)\)", re.S)


def check_file(sql_path: str) -> list[str]:
    repo_path = os.path.join(os.path.dirname(sql_path), "repository.go")
    if not os.path.exists(repo_path):
        return []
    queries = parse_go_strings(open(sql_path, encoding="utf-8").read())
    text = open(repo_path, encoding="utf-8").read()
    problems: list[str] = []
    starts = [m.start() for m in FUNC_SPLIT.finditer(text)]
    for idx, start in enumerate(starts):
        end = starts[idx + 1] if idx + 1 < len(starts) else len(text)
        block = text[start:end]
        for qm in QUERY_CALL.finditer(block):
            rowvar, qname = qm.group(1), qm.group(2)
            sql = queries.get(qname)
            if sql is None:
                continue
            expected = select_columns(sql)
            if expected is None or expected == 0:
                continue
            scan_args = None
            chained = CHAINED_SCAN.search(block, qm.end())
            # chained scan must immediately follow this call, not a later statement
            if chained and chained.start() - qm.end() < 4:
                scan_args = chained.group(1)
            elif rowvar:
                rs = ROWS_SCAN.search(block, qm.end())
                if rs and rs.group(1) == rowvar:
                    scan_args = rs.group(2)
            if scan_args is None:
                continue
            actual = len(split_top(scan_args))
            if expected != actual:
                line = text[: start + qm.start()].count("\n") + 1
                problems.append(
                    f"{repo_path}:{line}: {qname} SELECT {expected} 列，Scan {actual} 个目标"
                )
    return problems


def main() -> int:
    files = sorted(
        glob.glob(os.path.join(ROOT, "internal/infra/postgres/*/sql.go"))
        + glob.glob(os.path.join(ROOT, "internal/infra/mysql/*/sql.go"))
    )
    problems: list[str] = []
    for path in files:
        problems.extend(check_file(path))
    if problems:
        print("FAIL: SQL SELECT 列数与 Scan 目标数不一致：", file=sys.stderr)
        for p in problems:
            print("  " + p, file=sys.stderr)
        return 1
    print(f"PASS: {len(files)} 个仓储的 SELECT/Scan 列数一致")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

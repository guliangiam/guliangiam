#!/usr/bin/env python3
"""SQL location gate: only internal/infra/** may own SQL literals.

Scans internal/{app,domain,platform,transport} non-test .go files for string
literals (double-quoted and raw backtick) containing SQL statements. Comments
are ignored. This closes the gap that let `internal/domain/records/chain_sql.go`
hold six SQL constants while `check-layers.sh` stayed green (it only checks
imports).

Usage: python3 scripts/check-sql-location.py
Exit 1 on any hit; prints `path:line: snippet`.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SCAN_DIRS = [
    ROOT / "internal" / "app",
    ROOT / "internal" / "domain",
    ROOT / "internal" / "platform",
    ROOT / "internal" / "transport",
]

SQL_PATTERNS = [
    re.compile(r"\bSELECT\b[\s\S]*?\bFROM\b", re.IGNORECASE),
    re.compile(r"\bINSERT\s+INTO\b", re.IGNORECASE),
    re.compile(r"\bUPDATE\s+[\w.\"`]+\s+SET\b", re.IGNORECASE),
    re.compile(r"\bDELETE\s+FROM\b", re.IGNORECASE),
    re.compile(r"\bALTER\s+TABLE\b", re.IGNORECASE),
    re.compile(r"\bCREATE\s+TABLE\b", re.IGNORECASE),
]

# 首次运行命中的 platform 既有 SQL 站点（不属于本批 chain SQL 迁移）。
# 每一条都必须保留原因；新增 allowlist 必须在同一变更中说明为什么不能迁走。
ALLOWLIST = {
    "internal/platform/db/migrate.go": (
        "platform/db 是迁移账本与 schema 存在性原语的归属层；bootstrap 在建 infra "
        "之前就要用它，迁到 infra 会反转 bootstrap→infra 依赖。"
    ),
    "internal/platform/db/postgres.go": (
        "数据库存在性/租户定位原语，供 bootstrap 在 infra 构造前使用；迁到 infra "
        "会形成 bootstrap→infra→platform/db 的循环。"
    ),
    "internal/platform/crypto/provider_backfill.go": (
        "crypto 回填运维作业（provider 凭据列），SQL 列清单来自 crypto 注册表；"
        "待独立批次迁到 infra，当前不在 chain SQL 范围。"
    ),
    "internal/platform/crypto/reencrypt_job.go": (
        "crypto 密钥轮换/重加密运维作业；待独立批次迁到 infra，当前不在 chain SQL 范围。"
    ),
}


def string_literals(path: Path):
    """Yield (line_number, literal_text) for Go string literals, skipping comments."""
    text = path.read_text(encoding="utf-8", errors="replace")
    index = 0
    line = 1
    length = len(text)
    while index < length:
        char = text[index]
        nxt = text[index + 1] if index + 1 < length else ""

        if char == "\n":
            line += 1
            index += 1
            continue

        if char == "/" and nxt == "/":
            index += 2
            while index < length and text[index] != "\n":
                index += 1
            continue

        if char == "/" and nxt == "*":
            index += 2
            while index < length and not (text[index] == "*" and index + 1 < length and text[index + 1] == "/"):
                if text[index] == "\n":
                    line += 1
                index += 1
            index += 2
            continue

        if char == "`":
            start_line = line
            index += 1
            value = []
            while index < length and text[index] != "`":
                if text[index] == "\n":
                    line += 1
                value.append(text[index])
                index += 1
            index += 1
            yield start_line, "".join(value)
            continue

        if char == '"':
            start_line = line
            index += 1
            value = []
            while index < length:
                if text[index] == "\\":
                    index += 2
                    continue
                if text[index] == '"':
                    index += 1
                    break
                if text[index] == "\n":
                    line += 1
                value.append(text[index])
                index += 1
            yield start_line, "".join(value)
            continue

        index += 1


def main() -> int:
    hits = 0
    scanned = 0
    for root in SCAN_DIRS:
        if not root.exists():
            continue
        for path in sorted(root.rglob("*.go")):
            if path.name.endswith("_test.go"):
                continue
            scanned += 1
            relpath = str(path.relative_to(ROOT))
            file_hits = []
            for line, value in string_literals(path):
                for pattern in SQL_PATTERNS:
                    if pattern.search(value):
                        file_hits.append((line, " ".join(value.split())[:120]))
                        break
            if not file_hits:
                continue
            if relpath in ALLOWLIST:
                for line, snippet in file_hits:
                    print(f"ALLOWLIST {relpath}:{line}: {snippet} — {ALLOWLIST[relpath]}")
                continue
            for line, snippet in file_hits:
                print(f"{relpath}:{line}: {snippet}")
            hits += len(file_hits)

    if hits:
        print(f"FAIL: {hits} SQL literal(s) outside internal/infra ({scanned} files scanned)")
        return 1
    print(f"PASS: no SQL literals in app/domain/platform/transport ({scanned} files scanned)")
    return 0


if __name__ == "__main__":
    sys.exit(main())

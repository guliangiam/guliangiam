#!/usr/bin/env python3
"""AC 需求锚门禁：需求编号连续性 + 验收条件需求锚定。

两个失败面：
1. `docs/requirements/*.md` 的编号断档（除非被保留号声明覆盖）与重复定义；
2. `docs/acceptance-criteria.md` 的每个 AC family 必须映射到已存在需求前缀，
   且登记在 `REQUIREMENT_ANCHOR_REQUIRED` 的 family 每条 AC 必须引用已定义需求 ID。
"""
from __future__ import annotations

import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
REQ_DIR = os.path.join(REPO, "docs", "requirements")
AC_FILE = os.path.join(REPO, "docs", "acceptance-criteria.md")

REQ_ROW_RE = re.compile(r"^\|\s*([A-Z][A-Z0-9]*)-(\d{3})\s*\|")
SECTION_RANGE_RE = re.compile(r"\[REQ:([A-Z][A-Z0-9]*)-(\d{3})\s*~\s*\1-(\d{3})\]")
RESERVED_RE = re.compile(r"([A-Z][A-Z0-9]*)-(\d{3})\s*~\s*\1-(\d{3})[^\n]*保留号")
AC_RE = re.compile(r"^###\s+(AC-([A-Z0-9]+(?:-[A-Z0-9]+)*)-(\d{3}))\b")

# AC family -> 需求前缀族。新增 family 必须先在此登记，禁止静默放行。
AC_REQUIREMENT_DOMAIN: dict[str, str] = {
    "AIGW": "AIGW", "API": "API", "ARCH": "ARCH", "AUDITCHAIN": "AUDIT",
    "AUDIT-DENIAL": "SEC",
    "AUTH": "AUTH", "B2B": "B2B", "COMM": "COMM", "CRYPTO": "CRYPTO",
    "DATA": "DATA", "DR": "DR", "EVENT": "EVENT", "EXT": "EXT",
    "FE": "FE", "FEGOV": "FEGOV", "IDEM": "IDEM", "IGA": "IGA",
    "JWKS": "KEY", "KEY": "KEY", "METER": "METER", "MIGRATE": "MIGRATE",
    "NCIDAS": "AUTH", "OAUTH": "AUTH", "OUTBOX": "OUTBOX", "PAY": "PAY",
    "PERM": "PERM", "PRIV": "PRIV", "PROTO": "PROTO", "RATE": "RATE",
    "RATELIMIT": "RATE", "REBAC": "REBAC", "RELEASE": "RELEASE",
    "RISK": "RISK", "RLS": "RLS", "SEC": "SEC", "SESSION": "SESSION",
    "SLO": "SLO", "TENANT": "TENANT", "TOKEN": "AUTH", "WEBHOOK": "WEBHOOK",
    "WLI": "WLI", "USER": "USER", "APP": "APP", "PROV": "PROV",
}

# 这些 family 的每条 AC 必须显式引用一个已定义需求 ID。
REQUIREMENT_ANCHOR_REQUIRED = {"NCIDAS"}


def read_requirements() -> tuple[dict[str, set[int]], set[str], set[str], list[str]]:
    """返回：已定义编号、保留号、声明了编号空间的前缀、问题列表。

    连续性只在「显式声明了编号空间」的前缀内强制：`[REQ:PREFIX-a ~ PREFIX-b]`
    划定空间，空间内未列出的编号必须由本节的「保留号」声明覆盖，否则断档。
    未声明空间的前缀按 `docs/requirements/README.md`：未列出条目即保留号。
    """
    defined: dict[str, set[int]] = {}
    reserved: set[tuple[str, int]] = set()
    spaced_prefixes: set[str] = set()
    problems: list[str] = []
    for name in sorted(os.listdir(REQ_DIR)):
        if not name.endswith(".md"):
            continue
        path = os.path.join(REQ_DIR, name)
        rel = os.path.relpath(path, REPO)
        current_space: tuple[str, int, int] | None = None
        space_reserved = False

        def close_space() -> None:
            if current_space and space_reserved:
                prefix, lo, hi = current_space
                for number in range(lo, hi + 1):
                    reserved.add((prefix, number))

        for lineno, line in enumerate(open(path, encoding="utf-8").read().split("\n"), 1):
            m = REQ_ROW_RE.match(line)
            if m:
                prefix, number = m.group(1), int(m.group(2))
                if number in defined.setdefault(prefix, set()):
                    problems.append(f"REQ_DUPLICATE {prefix}-{number:03d} {rel}:{lineno}")
                defined[prefix].add(number)
                continue
            space = SECTION_RANGE_RE.search(line)
            if space:
                close_space()
                current_space = (space.group(1), int(space.group(2)), int(space.group(3)))
                spaced_prefixes.add(space.group(1))
                space_reserved = "保留号" in line
                continue
            if line.startswith("### "):
                close_space()
                current_space = None
                space_reserved = False
                continue
            if current_space and "保留号" in line:
                space_reserved = True
            r = RESERVED_RE.search(line)
            if r:
                for n in range(int(r.group(2)), int(r.group(3)) + 1):
                    reserved.add((r.group(1), n))
        close_space()
    return defined, reserved, spaced_prefixes, problems


def main() -> int:
    problems: list[str] = []
    defined, reserved, spaced_prefixes, req_problems = read_requirements()
    problems.extend(req_problems)

    # 编号空间内的缺口必须被「保留号」声明覆盖；未声明空间的前缀不在此列。
    for prefix in sorted(spaced_prefixes):
        numbers = sorted(defined.get(prefix, set()))
        for prev, cur in zip(numbers, numbers[1:]):
            for missing in range(prev + 1, cur):
                if (prefix, missing) not in reserved:
                    problems.append(
                        f"REQ_GAP {prefix}-{missing:03d} 位于 {prefix}-{prev:03d} 与 {prefix}-{cur:03d} 之间"
                    )

    defined_ids = {f"{p}-{n:03d}" for p, nums in defined.items() for n in nums}
    lines = open(AC_FILE, encoding="utf-8").read().split("\n")
    current: tuple[str, str, int] | None = None
    bodies: list[tuple[str, str, int, list[str]]] = []
    for lineno, line in enumerate(lines, 1):
        m = AC_RE.match(line)
        if m:
            if current:
                bodies.append((*current, body_lines))
            current = (m.group(1), m.group(2), lineno)
            body_lines: list[str] = []
            continue
        if current:
            body_lines.append(line)
    if current:
        bodies.append((*current, body_lines))

    for ac_id, family, lineno, body in bodies:
        if family not in AC_REQUIREMENT_DOMAIN:
            problems.append(f"AC_FAMILY_UNMAPPED {ac_id} docs/acceptance-criteria.md:{lineno}")
            continue
        if family in REQUIREMENT_ANCHOR_REQUIRED:
            found = [
                token
                for text in body
                for token in re.findall(r"[A-Z][A-Z0-9]*-[0-9]{3}", text)
                if token in defined_ids
            ]
            if not found:
                problems.append(f"AC_ANCHOR_ORPHAN {ac_id} docs/acceptance-criteria.md:{lineno}")

    print("=== check-doc-ac-anchors.py ===")
    print(f"需求定义前缀 {len(defined)} 个，AC {len(bodies)} 条。")
    for problem in problems:
        print(problem, file=sys.stderr)
    print(f"FAIL_COUNT={len(problems)}")
    if problems:
        return 1
    print("PASS: 需求编号连续且 AC 需求锚完整")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""check-descriptions-responsive.py — Descriptions 固定多列/固定跨列门禁

Why this exists
---------------
Ant Design v6 的 `Descriptions` 同时支持 `column` 与 `Item.span` 的断点对象。
只改 `column` 不改 `span` 时，窄视口降为单列后，`span={2}` 的条目会越过列数，
antd 报「Sum of column span in a line not match column」并出现错行；只改 `span`
不改 `column` 则窄屏仍被挤成多列。两者必须成对响应式，这个组合在 tsc、lint 与
组件测试里都看不出来，只在该视口渲染时才暴露。

判定规则（窄且可判定，不做意图判断）
  R1. 扫描 `web/src/pages/**/*.tsx`，跳过 `*.test.*` / `*.stories.*`。
  R2. 解析 `<Descriptions ...>` 开标签（跳过 `<Descriptions.Item>`）：
      `column={N}` 为数字且 N > 1 → FAIL。
  R3. 解析 `<Descriptions.Item ...>` 开标签：`span={N}` 为数字且 N > 1 → FAIL。
  R4. `column={1}`、`span={1}`、`span="filled"` 以及断点对象形式均允许。

Usage: python3 scripts/check-descriptions-responsive.py
Exit code 0 = PASS；1 = 存在固定多列 / 固定跨列。

消费方：scripts/verify-docs.sh（RUN_CODE_CHECKS=1）、CI。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
PAGES = REPO / "web" / "src" / "pages"

COLUMN_RE = re.compile(r"column=\{(\d+)\}")
SPAN_RE = re.compile(r"span=\{(\d+)\}")


def find_tag_end(text: str, start: int) -> int:
    """Return the index of the `>` that closes the opening tag, or -1."""
    depth = 0
    index = start
    while index < len(text):
        char = text[index]
        if char in "{(":
            depth += 1
        elif char in "})":
            depth -= 1
        elif char == ">" and depth == 0:
            return index
        index += 1
    return -1


def iter_opening_tags(text: str, literal: str):
    """Yield (line_number, tag_text) for each JSX opening tag with the literal."""
    index = 0
    while True:
        index = text.find(literal, index)
        if index == -1:
            return
        next_char_index = index + len(literal)
        if text[next_char_index : next_char_index + 1] == ".":
            index = next_char_index
            continue
        end = find_tag_end(text, next_char_index)
        if end == -1:
            return
        line_number = text.count("\n", 0, index) + 1
        yield line_number, text[index : end + 1]
        index = end + 1


def main() -> int:
    findings: list[str] = []
    scanned = 0

    for path in sorted(PAGES.rglob("*.tsx")):
        name = path.name
        if ".test." in name or ".stories." in name:
            continue
        text = path.read_text(encoding="utf-8")
        if "<Descriptions" not in text:
            continue
        scanned += 1
        relative = path.relative_to(REPO)

        for line_number, tag in iter_opening_tags(text, "<Descriptions"):
            match = COLUMN_RE.search(tag)
            if match and int(match.group(1)) > 1:
                findings.append(
                    f"  {relative}:{line_number}  column={{{match.group(1)}}} "
                    f"(use a breakpoint object such as {{ xs: 1, sm: 2 }})"
                )

        for line_number, tag in iter_opening_tags(text, "<Descriptions.Item"):
            match = SPAN_RE.search(tag)
            if match and int(match.group(1)) > 1:
                findings.append(
                    f"  {relative}:{line_number}  span={{{match.group(1)}}} on "
                    f"Descriptions.Item (pair it with the responsive column)"
                )

    if findings:
        print("FAIL: 存在固定多列 / 固定跨列，窄视口会错行")
        print("\n".join(findings))
        print(f"\n扫描 {scanned} 个含 Descriptions 的页面文件，命中 {len(findings)} 处。")
        print("修复：`column` 与 `Item span` 同步改为断点对象，例如 { xs: 1, sm: 2 }。")
        return 1

    print(f"PASS: Descriptions 响应式声明完整（扫描 {scanned} 个含 Descriptions 的页面文件）")
    return 0


if __name__ == "__main__":
    sys.exit(main())

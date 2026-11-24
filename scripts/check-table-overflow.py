#!/usr/bin/env python3
"""check-table-overflow.py — 详情页宽列未声明横向滚动的静态门禁

Why this exists
---------------
列表页统一走 `ResourceListCard`，它给 Ant Design Table 固定加了
`scroll={{ x: tableScrollX ?? 1600 }}`；详情页/tab 里直接渲染的 `<Table>` 没有这层
保护。列宽一旦显式声明，窄视口下表格会按列宽撑开，把页面容器顶出横向滚动条。
tsc、oxlint 与 mock 了 HTTP 的组件测试都看不见：问题只在布局阶段出现。

判定规则（窄且可判定，不做意图判断）
  R1. 扫描 `web/src/pages/**/*.tsx`，跳过 `*.test.*` / `*.stories.*`。
  R2. 对每个 `const columns... = [` 列定义块（按括号配对提取），累加块内数值
      `width:`。字符串宽度（如 "40%"）不计入，避免把比例列误判为固定宽度。
  R3. 任一块合计 >= MIN_WIDTH_SUM 时，只检查「该块之后最近的一个 `<Table`」是否
      声明了 `scroll=`。只推进到下一个 Table 而不是全文件搜索，才能在同一文件有
      多张表时指出具体是哪一张漏了（例如对账页只有明细表需要滚动，批次表不需要）。
  R4. 没有 `<Table` 的文件、列定义块合计都低于阈值的文件直接跳过。

阈值依据：768px 视口下内容区约 720px，加上固定列阴影与单元格内边距，900px
的列宽合计已经不能在容器内一次性展示（应当交给表格自身的横向滚动）。

Usage: python3 scripts/check-table-overflow.py
Exit code 0 = PASS；1 = 存在未声明滚动的高宽列定义块。

消费方：scripts/verify-docs.sh（RUN_CODE_CHECKS=1）、CI。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
PAGES = REPO / "web" / "src" / "pages"
MIN_WIDTH_SUM = 900

COLUMNS_RE = re.compile(r"const\s+columns(?:\s*:\s*[^=]+)?\s*=\s*\[")
WIDTH_RE = re.compile(r"width:\s*(\d{2,4})")
TABLE_RE = re.compile(r"<Table")
SCROLL_RE = re.compile(r"scroll=")


def iter_column_blocks(text: str) -> list[tuple[int, int, int, int]]:
    """Return (line_number, width_sum, width_count, end_index) per columns array."""
    blocks: list[tuple[int, int, int, int]] = []
    for match in COLUMNS_RE.finditer(text):
        start = match.start()
        open_index = text.find("[", match.end() - 1)
        if open_index == -1:
            continue
        depth = 0
        index = open_index
        while index < len(text):
            char = text[index]
            if char == "[":
                depth += 1
            elif char == "]":
                depth -= 1
                if depth == 0:
                    break
            index += 1
        block = text[open_index : index + 1]
        widths = [int(value) for value in WIDTH_RE.findall(block)]
        if not widths:
            continue
        line_number = text.count("\n", 0, start) + 1
        blocks.append((line_number, sum(widths), len(widths), index + 1))
    return blocks


def next_table_opening_tag(text: str, start: int) -> str | None:
    """Return the opening tag of the first `<Table` after start, or None.

    Handles the generic form `<Table<T>`: the type parameter's closing `>` is not
    the end of the opening tag, so it is skipped before looking for the props'
    terminator. Otherwise a table written as `<Table<Row> ... scroll={{ x }} />`
    would look like it ended before its props.
    """
    table_index = text.find("<Table", start)
    if table_index == -1:
        return None
    index = table_index + len("<Table")
    # Skip an optional generic type parameter: `<Table<Row>>`.
    probe = index
    while probe < len(text) and text[probe].isspace():
        probe += 1
    if probe < len(text) and text[probe] == "<":
        angle_depth = 0
        index = probe
        while index < len(text):
            char = text[index]
            if char == "<":
                angle_depth += 1
            elif char == ">":
                angle_depth -= 1
                if angle_depth == 0:
                    index += 1
                    break
            index += 1
    depth = 0
    while index < len(text):
        char = text[index]
        if char in "{(":
            depth += 1
        elif char in "})":
            depth -= 1
        elif char == ">" and depth == 0:
            return text[table_index : index + 1]
        index += 1
    return text[table_index:]


def main() -> int:
    findings: list[str] = []
    scanned = 0
    wide_blocks = 0

    for path in sorted(PAGES.rglob("*.tsx")):
        name = path.name
        if ".test." in name or ".stories." in name:
            continue
        text = path.read_text(encoding="utf-8")
        if not TABLE_RE.search(text):
            continue
        scanned += 1
        table_count = len(TABLE_RE.findall(text))
        for line_number, width_sum, width_count, end_index in iter_column_blocks(text):
            if width_sum < MIN_WIDTH_SUM:
                continue
            wide_blocks += 1
            opening_tag = next_table_opening_tag(text, end_index)
            if opening_tag is None:
                continue
            if SCROLL_RE.search(opening_tag):
                continue
            relative = path.relative_to(REPO)
            findings.append(
                f"  {relative}:{line_number}  width_sum={width_sum} "
                f"columns={width_count} next_table_has_scroll=false "
                f"(file tables={table_count})"
            )

    if findings:
        print("FAIL: 宽列定义未声明表格横向滚动（窄视口会把页面撑宽）")
        print("\n".join(findings))
        print(
            f"\n扫描 {scanned} 个含 Table 的页面文件，宽列块 {wide_blocks} 个，"
            f"未受保护 {len(findings)} 个。"
        )
        print("修复：给对应 <Table> 加 scroll={{ x: <列宽合计 + 选择列余量> }}。")
        return 1

    print(
        f"PASS: 页面表格横向滚动声明完整（扫描 {scanned} 个含 Table 的页面文件，"
        f"宽列块 {wide_blocks} 个均已有 scroll）"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())

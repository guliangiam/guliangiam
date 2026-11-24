#!/usr/bin/env python3
"""子菜单卡片化门禁：禁止同一标签页/分区出现双层卡片。

规则（只看 JSX 标签嵌套，不执行代码）：
  R1 `TabCard` 内不得再出现 `<Card>`；
  R2 `className` 含 `gl-section-anchor` 的元素内不得直接出现 `<Card>`；
  R3 带 `gl-tabs-card` 的 `<Tabs>` 内不得再出现 `<Card>`。

`FormSection` 不属于 `Card` 标签：它在卡片化作用域内由 CSS 扁平化为
「卡内分区 + 细横线」，因此允许作为唯一卡片内的功能单元。
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1] / "web" / "src"
TOKEN = re.compile(
    r"<([A-Za-z][\w.]*)((?:[^<>\"']|\"[^\"]*\"|'[^']*')*?)(/?)>|</([A-Za-z][\w.]*)>",
    re.S,
)
COMMENT = re.compile(r"(?<!:)//[^\n]*")


def violations_for(path: Path) -> list[tuple[int, str]]:
    text = COMMENT.sub("", path.read_text(encoding="utf-8"))
    stack: list[tuple[str, str]] = []
    violations: list[tuple[int, str]] = []

    for match in TOKEN.finditer(text):
        line = text.count("\n", 0, match.start()) + 1
        closing = match.group(4)
        if closing:
            for index in range(len(stack) - 1, -1, -1):
                if stack[index][0] == closing:
                    del stack[index:]
                    break
            continue

        name = match.group(1)
        attrs = match.group(2) or ""
        self_closing = match.group(3) == "/"

        if name == "Card":
            if any(entry[0] == "TabCard" for entry in stack):
                violations.append((line, "R1 TabCard 内出现嵌套 Card"))
            if any(
                entry[0] == "div" and "gl-section-anchor" in entry[1] for entry in stack
            ):
                violations.append((line, "R2 gl-section-anchor 内直接出现 Card"))
            if any(
                entry[0] == "Tabs" and "gl-tabs-card" in entry[1] for entry in stack
            ):
                violations.append((line, "R3 gl-tabs-card 的标签页内出现 Card"))

        if not self_closing:
            stack.append((name, attrs))

    return violations


def main() -> int:
    failures: list[str] = []
    checked = 0
    for path in sorted(ROOT.rglob("*.tsx")):
        if ".test." in path.name or ".stories." in path.name:
            continue
        checked += 1
        for line, rule in violations_for(path):
            failures.append(f"{path.relative_to(ROOT.parent.parent)}:{line}: {rule}")

    if failures:
        print("子菜单卡片化门禁失败：同一标签页/分区出现双层卡片")
        for item in failures:
            print("  " + item)
        return 1
    print(f"check-card-patterns: OK ({checked} tsx files)")
    return 0


if __name__ == "__main__":
    sys.exit(main())

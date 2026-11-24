#!/usr/bin/env python3
"""check-content-width.py — 控制台页面级横向宽度门禁

Why this exists
---------------
主显示区只允许三档语义宽度（`web/src/styles.css`）：

  · `.gl-content__inner`          列表 / 表格 / 仪表盘   1600px
  · `.gl-content__inner--form`    表单 / 详情            1200px
  · `.gl-content__inner--reading` 阅读型设置 / 确认页     960px

`docs/design/frontend/README.md` 明确禁止控制台页面再写页面级横向 `max-width`
或第二套容器。页面自己写 600/800px 时，同一壳层下的页面宽度会跳变，窄视口样式
也会分叉；tsc、oxlint 与组件测试都看不见。

判定规则
  R1. 扫描 `web/src/pages/**/*.tsx`（跳过 `*.test.*`）。
  R2. 跳过独立认证页（`pages/auth/**`）与 `gl-standalone-page` 文件：它们有独立的
      单面板宽度规范。
  R3. 跳过已含 `gl-content__inner--` 的文件：语义宽度页内部的卡片可以有自己的内容宽度。
  R4. 其余文件出现数值 `maxWidth >= 300` 或 `max-width: >=300px` 即违规。
      阈值排除文本省略宽度（如 `maxWidth: 180`）这类非页面宽度；占位符/示例 CSS
      字符串（同一行含 `placeholder`）不是布局声明，跳过。

Usage: python3 scripts/check-content-width.py
Exit code 0 = PASS；1 = 存在控制台页面级 max-width。

消费方：scripts/verify-docs.sh（RUN_CODE_CHECKS=1）、CI。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
PAGES = REPO / "web" / "src" / "pages"
MIN_PAGE_WIDTH = 300

MAX_WIDTH_JS_RE = re.compile(r"maxWidth\s*:\s*(\d{3,4})")
MAX_WIDTH_CSS_RE = re.compile(r"max-width\s*:\s*(\d{3,4})px")


def iter_pages() -> list[Path]:
    files = []
    for path in sorted(PAGES.rglob("*.tsx")):
        if ".test." in path.name:
            continue
        if path.relative_to(PAGES).parts[0] == "auth":
            continue
        files.append(path)
    return files


def main() -> int:
    violations = []
    print("== check-content-width.py ==")
    for path in iter_pages():
        text = path.read_text()
        if "gl-standalone-page" in text or "gl-content__inner--" in text:
            continue
        for regex in (MAX_WIDTH_JS_RE, MAX_WIDTH_CSS_RE):
            for match in regex.finditer(text):
                if int(match.group(1)) < MIN_PAGE_WIDTH:
                    continue
                line_start = text.rfind("\n", 0, match.start()) + 1
                line_end = text.find("\n", match.start())
                line_text = text[line_start : line_end if line_end != -1 else len(text)]
                if "placeholder" in line_text:
                    continue
                line = text.count("\n", 0, match.start()) + 1
                violations.append((path.relative_to(REPO), line, match.group(0)))

    if violations:
        for path, line, snippet in violations:
            print(f"FAIL: {path}:{line} {snippet} → 控制台页面禁止页面级 max-width，请改用语义宽度类")
        print(f"FAIL: {len(violations)} 处控制台页面级宽度")
        return 1

    print("PASS: 主显示区宽度门禁通过（控制台页面均使用三档语义宽度类）")
    return 0


if __name__ == "__main__":
    sys.exit(main())

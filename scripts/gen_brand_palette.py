#!/usr/bin/env python3
"""gen_brand_palette.py — 从 styles.css 生成 antd 主题镜像

唯一事实源：`web/src/styles.css` 的 `:root`（light）与 `[data-theme="dark"]`（dark）块
生成物：`web/src/app/theme/brand-palette.ts`

为什么需要镜像：Ant Design 计算主题时读不到 CSS 自定义属性，只能吃字面量。
因此把 antd 主题与对比度断言真正用到的 `--gl-color-*` 取值导出为 JS 常量。
`TOKEN_MAP` 只声明「antd 令牌键 ↔ CSS 令牌名」的对应关系，取值范围一律来自 CSS。
`DERIVED_TOKEN_MAP` 声明业务派生令牌（不注入 antd ThemeConfig），同样纳入镜像与零差异门禁。

用法：
  python3 scripts/gen_brand_palette.py           # 生成（写入磁盘）
  python3 scripts/gen_brand_palette.py --check   # 零差异门禁

输入：web/src/styles.css :root / [data-theme="dark"] 块
输出：web/src/app/theme/brand-palette.ts；--check 模式 diff 并输出 PASS/FAIL
--check 行为：--check 模式比较当前 brand-palette.ts 与生成结果的差异，无差异时 PASS
消费方：check-generated-zero-diff.sh、verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CSS_PATH = ROOT / "web/src/styles.css"
MIRROR_PATH = ROOT / "web/src/app/theme/brand-palette.ts"

LIGHT_SELECTOR = ":root {"
DARK_SELECTOR = '[data-theme="dark"] {'

# antd 主题令牌键 → `web/src/styles.css` 中的颜色令牌名。
# 只有对应关系是声明式的；取值全部从 CSS 读取，因此不存在「两处手写同一事实」。
TOKEN_MAP: tuple[tuple[str, str], ...] = (
    ("colorPrimary", "--gl-color-primary"),
    ("colorSuccess", "--gl-color-success"),
    ("colorWarning", "--gl-color-warning"),
    ("colorError", "--gl-color-error"),
    ("colorText", "--gl-color-text"),
    ("colorTextSecondary", "--gl-color-text-secondary"),
    ("colorTextTertiary", "--gl-color-text-tertiary"),
    ("colorBgContainer", "--gl-color-bg-elevated"),
    ("colorBorder", "--gl-color-border"),
)

# 业务派生令牌：由基础色合成，不进入 antd ThemeConfig，但消费方（如固定列样式）
# 需要与 CSS 同源的字面量，因此同样纳入生成物与零差异门禁。
DERIVED_TOKEN_MAP: tuple[tuple[str, str], ...] = (
    ("primaryBgSolid", "--gl-color-primary-bg-solid"),
    ("zebraBg", "--gl-color-zebra-bg"),
)

HEADER = """/**
 * 本文件由生成器产出，禁止手改。
 *
 * 事实源：`web/src/styles.css` 的 `:root`（light）与 `[data-theme="dark"]`（dark）块。
 * 复现命令：`python3 scripts/gen_brand_palette.py`
 * 零差异门禁：`python3 scripts/check-token-parity.py`
 *
 * 为什么需要这份镜像：Ant Design 计算主题时读不到 CSS 自定义属性，只能吃字面量；
 * 业务派生令牌（固定列/斑马纹等）同样需要与 CSS 同源的字面量，因此一并导出。
 * 改主题色请改 `styles.css` 后重新生成，不要直接编辑本文件。
 */
"""


def _oxfmt(text: str, rel_path: str) -> str:
    """把渲染出的 TS 交给 oxfmt，使生成物本身就是 `pnpm run format:check` 的结果。

    生成物若不自带格式，`pnpm run format` 与「重新生成」会互相推翻（web 的格式
    门禁覆盖仓库全部 TS，不区分手写与生成）。`--stdin-filepath` 让 oxfmt 只按该
    路径推断解析器，结果写到 stdout。
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


def css_block(text: str, selector: str) -> str:
    start = text.find(selector)
    if start < 0:
        raise SystemExit(f"FAIL: 在 web/src/styles.css 中找不到选择器 {selector!r}")
    end = text.find("\n}", start)
    if end < 0:
        raise SystemExit(f"FAIL: web/src/styles.css 的 {selector!r} 块未闭合")
    return text[start:end]


def css_tokens(block: str) -> dict[str, str]:
    return {
        name: re.sub(r"\s+", " ", value.strip()) for name, value in re.findall(
            r"(--[a-z0-9-]+)\s*:\s*([^;]+);", block
        )
    }


def render() -> str:
    css = CSS_PATH.read_text(encoding="utf-8")
    light = css_tokens(css_block(css, LIGHT_SELECTOR))
    dark = css_tokens(css_block(css, DARK_SELECTOR))

    problems = []
    for _, css_var in (*TOKEN_MAP, *DERIVED_TOKEN_MAP):
        for mode, tokens in (("light", light), ("dark", dark)):
            if css_var not in tokens:
                problems.append(f"{css_var} 未在 styles.css 的 {mode} 块中声明")
    if problems:
        raise SystemExit("FAIL: 主题令牌事实源不完整\n  - " + "\n  - ".join(problems))

    lines = [HEADER, 'export type PaletteMode = "light" | "dark";', ""]
    lines.append("/** antd 主题令牌键 → `web/src/styles.css` 中的颜色令牌名。 */")
    lines.append("export const THEME_TOKEN_CSS_VARS = {")
    for token_key, css_var in TOKEN_MAP:
        lines.append(f"  {token_key}: {json.dumps(css_var)},")
    lines.append("} as const;")
    lines.append("")
    lines.append("/** 与上述 CSS 令牌逐字一致的取值，由 `styles.css` 生成。 */")
    lines.append("export const BRAND_PALETTE = {")
    for mode, tokens in (("light", light), ("dark", dark)):
        lines.append(f"  {mode}: {{")
        for token_key, css_var in TOKEN_MAP:
            lines.append(f"    {token_key}: {json.dumps(tokens[css_var])},")
        lines.append("  },")
    lines.append("} as const;")
    lines.append("")
    lines.append("/** 业务派生颜色令牌（不注入 antd ThemeConfig）→ `styles.css` 中的颜色令牌名。 */")
    lines.append("export const DERIVED_TOKEN_CSS_VARS = {")
    for token_key, css_var in DERIVED_TOKEN_MAP:
        lines.append(f"  {token_key}: {json.dumps(css_var)},")
    lines.append("} as const;")
    lines.append("")
    lines.append("/** 与上述派生 CSS 令牌逐字一致的取值，由 `styles.css` 生成。 */")
    lines.append("export const DERIVED_COLOR_TOKENS = {")
    for mode, tokens in (("light", light), ("dark", dark)):
        lines.append(f"  {mode}: {{")
        for token_key, css_var in DERIVED_TOKEN_MAP:
            lines.append(f"    {token_key}: {json.dumps(tokens[css_var])},")
        lines.append("  },")
    lines.append("} as const;")

    return _oxfmt("\n".join(lines) + "\n", "web/src/app/theme/brand-palette.ts")


def check() -> int:
    expected = render()
    actual = MIRROR_PATH.read_text(encoding="utf-8") if MIRROR_PATH.exists() else ""
    if expected != actual:
        print("FAIL: web/src/app/theme/brand-palette.ts 与 styles.css 不一致（生成物陈旧或被手改）")
        print("  修复：python3 scripts/gen_brand_palette.py")
        return 1
    print(f"PASS: brand-palette.ts 与 styles.css 零差异（{len(TOKEN_MAP)} 个 antd 令牌 + {len(DERIVED_TOKEN_MAP)} 个派生令牌 × 2 模式）")
    return 0


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="只校验生成物是否与事实源零差异")
    args = parser.parse_args(argv)

    if args.check:
        return check()

    expected = render()
    MIRROR_PATH.parent.mkdir(parents=True, exist_ok=True)
    MIRROR_PATH.write_text(expected, encoding="utf-8")
    print(f"OK: 已写入 {MIRROR_PATH.relative_to(ROOT)}（{len(TOKEN_MAP)} 个 antd 令牌 + {len(DERIVED_TOKEN_MAP)} 个派生令牌 × 2 模式）")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

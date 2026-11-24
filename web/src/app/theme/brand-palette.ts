/**
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

export type PaletteMode = "light" | "dark";

/** antd 主题令牌键 → `web/src/styles.css` 中的颜色令牌名。 */
export const THEME_TOKEN_CSS_VARS = {
  colorPrimary: "--gl-color-primary",
  colorSuccess: "--gl-color-success",
  colorWarning: "--gl-color-warning",
  colorError: "--gl-color-error",
  colorText: "--gl-color-text",
  colorTextSecondary: "--gl-color-text-secondary",
  colorTextTertiary: "--gl-color-text-tertiary",
  colorBgContainer: "--gl-color-bg-elevated",
  colorBorder: "--gl-color-border",
} as const;

/** 与上述 CSS 令牌逐字一致的取值，由 `styles.css` 生成。 */
export const BRAND_PALETTE = {
  light: {
    colorPrimary: "#0b8073",
    colorSuccess: "#1f9d6c",
    colorWarning: "#c67a22",
    colorError: "#cf2e2e",
    colorText: "#102a2c",
    colorTextSecondary: "rgba(16, 42, 44, 0.66)",
    colorTextTertiary: "#5f7076",
    colorBgContainer: "#ffffff",
    colorBorder: "rgba(16, 42, 44, 0.08)",
  },
  dark: {
    colorPrimary: "#2ec4b6",
    colorSuccess: "#3ddc84",
    colorWarning: "#f0a030",
    colorError: "#f06060",
    colorText: "#e8e8e8",
    colorTextSecondary: "rgba(232, 232, 232, 0.65)",
    colorTextTertiary: "#949494",
    colorBgContainer: "#1f1f1f",
    colorBorder: "rgba(255, 255, 255, 0.08)",
  },
} as const;

/** 业务派生颜色令牌（不注入 antd ThemeConfig）→ `styles.css` 中的颜色令牌名。 */
export const DERIVED_TOKEN_CSS_VARS = {
  primaryBgSolid: "--gl-color-primary-bg-solid",
  zebraBg: "--gl-color-zebra-bg",
} as const;

/** 与上述派生 CSS 令牌逐字一致的取值，由 `styles.css` 生成。 */
export const DERIVED_COLOR_TOKENS = {
  light: {
    primaryBgSolid: "color-mix( in srgb, var(--gl-color-primary) 8%, var(--gl-color-bg-elevated) )",
    zebraBg: "color-mix(in srgb, #000 1%, var(--gl-color-bg-elevated))",
  },
  dark: {
    primaryBgSolid:
      "color-mix( in srgb, var(--gl-color-primary) 12%, var(--gl-color-bg-elevated) )",
    zebraBg: "color-mix(in srgb, #fff 3%, var(--gl-color-bg-elevated))",
  },
} as const;

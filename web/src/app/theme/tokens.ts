/**
 * Design Tokens — spacing / typography / elevation
 *
 * 这些 token 会通过 ConfigProvider.token 注入 Ant Design 主题，
 * 同时作为 CSS 自定义属性暴露给 styles.css。
 *
 * 颜色 token 已在 brand-palette.ts，这里补充其他三类。
 */

// ─── Spacing ───────────────────────────────────────────────────────────────────
// 基于 4px 基准网格
export const SPACING = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 24,
  xxl: 32,
  xxxl: 48,
} as const;

// ─── Typography ────────────────────────────────────────────────────────────────
export const TYPOGRAPHY = {
  fontFamily:
    "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif",
  fontSize: 14,
  fontSizeSM: 12,
  fontSizeLG: 16,
  fontSizeXL: 20,
  fontSizeHeading1: 38,
  fontSizeHeading2: 30,
  fontSizeHeading3: 24,
  fontSizeHeading4: 20,
  fontSizeHeading5: 16,
  lineHeight: 1.5715,
  lineHeightSM: 1.6667,
  lineHeightLG: 1.5,
  fontWeight: 400,
  fontWeightStrong: 600,
} as const;

// ─── Elevation (box-shadow) ─────────────────────────────────────────────────────
export const ELEVATION = {
  none: "none",
  xs: "0 1px 2px rgba(16, 42, 44, 0.06)",
  sm: "0 2px 4px rgba(16, 42, 44, 0.08)",
  md: "0 4px 12px rgba(16, 42, 44, 0.10)",
  lg: "0 8px 24px rgba(16, 42, 44, 0.12)",
  xl: "0 16px 48px rgba(16, 42, 44, 0.16)",
} as const;

// 暗模式 elevation 稍强（更暗的背景需要更大对比度）
export const ELEVATION_DARK = {
  none: "none",
  xs: "0 1px 2px rgba(0, 0, 0, 0.30)",
  sm: "0 2px 4px rgba(0, 0, 0, 0.35)",
  md: "0 4px 12px rgba(0, 0, 0, 0.40)",
  lg: "0 8px 24px rgba(0, 0, 0, 0.45)",
  xl: "0 16px 48px rgba(0, 0, 0, 0.50)",
} as const;

// ─── 汇总为 antd token 格式 ────────────────────────────────────────────────────
export const ANTD_TOKEN_EXTRA = {
  // spacing
  marginXS: SPACING.xs,
  marginSM: SPACING.sm,
  marginMD: SPACING.md,
  marginLG: SPACING.lg,
  marginXL: SPACING.xl,
  marginXXL: SPACING.xxl,
  paddingXS: SPACING.xs,
  paddingSM: SPACING.sm,
  paddingMD: SPACING.md,
  paddingLG: SPACING.lg,
  paddingXL: SPACING.xl,
  paddingXXL: SPACING.xxl,

  // typography
  ...TYPOGRAPHY,
} as const;

export type GuliangDesignTokens = {
  spacing: typeof SPACING;
  typography: typeof TYPOGRAPHY;
  elevation: typeof ELEVATION;
  elevationDark: typeof ELEVATION_DARK;
};

export const DESIGN_TOKENS: GuliangDesignTokens = {
  spacing: SPACING,
  typography: TYPOGRAPHY,
  elevation: ELEVATION,
  elevationDark: ELEVATION_DARK,
};

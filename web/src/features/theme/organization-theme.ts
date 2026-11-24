/**
 * 组织主题：token 白名单与 CSS 注入。
 *
 * 组织设置页允许管理员写 Theme Tokens JSON 与 Custom CSS。为避免任意 token
 * 破坏整站可读性，只接受一组白名单键，非法值丢弃并记录 warning，由设置页提示。
 */

export const ORGANIZATION_THEME_TOKEN_KEYS = [
  "colorPrimary",
  "colorSuccess",
  "colorWarning",
  "colorError",
  "colorInfo",
  "borderRadius",
  "fontSize",
  "colorBgLayout",
  "colorBgContainer",
  "colorText",
  "colorBorder",
] as const;

export type OrganizationThemeTokenKey = (typeof ORGANIZATION_THEME_TOKEN_KEYS)[number];

export type SanitizedOrganizationTheme = {
  tokens: Partial<Record<OrganizationThemeTokenKey, string | number>>;
  warnings: string[];
};

function isSafeCssColor(value: string): boolean {
  return /^(#[0-9a-fA-F]{3,8}|rgba?\([^)]*\)|hsla?\([^)]*\)|[a-zA-Z]+)$/.test(value.trim());
}

// 数值键的安全区间：越界/非有限值会让 antd 派生 token 塌缩（fontSize<=0、
// 负 borderRadius）或撑破布局；string 传给数值键会被 tokenNumber 静默忽略，
// 造成设置页假成功，故一律拒绝并提示。
const NUMERIC_TOKEN_BOUNDS: Partial<
  Record<OrganizationThemeTokenKey, { min: number; max: number }>
> = {
  borderRadius: { min: 0, max: 100 },
  fontSize: { min: 8, max: 72 },
};

export function sanitizeOrganizationTokens(raw: unknown): SanitizedOrganizationTheme {
  const warnings: string[] = [];
  const tokens: SanitizedOrganizationTheme["tokens"] = {};
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return { tokens, warnings };
  }

  for (const key of ORGANIZATION_THEME_TOKEN_KEYS) {
    const value = (raw as Record<string, unknown>)[key];
    if (value === undefined || value === null) continue;

    const bounds = NUMERIC_TOKEN_BOUNDS[key];
    if (bounds) {
      if (
        typeof value !== "number" ||
        !Number.isFinite(value) ||
        value < bounds.min ||
        value > bounds.max
      ) {
        warnings.push(`${key} must be a number between ${bounds.min} and ${bounds.max}`);
        continue;
      }
      tokens[key] = value;
      continue;
    }

    // 白名单其余键均为颜色：只接受合法 CSS 颜色字符串，数字/空串一律拒绝。
    if (typeof value !== "string" || value.trim() === "" || !isSafeCssColor(value)) {
      warnings.push(`${key} is not a valid CSS color`);
      continue;
    }
    tokens[key] = value;
  }

  return { tokens, warnings };
}

/** 去掉可执行/越界 CSS 片段；保留普通样式声明。 */
export function sanitizeOrganizationCss(css: string): string {
  return css
    .replace(/<\/?style[^>]*>/gi, "")
    .replace(/@import[^;]+;/gi, "")
    .replace(/expression\s*\(/gi, "")
    .replace(/javascript\s*:/gi, "");
}

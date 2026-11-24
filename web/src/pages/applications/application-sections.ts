/** 应用详情页的业务分区定义与 `?tab=` 兼容解析。 */

export type AppSectionKey =
  | "basics"
  | "authentication"
  | "oidc"
  | "saml"
  | "providers"
  | "ui"
  | "security"
  | "reverseProxy";

export const APP_SECTION_KEYS: AppSectionKey[] = [
  "basics",
  "authentication",
  "oidc",
  "saml",
  "providers",
  "ui",
  "security",
  "reverseProxy",
];

/**
 * Legacy `?tab=` keys stay valid so bookmarks and deep links keep working.
 * 旧的分组/页签键折叠到新的业务分区；规范化 effect 再把分区键写回 URL。
 */
export const LEGACY_TAB_TO_SECTION: Record<string, AppSectionKey> = {
  overview: "basics",
  basic: "basics",
  basics: "basics",
  auth: "authentication",
  authentication: "authentication",
  "auth-protocol": "authentication",
  oidc: "oidc",
  "oidc-oauth": "oidc",
  "custom-scopes": "oidc",
  scopes: "oidc",
  saml: "saml",
  providers: "providers",
  integration: "providers",
  ui: "ui",
  "ui-customization": "ui",
  uiCustomization: "ui",
  experience: "ui",
  security: "security",
  "signin-log": "security",
  logs: "security",
  operations: "security",
  "reverse-proxy": "reverseProxy",
  reverseProxy: "reverseProxy",
};

/** URL `?tab=` → 可渲染的分区键；未知值返回 null（由调用方回落到默认分区）。 */
export function resolveAppSection(raw: string | null | undefined): AppSectionKey | null {
  const value = (raw ?? "").trim();
  if (!value) return null;
  if (APP_SECTION_KEYS.includes(value as AppSectionKey)) return value as AppSectionKey;
  return LEGACY_TAB_TO_SECTION[value] ?? LEGACY_TAB_TO_SECTION[value.toLowerCase()] ?? null;
}

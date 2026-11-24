/**
 * 应用对外访问入口 URL。
 *
 * 默认登录/注册页的规则由前端路由决定（见 `app/router/auth-routes.tsx`）：
 *   /login/{organizationSlug}?application={applicationSlug}
 *   /signup/{organizationSlug}?application={applicationSlug}
 *
 * 应用上配置了自定义地址（`signinUrl` / `signupUrl`）时，自定义地址优先。
 */

export type AccessUrlContext = {
  origin: string;
  organizationSlug: string;
  applicationSlug: string;
};

function trimmed(value: string | undefined): string {
  return (value ?? "").trim();
}

export function defaultSigninUrl({ origin, organizationSlug, applicationSlug }: AccessUrlContext) {
  const base = `${origin}/login/${encodeURIComponent(organizationSlug)}`;
  return applicationSlug ? `${base}?application=${encodeURIComponent(applicationSlug)}` : base;
}

export function defaultSignupUrl({ origin, organizationSlug, applicationSlug }: AccessUrlContext) {
  const base = `${origin}/signup/${encodeURIComponent(organizationSlug)}`;
  return applicationSlug ? `${base}?application=${encodeURIComponent(applicationSlug)}` : base;
}

/** 自定义地址优先；为空时回退到默认地址。 */
export function effectiveAccessUrl(custom: string | undefined, fallback: string): string {
  return trimmed(custom) || fallback;
}

export function isCustomAccessUrl(custom: string | undefined): boolean {
  return trimmed(custom) !== "";
}

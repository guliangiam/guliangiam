import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import {
  AUTH_ACCOUNT,
  AUTH_AUTHORIZE,
  AUTH_CAPTCHA,
  AUTH_CAPTCHA_STATUS,
  AUTH_CAPTCHA_VERIFY,
  AUTH_CAS_LOGOUT,
  AUTH_LOGIN,
  AUTH_LOGIN_CHALLENGE,
  AUTH_LOGIN_CHALLENGE_APPROVE,
  AUTH_LOGIN_CHALLENGE_REJECT,
  AUTH_LOGIN_LDAP,
  AUTH_LOGOUT,
  AUTH_PASSWORD_CHANGE,
  AUTH_SOCIAL_AUTHORIZE,
  AUTH_SOCIAL_LOGIN,
  AUTH_SSO_LOGOUT,
  AUTH_TOKENS_EXCHANGE,
  AUTH_VERIFY_CODE,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

/**
 * 组织密码策略的生效快照。后端账户上下文直接下发，前端表单与提示文案
 * 必须以此为准，禁止再出现本地硬编码的长度门槛。
 */
export type PasswordPolicy = {
  minLength: number;
  maxLength: number;
  requireUppercase: boolean;
  requireLowercase: boolean;
  requireDigit: boolean;
  requireSpecial: boolean;
  rejectCommonPasswords: boolean;
  rejectUsername: boolean;
  passwordHistoryCount: number;
  passwordMaxAgeDays: number;
};

/** 仅在账户上下文缺失策略时兜底；与后端 domain 默认值保持一致。 */
export const DEFAULT_PASSWORD_POLICY: PasswordPolicy = {
  minLength: 14,
  maxLength: 128,
  requireUppercase: true,
  requireLowercase: true,
  requireDigit: true,
  requireSpecial: true,
  rejectCommonPasswords: true,
  rejectUsername: true,
  passwordHistoryCount: 5,
  passwordMaxAgeDays: 90,
};

export type SessionPolicy = {
  idleTimeoutMinutes: number;
};

export const DEFAULT_SESSION_POLICY: SessionPolicy = {
  idleTimeoutMinutes: 30,
};

export type AuthAccountContext = {
  account: SessionAccount;
  organization: SessionOrganization;
  accessToken?: string;
  passwordPolicy: PasswordPolicy;
  /** 与后端空闲过期判定同源；缺省时前端回落到 30 分钟。 */
  sessionPolicy: SessionPolicy;
  /** 来自 user_credentials.need_update_password；当前为提示态，不做后端硬拦截。 */
  needUpdatePassword: boolean;
};

export type SessionAccount = {
  id?: string;
  owner: string;
  name: string;
  displayName: string | null;
  avatar: string | null;
  isAdmin: boolean;
};

export type SessionOrganization = {
  id: string;
  name: string;
  displayName: string | null;
  logoUrl: string | null;
  faviconUrl: string | null;
  navItems: string[];
  userNavItems: string[];
  /** navPreferences.navGroupOrder：菜单分组顺序（组 key 数组）；未配置时为空数组。 */
  navGroupOrder: string[];
};

export type LoginMfaMethod = {
  id: string;
  methodType: string;
  displayName?: string;
  target?: string;
  countryCode?: string;
};

export type LoginSuccess = AuthAccountContext & {
  status: "success";
};

export type LoginMfaRequired = {
  status: "mfa_required";
  nextMfa?: string;
  challengeId: string;
  challengeState?: string;
  challengeExpiresAt?: string;
  requiredMfa: string[];
  availableMfaMethods: LoginMfaMethod[];
};

export type LoginChallenge = {
  id: string;
  state: string;
  methodType?: string;
  expiresAt: string;
  verifiedAt?: string;
};

export type LoginMfaSetupRequired = {
  status: "required_mfa_setup";
  challengeId: string;
  challengeExpiresAt?: string;
  requiredMfa: string[];
};

export type LoginResult = LoginSuccess | LoginMfaRequired | LoginMfaSetupRequired;

export type AuthorizePayload = {
  clientId: string;
  redirectUri: string;
  responseType?: "code";
  requestedScopes?: string[];
  state?: string;
  nonce?: string;
  codeChallenge?: string;
  codeChallengeMethod?: "S256";
  resource?: string;
};

export type AuthorizeSuccess = {
  status: "authorized";
  code: {
    code: string;
    redirectUri: string;
    redirectTo: string;
    state: string;
    expiresAt?: string;
  };
};

export type AuthorizeConsentRequired = {
  status: "consent_required";
  evaluation: {
    applicationId: string;
    applicationSlug: string;
    requestedScopes: string[];
    consentableScopes: string[];
    grantedScopes: string[];
    missingScopes: string[];
    required: boolean;
  };
};

export type AuthorizeResult = AuthorizeSuccess | AuthorizeConsentRequired;

export type TokenExchangePayload = {
  grantType?: "authorization_code";
  clientId: string;
  clientSecret?: string;
  code: string;
  redirectUri: string;
  codeVerifier?: string;
  resource?: string;
};

export type TokenExchangeResult = {
  status: "issued";
  accessToken: string;
  refreshToken?: string;
  idToken?: string;
  tokenType: "Bearer";
  expiresIn: number;
  scope?: string;
};

type LoginVerificationPayload = {
  mfaType?: string;
  passcode?: string;
  recoveryCode?: string;
  enableMfaRemember?: boolean;
  /** 一次性码登录（验证码 tab）：与 password 互斥，通道值必须是 "email"/"sms"。 */
  verificationCode?: string;
  verificationType?: "email" | "sms";
  /** 组织启用 captcha provider 时必须上送，后端 EnforceCaptcha 强制校验。 */
  captchaToken?: string;
  captchaId?: string;
  clientId?: string;
  redirectUri?: string;
  scope?: string;
  state?: string;
  nonce?: string;
  codeChallengeMethod?: string;
  codeChallenge?: string;
};

export type LoginPayload =
  | ({
      organizationSlug: string;
      username: string;
      password: string;
      applicationId: string;
      challengeId?: never;
    } & LoginVerificationPayload)
  | ({
      challengeId: string;
      organizationSlug?: string;
      username?: string;
      password?: string;
      applicationId?: string;
    } & LoginVerificationPayload);

export type ChallengeLoginPayload = {
  challengeId: string;
  mfaType?: string;
  passcode?: string;
  recoveryCode?: string;
  enableMfaRemember?: boolean;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const authSessionAccountSchema = z.object({
  id: z.string().optional(),
  owner: z.string(),
  name: z.string(),
  displayName: z.string().optional(),
  avatar: z.string().optional(),
  isAdmin: z.boolean(),
});

const authSessionOrganizationSchema = z.object({
  id: z.string(),
  name: z.string(),
  displayName: z.string().optional(),
  logoUrl: z.string().optional().nullable(),
  faviconUrl: z.string().optional().nullable(),
  navItems: z
    .array(z.string())
    .nullable()
    .transform((value) => value ?? []),
  userNavItems: z
    .array(z.string())
    .nullable()
    .transform((value) => value ?? []),
  navGroupOrder: z
    .array(z.string())
    .optional()
    .nullable()
    .transform((value) => value ?? []),
});

const passwordPolicySchema = z.object({
  minLength: z.number().int().default(DEFAULT_PASSWORD_POLICY.minLength),
  maxLength: z.number().int().default(DEFAULT_PASSWORD_POLICY.maxLength),
  requireUppercase: z.boolean().default(DEFAULT_PASSWORD_POLICY.requireUppercase),
  requireLowercase: z.boolean().default(DEFAULT_PASSWORD_POLICY.requireLowercase),
  requireDigit: z.boolean().default(DEFAULT_PASSWORD_POLICY.requireDigit),
  requireSpecial: z.boolean().default(DEFAULT_PASSWORD_POLICY.requireSpecial),
  rejectCommonPasswords: z.boolean().default(DEFAULT_PASSWORD_POLICY.rejectCommonPasswords),
  rejectUsername: z.boolean().default(DEFAULT_PASSWORD_POLICY.rejectUsername),
  passwordHistoryCount: z.number().int().default(DEFAULT_PASSWORD_POLICY.passwordHistoryCount),
  passwordMaxAgeDays: z.number().int().default(DEFAULT_PASSWORD_POLICY.passwordMaxAgeDays),
});

const sessionPolicySchema = z.object({
  idleTimeoutMinutes: z
    .number()
    .int()
    .positive()
    .default(DEFAULT_SESSION_POLICY.idleTimeoutMinutes),
});

const authAccountContextSchema = z.object({
  account: authSessionAccountSchema,
  organization: authSessionOrganizationSchema,
  accessToken: z.string().optional(),
  passwordPolicy: passwordPolicySchema.optional(),
  sessionPolicy: sessionPolicySchema.optional(),
  needUpdatePassword: z.boolean().catch(false),
});

const loginMfaMethodSchema = z.object({
  id: z.string(),
  methodType: z.string(),
  displayName: z.string().optional(),
  target: z.string().optional(),
  countryCode: z.string().optional(),
});

const loginSuccessSchema = authAccountContextSchema.extend({
  status: z.literal("success"),
});

const loginMfaRequiredSchema = z.object({
  status: z.literal("mfa_required"),
  nextMfa: z.string().optional(),
  challengeId: z.string(),
  challengeState: z.string().optional(),
  challengeExpiresAt: z.string().optional(),
  requiredMfa: z.array(z.string()).catch([]),
  availableMfaMethods: z.array(loginMfaMethodSchema).catch([]),
});

const loginChallengeSchema = z.object({
  id: z.string(),
  state: z.string(),
  methodType: z.string().optional(),
  expiresAt: z.string(),
  verifiedAt: z.string().optional(),
});

const loginMfaSetupRequiredSchema = z.object({
  status: z.literal("required_mfa_setup"),
  challengeId: z.string(),
  challengeExpiresAt: z.string().optional(),
  requiredMfa: z.array(z.string()).catch([]),
});

const loginResultSchema = z.discriminatedUnion("status", [
  loginSuccessSchema,
  loginMfaRequiredSchema,
  loginMfaSetupRequiredSchema,
]);

const authorizeSuccessSchema = z.object({
  status: z.literal("authorized"),
  code: z.object({
    code: z.string(),
    redirectUri: z.string(),
    redirectTo: z.string(),
    state: z.string(),
    expiresAt: z.string().optional(),
  }),
});

const authorizeConsentRequiredSchema = z.object({
  status: z.literal("consent_required"),
  evaluation: z.object({
    applicationId: z.string(),
    applicationSlug: z.string(),
    requestedScopes: z.array(z.string()).catch([]),
    consentableScopes: z.array(z.string()).catch([]),
    grantedScopes: z.array(z.string()).catch([]),
    missingScopes: z.array(z.string()).catch([]),
    required: z.boolean(),
  }),
});

const authorizeResultSchema = z.discriminatedUnion("status", [
  authorizeSuccessSchema,
  authorizeConsentRequiredSchema,
]);

const tokenExchangeResultSchema = z.object({
  status: z.literal("issued"),
  accessToken: z.string(),
  refreshToken: z.string().optional(),
  // Present only when the granted scopes contain `openid` (OIDC Core §3.1.3.3).
  idToken: z.string().optional(),
  tokenType: z.literal("Bearer"),
  expiresIn: z.number(),
  scope: z.string().optional(),
});

const logoutResponseSchema = z.object({
  status: z.literal("logged_out"),
});

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

function toAuthAccountContext(
  payload: z.infer<typeof authAccountContextSchema>,
): AuthAccountContext {
  return {
    account: {
      id: payload.account.id,
      owner: payload.account.owner,
      name: payload.account.name,
      displayName: payload.account.displayName ?? null,
      avatar: payload.account.avatar ?? null,
      isAdmin: payload.account.isAdmin,
    },
    organization: {
      id: payload.organization.id,
      name: payload.organization.name,
      displayName: payload.organization.displayName ?? null,
      logoUrl: payload.organization.logoUrl ?? null,
      faviconUrl: payload.organization.faviconUrl ?? null,
      navItems: payload.organization.navItems,
      userNavItems: payload.organization.userNavItems,
      navGroupOrder: payload.organization.navGroupOrder,
    },
    accessToken: payload.accessToken,
    passwordPolicy: payload.passwordPolicy ?? DEFAULT_PASSWORD_POLICY,
    sessionPolicy: payload.sessionPolicy ?? DEFAULT_SESSION_POLICY,
    needUpdatePassword: payload.needUpdatePassword ?? false,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getAuthAccountContext(): Promise<AuthAccountContext> {
  const response = await requestDocumentJson(AUTH_ACCOUNT);
  return toAuthAccountContext(authAccountContextSchema.parse(response));
}

/**
 * PATCH /api/v1/auth/account —— 会话主体自助编辑资料/绑定联系方式。
 *
 * 身份来自会话（服务端不接受请求体里的组织/用户名），因此扫码自动注册的 guest 账号
 * 也能在这里认领自己：改显示名不需要验证码，绑定 email/phone 必须带发给该联系方式
 * 的验证码（先走 /api/v1/auth/verification-code/send）。响应与 GET 同形，可就地刷新。
 */
export async function updateOwnAccount(values: {
  displayName?: string;
  avatarUrl?: string;
  email?: string;
  phone?: string;
  verificationCode?: string;
}): Promise<AuthAccountContext> {
  const response = await requestDocumentJson(AUTH_ACCOUNT, {
    method: "PATCH",
    body: JSON.stringify(values),
  });
  return toAuthAccountContext(authAccountContextSchema.parse(response));
}

export async function loginWithPassword(payload: LoginPayload): Promise<LoginResult> {
  const response = await requestDocumentJson(AUTH_LOGIN, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });

  const parsed = loginResultSchema.parse(response);
  if (parsed.status === "success") {
    return {
      status: "success",
      ...toAuthAccountContext(parsed),
    };
  }

  if (parsed.status === "required_mfa_setup") {
    return {
      status: "required_mfa_setup",
      challengeId: parsed.challengeId,
      challengeExpiresAt: parsed.challengeExpiresAt,
      requiredMfa: parsed.requiredMfa,
    };
  }

  return {
    status: "mfa_required",
    nextMfa: parsed.nextMfa,
    challengeId: parsed.challengeId,
    challengeState: parsed.challengeState,
    challengeExpiresAt: parsed.challengeExpiresAt,
    requiredMfa: parsed.requiredMfa,
    availableMfaMethods: parsed.availableMfaMethods,
  };
}

export async function authorizeWithSession(payload: AuthorizePayload): Promise<AuthorizeResult> {
  const response = await requestDocumentJson(AUTH_AUTHORIZE, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });

  return authorizeResultSchema.parse(response);
}

export async function exchangeAuthorizationCode(
  payload: TokenExchangePayload,
): Promise<TokenExchangeResult> {
  const response = await requestDocumentJson(AUTH_TOKENS_EXCHANGE, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });

  return tokenExchangeResultSchema.parse(response);
}

export async function getLoginChallenge(challengeId: string): Promise<LoginChallenge> {
  const response = await requestDocumentJson(AUTH_LOGIN_CHALLENGE(challengeId));
  return loginChallengeSchema.parse(response);
}

export async function approveLoginChallenge(challengeId: string): Promise<LoginChallenge> {
  const response = await requestDocumentJson(AUTH_LOGIN_CHALLENGE_APPROVE(challengeId), {
    method: "POST",
  });
  return loginChallengeSchema.parse(response);
}

export async function rejectLoginChallenge(challengeId: string): Promise<LoginChallenge> {
  const response = await requestDocumentJson(AUTH_LOGIN_CHALLENGE_REJECT(challengeId), {
    method: "POST",
  });
  return loginChallengeSchema.parse(response);
}

// ─────────────────────────────────────────────
// Social Login & LDAP
// ─────────────────────────────────────────────

export type SocialLoginParams = {
  providerId: string;
  code: string;
  state?: string;
  codeVerifier?: string;
  organizationSlug?: string;
  applicationId: string;
  responseType?: string;
  redirectUri?: string;
  scope?: string[];
};

export type SocialAuthorizeParams = {
  providerId: string;
  organizationSlug?: string;
  applicationId?: string;
  state?: string;
  scope?: string[];
};

export type SocialAuthorizeResult = {
  redirectUrl: string;
};

const socialAuthorizeResultSchema = z.object({
  redirectUrl: z.string(),
});

export async function socialLogin(params: SocialLoginParams): Promise<LoginResult> {
  const response = await requestDocumentJson(AUTH_SOCIAL_LOGIN, {
    method: "POST",
    body: JSON.stringify(params),
    headers: {
      "Content-Type": "application/json",
    },
  });

  const parsed = loginResultSchema.parse(response);
  if (parsed.status === "success") {
    return {
      status: "success",
      ...toAuthAccountContext(parsed),
    };
  }

  if (parsed.status === "required_mfa_setup") {
    return {
      status: "required_mfa_setup",
      challengeId: parsed.challengeId,
      challengeExpiresAt: parsed.challengeExpiresAt,
      requiredMfa: parsed.requiredMfa,
    };
  }

  return {
    status: "mfa_required",
    nextMfa: parsed.nextMfa,
    challengeId: parsed.challengeId,
    challengeState: parsed.challengeState,
    challengeExpiresAt: parsed.challengeExpiresAt,
    requiredMfa: parsed.requiredMfa,
    availableMfaMethods: parsed.availableMfaMethods,
  };
}

export async function socialAuthorize(
  params: SocialAuthorizeParams,
): Promise<SocialAuthorizeResult> {
  const response = await requestDocumentJson(AUTH_SOCIAL_AUTHORIZE, {
    method: "POST",
    body: JSON.stringify(params),
    headers: {
      "Content-Type": "application/json",
    },
  });

  return socialAuthorizeResultSchema.parse(response);
}

export type LdapLoginParams = {
  organizationSlug: string;
  username: string;
  password: string;
  ldapId?: string;
  applicationId: string;
  captchaToken?: string;
  captchaId?: string;
};

export async function ldapLogin(params: LdapLoginParams): Promise<LoginResult> {
  const response = await requestDocumentJson(AUTH_LOGIN_LDAP, {
    method: "POST",
    body: JSON.stringify(params),
    headers: {
      "Content-Type": "application/json",
    },
  });

  const parsed = loginResultSchema.parse(response);
  if (parsed.status === "success") {
    return {
      status: "success",
      ...toAuthAccountContext(parsed),
    };
  }

  if (parsed.status === "required_mfa_setup") {
    return {
      status: "required_mfa_setup",
      challengeId: parsed.challengeId,
      challengeExpiresAt: parsed.challengeExpiresAt,
      requiredMfa: parsed.requiredMfa,
    };
  }

  return {
    status: "mfa_required",
    nextMfa: parsed.nextMfa,
    challengeId: parsed.challengeId,
    challengeState: parsed.challengeState,
    challengeExpiresAt: parsed.challengeExpiresAt,
    requiredMfa: parsed.requiredMfa,
    availableMfaMethods: parsed.availableMfaMethods,
  };
}

export async function logoutSession() {
  const response = await requestDocumentJson(AUTH_LOGOUT, {
    method: "POST",
  });

  return logoutResponseSchema.parse(response);
}

// ─────────────────────────────────────────────
// Captcha Status
// ─────────────────────────────────────────────

export type CaptchaStatus = {
  required: boolean;
  providerType?: string;
  siteKey?: string;
};

export async function getCaptchaStatus(organizationId: string): Promise<CaptchaStatus> {
  const response = await requestDocumentJson(
    `${AUTH_CAPTCHA_STATUS}?organizationId=${encodeURIComponent(organizationId)}`,
  );
  return response as CaptchaStatus;
}

// ─────────────────────────────────────────────
// Captcha Generate & Verify
// ─────────────────────────────────────────────

export type CaptchaGenerateResult = {
  captchaId: string;
  captchaImage: string;
  type: string;
  /** go_captcha only: "click" | "slide". */
  shape?: string;
  /** go_captcha only: base64 sub image (click hint thumb / slide tile). */
  captchaImageSub?: string;
};

export type CaptchaVerifyResult = {
  valid: boolean;
};

export async function generateCaptcha(organizationId: string): Promise<CaptchaGenerateResult> {
  const response = await requestDocumentJson(
    `${AUTH_CAPTCHA}?organizationId=${encodeURIComponent(organizationId)}`,
  );
  return response as CaptchaGenerateResult;
}

export async function verifyCaptcha(
  organizationId: string,
  captchaId: string,
  captchaValue: string,
): Promise<CaptchaVerifyResult> {
  const response = await requestDocumentJson(AUTH_CAPTCHA_VERIFY, {
    method: "POST",
    body: JSON.stringify({ organizationId, captchaId, captchaValue }),
  });
  return response as CaptchaVerifyResult;
}

// ─────────────────────────────────────────────
// Verify Code (standalone)
// ─────────────────────────────────────────────

export type VerifyCodePayload = {
  dest?: string;
  code: string;
  type: "email" | "sms";
  phone?: string;
  countryCode?: string;
};

export async function verifyCode(payload: VerifyCodePayload): Promise<void> {
  await requestDocumentJson(AUTH_VERIFY_CODE, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

// ─────────────────────────────────────────────
// SSO Logout
// ─────────────────────────────────────────────

export type SsoLogoutResult = {
  logoutUrls: string[];
};

export async function ssoLogout(): Promise<SsoLogoutResult> {
  const response = await requestDocumentJson(AUTH_SSO_LOGOUT, {
    method: "POST",
  });
  return response as SsoLogoutResult;
}

// ─────────────────────────────────────────────
// CAS Single Logout
// ─────────────────────────────────────────────

export type CasLogoutResult = {
  revokedSessions: number;
  redirectRejected: boolean;
  /**
   * The adjudicated return target, decided by the server against the
   * organization's registered redirect URIs. Empty when the redirect was
   * refused. The console navigates to this value verbatim and never re-uses the
   * `service` value it requested with: re-deriving the verdict in the browser is
   * exactly the open redirect the server-side whitelist exists to prevent
   * (设计报告 42.2「应用」行：回跳集合是开放重定向白名单的唯一来源).
   */
  redirectUri: string;
};

/**
 * Ask the server to end this CAS session and fan the revocation out to every
 * application in the organization (设计报告 28.10.5 FE-014).
 *
 * `redirect=report` selects the body form of the verdict instead of the 302 the
 * CAS protocol surface returns by default: a fetch cannot read the `Location`
 * header of a redirect it is not allowed to follow, and it must not be the
 * browser that decides whether the target was registered.
 */
export async function casLogout(service: string): Promise<CasLogoutResult> {
  const params = new URLSearchParams({ redirect: "report" });
  if (service) {
    params.set("service", service);
  }

  const response = await requestDocumentJson(`${AUTH_CAS_LOGOUT}?${params.toString()}`, {
    method: "POST",
  });

  const parsed = response as Partial<CasLogoutResult> | undefined;
  return {
    revokedSessions: typeof parsed?.revokedSessions === "number" ? parsed.revokedSessions : 0,
    redirectRejected: parsed?.redirectRejected === true,
    redirectUri: typeof parsed?.redirectUri === "string" ? parsed.redirectUri : "",
  };
}

// ─────────────────────────────────────────────
// Self-service password change (AC-AUTH-009)
// ─────────────────────────────────────────────

export type ChangePasswordPayload = {
  oldPassword: string;
  newPassword: string;
};

export type ChangePasswordResult = {
  status: "changed";
  revokedCount: number;
  refreshCount: number;
  accessCount: number;
};

/** Self-service password change — caller must have valid __Host-session cookie. Backend pulls orgSlug + username from session context (defense-in-depth: caller-supplied fields are ignored). AC-AUTH-009: ForceLogoutUser full cascade + session rotation. */
export async function changeOwnPassword(
  payload: ChangePasswordPayload,
): Promise<ChangePasswordResult> {
  const res = await requestDocumentJson(AUTH_PASSWORD_CHANGE, {
    method: "POST",
    body: JSON.stringify({
      oldPassword: payload.oldPassword,
      newPassword: payload.newPassword,
    }),
  });
  return res as ChangePasswordResult;
}

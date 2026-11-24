import type {
  CreateApplicationPayload,
  ApplicationClientCertificateInput,
  ApplicationDetail,
  ApplicationCustomScope,
} from "@/shared/api/iam/application";
import type { ApplicationConfigFormValues } from "./tabs/types";
import { isMTLSAuthMethod } from "./tabs/types";
import { normalizeStringList } from "./application-helpers";
import { getDefaultSignupItems } from "@/shared/lib/signup-field-config";
import { evaluateOidcIssues } from "./application-oidc-validation";
import type { OidcValidationOptions } from "./application-oidc-validation";

/** JWK members that only exist in private keys; a client must never send them. */
const JWKS_PRIVATE_FIELDS = ["d", "p", "q", "dp", "dq", "qi"] as const;

/**
 * Validate the JWKS textarea before it reaches the API. Returns one localized
 * message per violation; an empty list means the input is acceptable.
 */
export function validateJwksInput(
  tr: (key: string, options?: Record<string, unknown>) => string,
  raw?: string,
): string[] {
  const trimmed = raw?.trim() ?? "";
  if (trimmed === "") {
    return [tr("application.validation.jwksRequired")];
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return [tr("application.validation.jwksInvalidJson")];
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return [tr("application.validation.jwksInvalidShape")];
  }
  const keys = (parsed as { keys?: unknown }).keys;
  if (!Array.isArray(keys) || keys.length === 0) {
    return [tr("application.validation.jwksRequired")];
  }

  const errors: string[] = [];
  keys.forEach((entry, index) => {
    if (typeof entry !== "object" || entry === null || Array.isArray(entry)) {
      errors.push(tr("application.validation.jwksInvalidShape"));
      return;
    }
    const key = entry as Record<string, unknown>;
    const kidd = typeof key.kid === "string" ? key.kid.trim() : "";
    const label = kidd || `#${index + 1}`;
    if (kidd === "") {
      errors.push(tr("application.validation.jwksKidRequired", { key: label }));
    }
    const kty = typeof key.kty === "string" ? key.kty : "";
    if (kty !== "RSA" && kty !== "EC") {
      errors.push(tr("application.validation.jwksKeyType", { key: label }));
    }
    const leaking = JWKS_PRIVATE_FIELDS.filter((field) => field in key);
    if (leaking.length > 0) {
      errors.push(
        tr("application.validation.jwksKeyPrivate", {
          key: label,
          fields: leaking.join(", "),
        }),
      );
    }
  });
  return errors;
}

/**
 * Cross-field validation run before any create/update request. It enforces the
 * method↔material exclusivity: a method must not be submitted with material it
 * does not use, and each material must satisfy its own shape rules.
 *
 * `requireClientJwks` defaults to true (create, or switching into
 * private_key_jwt). On edit of an already private_key_jwt client the detail
 * response does not project the stored JWKS, so an empty textarea means
 * "keep the existing keys", not "this client has no keys".
 */
export type ApplicationConfigValidationOptions = OidcValidationOptions;

export function validateApplicationConfigValues(
  tr: (key: string, options?: Record<string, unknown>) => string,
  values: ApplicationConfigFormValues,
  options?: ApplicationConfigValidationOptions,
): string[] {
  const errors = evaluateOidcIssues(values, options).map((issue) =>
    tr(`application.validation.${issue.code}`, issue.params),
  );
  const method = values.tokenEndpointAuthMethod;
  const jwks = (values.jwks ?? "").trim();
  if (method === "private_key_jwt" && jwks !== "") {
    errors.push(...validateJwksInput(tr, jwks));
  }
  return errors;
}

/**
 * Validate and normalize custom scopes
 */
function normalizeCustomScopes(
  tr: (key: string, options?: Record<string, unknown>) => string,
  values?: ApplicationConfigFormValues["customScopes"],
) {
  if (!Array.isArray(values)) {
    return [] as ApplicationCustomScope[];
  }

  const scopes: ApplicationCustomScope[] = [];
  const seen = new Set<string>();
  for (const value of values) {
    const scope = value?.scope?.trim() ?? "";
    const displayName = value?.displayName?.trim() ?? "";
    const description = value?.description?.trim() ?? "";

    if (scope === "") {
      if (displayName !== "" || description !== "") {
        throw new Error(tr("application.validation.scopeRequired"));
      }
      continue;
    }
    if (seen.has(scope)) {
      throw new Error(tr("application.validation.scopeDuplicate", { scope }));
    }
    seen.add(scope);
    scopes.push({ scope, displayName, description });
  }

  return scopes;
}

/**
 * Parse the JWKS textarea into the JSON object the API expects. An empty input
 * means "no JWKS sent" — callers decide whether that is allowed for the chosen
 * auth method (private_key_jwt requires at least one key).
 */
export function parseJwksInput(
  tr: (key: string, options?: Record<string, unknown>) => string,
  raw?: string,
): Record<string, unknown> | undefined {
  const trimmed = raw?.trim() ?? "";
  if (trimmed === "") {
    return undefined;
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    throw new Error(tr("application.validation.jwksInvalidJson"));
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    throw new Error(tr("application.validation.jwksInvalidJson"));
  }
  return parsed as Record<string, unknown>;
}

/**
 * Create default form values for a new application
 */
export function createDefaultApplicationConfigFormValues(
  organizationId?: string,
): ApplicationConfigFormValues {
  return {
    organizationId: organizationId ?? "",
    slug: "",
    displayName: "",
    category: "",
    appType: "",
    description: "",
    homepageUrl: "",
    domain: "",
    status: "active",
    clientId: "",
    clientSecret: "",
    tokenEndpointAuthMethod: "none",
    redirectUris: [],
    // A public client can only redeem the authorization_code grant; default
    // the form to the one combination that authenticates today.
    grantTypes: ["authorization_code"],
    // 新建应用预置一组可直接使用的 OIDC 配置，避免空表单让人无从下手；
    // 编辑态由 `buildApplicationConfigFormValues` 以服务端值为准。
    customScopes: [
      { scope: "openid", displayName: "OpenID", description: "Verify the user's identity" },
      { scope: "profile", displayName: "Profile", description: "Read the basic profile" },
      { scope: "email", displayName: "Email", description: "Read the email address" },
    ],
    logo: "",
    enableSamlCompress: false,
    enableSignup: true,
    enableGuestSignin: false,
    enableExclusiveSignin: false,
    enableAutoSignin: true, // align with backend schema z.boolean().default(true)
    enableLinkWithEmail: false,
    disableSignin: false,
    enableSigninSession: false,
    cookieExpireInHours: 168,
    tags: [],
    // 与真实注册页/预览帧共用同一默认项事实源，避免三处词表漂移。
    signupItems: getDefaultSignupItems(),
    signupUrl: "",
    forgetUrl: "",
    affiliationUrl: "",
    tokenFormat: "JWT",
    tokenFields: [],
    expireInHours: 24,
    refreshTokenExpireInHours: 168,
    tokenSigningMethod: "RS256",
    providerItems: [],
    providerIds: [],
    samlReplyUrl: "",
    certificate: "",
    customCss: "",
    customHeader: "",
    signinUrl: "",
    logoutUrl: "",
    logoutRedirectUrl: "",
    isPortal: false,
    isVisible: true,
    promptUrl: "",
    samlMetadata: "",
    isShared: false,
    title: "",
    favicon: "",
    menuOrder: 0,
    menuMode: "",
    forcedRedirectOrigin: "",
    samlNameIdFormat: "",
    samlNameIdContent: "",
    samlSignRequest: false,
    samlMetadataUrl: "",
    samlMetadataSource: "keep",
    termsOfUse: "",
    privacyPolicy: "",
    consentMode: "explicit",
    dpopRequired: false,
    pkceRequired: false,
    defaultGroup: "",
    orgChoiceMode: "",
    enableSamlC14n10: false,
    enableSamlPostBinding: false,
    disableSamlAttributes: false,
    enableSamlAssertionSignature: false,
    useEmailAsSamlNameId: false,
    codeExpire: 5,
    signinMethods: ["Password"],
    signinItems: [
      { name: "Password", visible: true, rule: "Optional" },
      { name: "Verification code", visible: true, rule: "Optional" },
      { name: "WebAuthn", visible: true, rule: "Optional" },
    ],
    formCss: "",
    formCssMobile: "",
    formSideHtml: "",
    formBackgroundUrl: "",
    formBackgroundUrlMobile: "",
    formOffset: "2",
    formPosition: "",
    headerHtml: "",
    footerHtml: "",
    signupHtml: "",
    signinHtml: "",
    ipWhitelist: "",
    ipBlacklist: "",
    failedSigninLimit: 5,
    failedSigninFrozenTime: 15,
    codeResendTimeout: 60,
    corsAllowedOrigins: "",
    enableBetaOidc: false,
    backchannelLogoutUri: "",
    frontchannelLogoutUri: "",
    frontchannelLogoutSessionRequired: false,
    otherDomains: [],
    upstreamHost: "",
    sslMode: "",
    sslCert: "",
  };
}

/**
 * Build the API payload for creating an application
 */
export function buildCreateApplicationPayload(
  tr: (key: string, options?: Record<string, unknown>) => string,
  values: ApplicationConfigFormValues,
  clientCertificates?: ApplicationClientCertificateInput[],
) {
  const organizationId = values.organizationId?.trim() ?? "";
  const slug = values.slug?.trim() ?? "";
  const clientSecret = values.clientSecret?.trim() ?? "";

  if (organizationId === "") {
    throw new Error(tr("application.validation.orgIdRequired"));
  }
  if (slug === "") {
    throw new Error(tr("application.validation.slugRequired"));
  }
  // Confidential client missing clientSecret — allowed on create; backend
  // auto-generates one via generateOAuthClientSecret(). On edit, leave
  // clientSecret blank to retain the existing secret (handled by backend).
  // Previously we threw here, forcing callers to provide a secret upfront.

  const payload: CreateApplicationPayload & Record<string, unknown> = {
    organizationId,
    slug,
    displayName: values.displayName.trim(),
    category: values.category?.trim() ?? "",
    appType: values.appType?.trim() ?? "",
    description: values.description?.trim() ?? "",
    homepageUrl: values.homepageUrl?.trim() ?? "",
    domain: values.domain?.trim() ?? "",
    status: values.status.trim(),
    clientId: values?.clientId?.trim() ?? "",
    tokenEndpointAuthMethod: values.tokenEndpointAuthMethod,
    redirectUris: normalizeStringList(values.redirectUris),
    grantTypes: normalizeStringList(values.grantTypes),
    customScopes: normalizeCustomScopes(tr, values.customScopes),
    ...(values.tokenEndpointAuthMethod === "none" ? { clientSecret: "" } : { clientSecret }),
    ...(values.tokenEndpointAuthMethod === "private_key_jwt" && values.jwks?.trim()
      ? { jwks: parseJwksInput(tr, values.jwks) }
      : {}),
    ...(values.tokenEndpointAuthMethod === "private_key_jwt" && values.jwksUri?.trim()
      ? { jwksUri: values.jwksUri.trim() }
      : {}),
    ...(isMTLSAuthMethod(values.tokenEndpointAuthMethod) && clientCertificates?.length
      ? { clientCertificates }
      : {}),
    logo: values.logo?.trim() ?? "",
    enableSamlCompress: values.enableSamlCompress ?? false,
    enableSignup: values.enableSignup ?? true,
    enableGuestSignin: values.enableGuestSignin ?? false,
    enableExclusiveSignin: values.enableExclusiveSignin ?? false,
    enableAutoSignin: values.enableAutoSignin ?? false,
    enableLinkWithEmail: values.enableLinkWithEmail ?? false,
    disableSignin: values.disableSignin ?? false,
    enableSigninSession: values.enableSigninSession ?? false,
    cookieExpireInHours: values.cookieExpireInHours ?? 168,
    tags: values.tags ?? [],
    signupItems: values.signupItems ?? [],
    signupUrl: values.signupUrl?.trim() ?? "",
    forgetUrl: values.forgetUrl?.trim() ?? "",
    affiliationUrl: values.affiliationUrl?.trim() ?? "",
    tokenFormat: values.tokenFormat?.trim() ?? "JWT",
    tokenFields: normalizeStringList(values.tokenFields),
    expireInHours: values.expireInHours ?? 24,
    refreshTokenExpireInHours: values.refreshTokenExpireInHours ?? 168,
    tokenSigningMethod: values.tokenSigningMethod?.trim() ?? "RS256",
    providerItems: values.providerItems ?? [],
    samlReplyUrl: values.samlReplyUrl?.trim() ?? "",
    certificate: values.certificate?.trim() ?? "",
    customCss: values.customCss ?? "",
    customHeader: values.customHeader ?? "",
    signinUrl: values.signinUrl?.trim() ?? "",
    logoutUrl: values.logoutUrl?.trim() ?? "",
    logoutRedirectUrl: values.logoutRedirectUrl?.trim() ?? "",
    isPortal: values.isPortal ?? false,
    isVisible: values.isVisible ?? true,
    promptUrl: values.promptUrl?.trim() ?? "",
    // 默认（"keep"）两套来源都原样提交，后端按 URL 优先解析；只有用户本次显式
    // 切换了来源，才在提交时清空另一侧，避免"只改一个无关字段"静默抹掉 XML。
    samlMetadata: values.samlMetadataSource === "url" ? "" : (values.samlMetadata ?? ""),
    isShared: values.isShared ?? false,
    title: values.title?.trim() ?? "",
    favicon: values.favicon?.trim() ?? "",

    menuOrder: values.menuOrder ?? 0,
    menuMode: values.menuMode?.trim() ?? "",
    forcedRedirectOrigin: values.forcedRedirectOrigin?.trim() ?? "",
    samlNameIdFormat: values.samlNameIdFormat?.trim() ?? "",
    samlNameIdContent: values.samlNameIdContent?.trim() ?? "",
    samlSignRequest: values.samlSignRequest ?? false,
    samlMetadataUrl:
      values.samlMetadataSource === "xml" ? "" : (values.samlMetadataUrl?.trim() ?? ""),
    termsOfUse: values.termsOfUse?.trim() ?? "",
    privacyPolicy: values.privacyPolicy?.trim() ?? "",
    consentMode: values.consentMode?.trim() ?? "explicit",
    dpopRequired: values.dpopRequired ?? false,
    pkceRequired: values.pkceRequired ?? false,
    defaultGroup: values.defaultGroup?.trim() ?? "",
    orgChoiceMode: values.orgChoiceMode?.trim() ?? "",
    enableSamlC14n10: values.enableSamlC14n10 ?? false,
    enableSamlPostBinding: values.enableSamlPostBinding ?? false,
    disableSamlAttributes: values.disableSamlAttributes ?? false,
    enableSamlAssertionSignature: values.enableSamlAssertionSignature ?? false,
    useEmailAsSamlNameId: values.useEmailAsSamlNameId ?? false,
    codeExpire: values.codeExpire ?? 5,
    signinItems: values.signinItems ?? [],
    formCss: values.formCss ?? "",
    formCssMobile: values.formCssMobile ?? "",
    formSideHtml: values.formSideHtml ?? "",
    formBackgroundUrl: values.formBackgroundUrl?.trim() ?? "",
    formBackgroundUrlMobile: values.formBackgroundUrlMobile?.trim() ?? "",
    formOffset: values.formOffset ?? "2",
    formPosition: values.formPosition?.trim() ?? "",
    headerHtml: values.headerHtml ?? "",
    footerHtml: values.footerHtml ?? "",
    signupHtml: values.signupHtml ?? "",
    signinHtml: values.signinHtml ?? "",
    ipWhitelist: values.ipWhitelist?.trim() ?? "",
    ipBlacklist: values.ipBlacklist?.trim() ?? "",
    failedSigninLimit: values.failedSigninLimit ?? 5,
    failedSigninFrozenTime: values.failedSigninFrozenTime ?? 15,
    codeResendTimeout: values.codeResendTimeout ?? 60,
    corsAllowedOrigins: values.corsAllowedOrigins?.trim() ?? "",
    enableBetaOidc: values.enableBetaOidc ?? false,
    backchannelLogoutUri: values.backchannelLogoutUri?.trim() ?? "",
    frontchannelLogoutUri: values.frontchannelLogoutUri?.trim() ?? "",
    frontchannelLogoutSessionRequired: values.frontchannelLogoutSessionRequired ?? false,
    otherDomains: normalizeStringList(values.otherDomains),
    upstreamHost: values.upstreamHost?.trim() ?? "",
    sslMode: values.sslMode?.trim() ?? "",
    sslCert: values.sslCert?.trim() ?? "",
    samlAttributes: values.samlAttributes ?? [],
    samlHashAlgorithm: values.samlHashAlgorithm?.trim() ?? "",
    samlC14nPrefix: values.samlC14nPrefix?.trim() ?? "",
    tokenAttributes: values.tokenAttributes ?? [],
    signinMethods: values.signinMethods ?? [],
  };

  // POST /applications 的请求体只接受基础字段（后端 CreateApplicationCommand
  // 尚未携带扩展配置），多传任意字段会被 DisallowUnknownFields 判为 400。
  // 其余配置由创建成功后的 PATCH 落库，具体见控制器里的两步创建。
  for (const key of Object.keys(payload)) {
    if (!CREATE_APPLICATION_SUPPORTED_KEYS.has(key)) delete payload[key];
  }
  return payload;
}

/**
 * 与后端 CreateApplicationRequest 完全一致的可写字段集合。
 * 字段来源：internal/transport/http/api/v1/applications/request.go
 */
const CREATE_APPLICATION_SUPPORTED_KEYS = new Set([
  "organizationId",
  "slug",
  "displayName",
  "category",
  "appType",
  "description",
  "homepageUrl",
  "domain",
  "status",
  "clientId",
  "clientSecret",
  "tokenEndpointAuthMethod",
  "jwks",
  "jwksUri",
  "clientCertificates",
  "redirectUris",
  "grantTypes",
  "customScopes",
  "isShared",
  "title",
  "favicon",
  "menuOrder",
  "menuMode",
  "enableSignup",
  "enableExclusiveSignin",
  "enableGuestSignin",
  "enableAutoSignin",
  "enableLinkWithEmail",
  "cookieExpireInHours",
  "signupUrl",
  "forgetUrl",
  "affiliationUrl",
  "tokenFields",
  "forcedRedirectOrigin",
  "codeExpire",
  "samlNameIdFormat",
  "samlNameIdContent",
  "samlSignRequest",
  "samlMetadataUrl",
  "enableSamlC14n10",
  "enableSamlPostBinding",
  "disableSamlAttributes",
  "enableSamlAssertionSignature",
  "useEmailAsSamlNameId",
  "consentMode",
  "dpopRequired",
  "pkceRequired",
  "ipWhitelist",
  "ipBlacklist",
  "failedSigninLimit",
  "failedSigninFrozenTime",
  "codeResendTimeout",
  "corsAllowedOrigins",
  "enableBetaOidc",
  "defaultGroup",
  "orgChoiceMode",
  "formCss",
  "formCssMobile",
  "formSideHtml",
  "formBackgroundUrl",
  "formBackgroundUrlMobile",
  "formOffset",
  "formPosition",
  "headerHtml",
  "footerHtml",
  "signupHtml",
  "signinHtml",
  "termsOfUse",
  "privacyPolicy",
  "backchannelLogoutUri",
  "otherDomains",
  "upstreamHost",
  "sslMode",
  "sslCert",
  "clientCert",
  "samlHashAlgorithm",
  "samlC14nPrefix",
  "pageHtml",
]);

/**
 * Build the API payload for updating application configuration
 */
export function buildApplicationConfigPayload(
  tr: (key: string, options?: Record<string, unknown>) => string,
  values: ApplicationConfigFormValues,
  detail: ApplicationDetail,
  clientCertificates?: ApplicationClientCertificateInput[],
) {
  const clientSecret = values.clientSecret?.trim() ?? "";
  // jwksUri 对齐后端 *string 语义：空串删除已存 URI。只在值变化（含认证方式
  // 离开 private_key_jwt 后的清空）时提交，避免每次保存都触发一次删除。
  const nextJwksUri =
    values.tokenEndpointAuthMethod === "private_key_jwt" ? (values.jwksUri?.trim() ?? "") : "";
  // 注意：不再在此前端校验 "public → confidential 必须提供 clientSecret"。
  // 该校验由后端 PatchApplication 的 validateOAuthClientSettings 负责：
  // 后端能拿到原始 application 状态（包括 ClientSecretHash 和原始 method），
  // 可以准确判断"本次变更是否引入了新的不一致"，从而避免 React Query cache
  // 过期或历史遗留配置导致的前端误判。

  return {
    slug: values.slug?.trim() ?? detail.slug,
    displayName: values.displayName.trim(),
    category: values.category?.trim() ?? "",
    appType: values.appType?.trim() ?? "",
    description: values.description?.trim() ?? "",
    homepageUrl: values.homepageUrl?.trim() ?? "",
    domain: values.domain?.trim() ?? "",
    status: values.status.trim(),
    // clientId is immutable (RFC 6749) — never transmit on update.
    // Backend PatchApplication rejects any mutation attempt as defense-in-depth.
    // tokenEndpointAuthMethod — 只在和原值不同时才发，避免 React Query cache
    // 过期导致后端误判"用户引入了新的 OIDC 不合法"。
    ...(values.tokenEndpointAuthMethod !== detail.tokenEndpointAuthMethod
      ? { tokenEndpointAuthMethod: values.tokenEndpointAuthMethod }
      : {}),
    redirectUris: normalizeStringList(values.redirectUris),
    grantTypes: normalizeStringList(values.grantTypes),
    customScopes: normalizeCustomScopes(tr, values.customScopes),
    // revision 只增不减：取表单与服务端快照的较大值。
    // 表单可能刚由保存响应同步到 N+1，而 detail 仍是缓存中的 N；
    // 反过来旧草稿里的 revision 也不能覆盖更新的 detail。
    revision: Math.max(values.revision ?? 0, detail.revision ?? 0),
    ...(values.tokenEndpointAuthMethod === "none"
      ? { clientSecret: "" }
      : clientSecret !== ""
        ? { clientSecret }
        : {}),
    ...(values.tokenEndpointAuthMethod === "private_key_jwt" && values.jwks?.trim()
      ? { jwks: parseJwksInput(tr, values.jwks) }
      : {}),
    ...(nextJwksUri !== (detail.jwksUri ?? "") ? { jwksUri: nextJwksUri } : {}),
    ...(clientCertificates && clientCertificates.length > 0 ? { clientCertificates } : {}),
    logo: values.logo?.trim() ?? "",
    enableSamlCompress: values.enableSamlCompress,
    enableSignup: values.enableSignup,
    enableGuestSignin: values.enableGuestSignin,
    enableExclusiveSignin: values.enableExclusiveSignin,
    enableAutoSignin: values.enableAutoSignin,
    enableLinkWithEmail: values.enableLinkWithEmail,
    disableSignin: values.disableSignin,
    enableSigninSession: values.enableSigninSession,
    cookieExpireInHours: values.cookieExpireInHours,
    tags: values.tags,
    signupItems: values.signupItems,
    signupUrl: values.signupUrl?.trim() ?? "",
    forgetUrl: values.forgetUrl?.trim() ?? "",
    affiliationUrl: values.affiliationUrl?.trim() ?? "",
    tokenFormat: values.tokenFormat?.trim() ?? "JWT",
    tokenFields: normalizeStringList(values.tokenFields),
    expireInHours: values.expireInHours,
    refreshTokenExpireInHours: values.refreshTokenExpireInHours,
    tokenSigningMethod: values.tokenSigningMethod?.trim() ?? "RS256",
    providerItems: values.providerItems,
    samlReplyUrl: values.samlReplyUrl?.trim() ?? "",
    certificate: values.certificate?.trim() ?? "",
    customCss: values.customCss ?? "",
    customHeader: values.customHeader ?? "",
    signinUrl: values.signinUrl?.trim() ?? "",
    logoutUrl: values.logoutUrl?.trim() ?? "",
    logoutRedirectUrl: values.logoutRedirectUrl?.trim() ?? "",
    isPortal: values.isPortal,
    isVisible: values.isVisible,
    promptUrl: values.promptUrl?.trim() ?? "",
    // 与创建一致：未显式切换来源时两套都提交，显式切换才清空另一侧。
    samlMetadata: values.samlMetadataSource === "url" ? "" : (values.samlMetadata ?? ""),
    isShared: values.isShared,
    title: values.title?.trim() ?? "",
    favicon: values.favicon?.trim() ?? "",

    menuOrder: values.menuOrder,
    menuMode: values.menuMode?.trim() ?? "",
    forcedRedirectOrigin: values.forcedRedirectOrigin?.trim() ?? "",
    samlNameIdFormat: values.samlNameIdFormat?.trim() ?? "",
    samlNameIdContent: values.samlNameIdContent?.trim() ?? "",
    samlSignRequest: values.samlSignRequest,
    samlMetadataUrl:
      values.samlMetadataSource === "xml" ? "" : (values.samlMetadataUrl?.trim() ?? ""),
    termsOfUse: values.termsOfUse?.trim() ?? "",
    privacyPolicy: values.privacyPolicy?.trim() ?? "",
    consentMode: values.consentMode?.trim() ?? "explicit",
    dpopRequired: values.dpopRequired,
    pkceRequired: values.pkceRequired,
    defaultGroup: values.defaultGroup?.trim() ?? "",
    orgChoiceMode: values.orgChoiceMode?.trim() ?? "",
    enableSamlC14n10: values.enableSamlC14n10,
    enableSamlPostBinding: values.enableSamlPostBinding,
    disableSamlAttributes: values.disableSamlAttributes,
    enableSamlAssertionSignature: values.enableSamlAssertionSignature,
    useEmailAsSamlNameId: values.useEmailAsSamlNameId,
    codeExpire: values.codeExpire,
    signinItems: values.signinItems,
    formCss: values.formCss ?? "",
    formCssMobile: values.formCssMobile ?? "",
    formSideHtml: values.formSideHtml ?? "",
    formBackgroundUrl: values.formBackgroundUrl?.trim() ?? "",
    formBackgroundUrlMobile: values.formBackgroundUrlMobile?.trim() ?? "",
    formOffset: values.formOffset,
    formPosition: values.formPosition?.trim() ?? "",
    headerHtml: values.headerHtml ?? "",
    footerHtml: values.footerHtml ?? "",
    signupHtml: values.signupHtml ?? "",
    signinHtml: values.signinHtml ?? "",
    ipWhitelist: values.ipWhitelist?.trim() ?? "",
    ipBlacklist: values.ipBlacklist?.trim() ?? "",
    failedSigninLimit: values.failedSigninLimit,
    failedSigninFrozenTime: values.failedSigninFrozenTime,
    codeResendTimeout: values.codeResendTimeout,
    corsAllowedOrigins: values.corsAllowedOrigins?.trim() ?? "",
    enableBetaOidc: values.enableBetaOidc,
    backchannelLogoutUri: values.backchannelLogoutUri?.trim() ?? "",
    frontchannelLogoutUri: values.frontchannelLogoutUri?.trim() ?? "",
    frontchannelLogoutSessionRequired: values.frontchannelLogoutSessionRequired ?? false,
    otherDomains: normalizeStringList(values.otherDomains),
    upstreamHost: values.upstreamHost?.trim() ?? "",
    sslMode: values.sslMode?.trim() ?? "",
    sslCert: values.sslCert?.trim() ?? "",
    samlAttributes: values.samlAttributes ?? [],
    samlHashAlgorithm: values.samlHashAlgorithm?.trim() ?? "",
    samlC14nPrefix: values.samlC14nPrefix?.trim() ?? "",
    tokenAttributes: values.tokenAttributes ?? [],
    signinMethods: values.signinMethods ?? [],
  };
}

/**
 * Normalize configuration snapshot for change detection.
 * Compares ALL form fields so any change triggers the dirty flag.
 */
export function normalizeConfigSnapshot(
  tr: (key: string, options?: Record<string, unknown>) => string,
  values?: Partial<ApplicationConfigFormValues>,
) {
  return {
    organizationId: values?.organizationId?.trim() ?? "",
    slug: values?.slug?.trim() ?? "",
    displayName: values?.displayName?.trim() ?? "",
    category: values?.category?.trim() ?? "",
    appType: values?.appType?.trim() ?? "",
    description: values?.description?.trim() ?? "",
    homepageUrl: values?.homepageUrl?.trim() ?? "",
    domain: values?.domain?.trim() ?? "",
    status: values?.status?.trim() ?? "",
    // clientId is immutable (RFC 6749) — Update path must omit it.
    clientSecret: values?.clientSecret?.trim() ?? "",
    mtlsCertificates: JSON.stringify(values?.mtlsCertificates ?? []),
    tokenEndpointAuthMethod: values?.tokenEndpointAuthMethod ?? "none",
    jwksUri: values?.jwksUri?.trim() ?? "",
    redirectUris: normalizeStringList(values?.redirectUris),
    grantTypes: normalizeStringList(values?.grantTypes),
    customScopes: normalizeCustomScopes(tr, values?.customScopes),
    logo: values?.logo?.trim() ?? "",
    isShared: values?.isShared ?? false,
    title: values?.title?.trim() ?? "",
    favicon: values?.favicon?.trim() ?? "",
    menuOrder: values?.menuOrder ?? 0,
    menuMode: values?.menuMode?.trim() ?? "",
    enableSamlCompress: values?.enableSamlCompress ?? false,
    enableSignup: values?.enableSignup ?? true,
    enableGuestSignin: values?.enableGuestSignin ?? false,
    enableExclusiveSignin: values?.enableExclusiveSignin ?? false,
    enableAutoSignin: values?.enableAutoSignin ?? false,
    enableLinkWithEmail: values?.enableLinkWithEmail ?? false,
    cookieExpireInHours: values?.cookieExpireInHours ?? 168,
    signupItems: JSON.stringify(values?.signupItems ?? []),
    signinMethods: JSON.stringify(values?.signinMethods ?? []),
    signinItems: JSON.stringify(values?.signinItems ?? []),
    signupUrl: values?.signupUrl?.trim() ?? "",
    forgetUrl: values?.forgetUrl?.trim() ?? "",
    affiliationUrl: values?.affiliationUrl?.trim() ?? "",
    tokenFormat: values?.tokenFormat?.trim() ?? "JWT",
    tokenFields: normalizeStringList(values?.tokenFields),
    tokenAttributes: JSON.stringify(values?.tokenAttributes ?? []),
    expireInHours: values?.expireInHours ?? 24,
    refreshTokenExpireInHours: values?.refreshTokenExpireInHours ?? 168,
    tokenSigningMethod: values?.tokenSigningMethod?.trim() ?? "RS256",
    codeExpire: values?.codeExpire ?? 5,
    forcedRedirectOrigin: values?.forcedRedirectOrigin?.trim() ?? "",
    providerItems: JSON.stringify(values?.providerItems ?? []),
    providerIds: [...normalizeStringList(values?.providerIds)].sort(),
    samlReplyUrl: values?.samlReplyUrl?.trim() ?? "",
    certificate: values?.certificate?.trim() ?? "",
    samlNameIdFormat: values?.samlNameIdFormat?.trim() ?? "",
    samlNameIdContent: values?.samlNameIdContent?.trim() ?? "",
    samlSignRequest: values?.samlSignRequest ?? false,
    samlMetadataUrl: values?.samlMetadataUrl?.trim() ?? "",
    samlMetadataSource: values?.samlMetadataSource ?? "keep",
    customCss: values?.customCss ?? "",
    customHeader: values?.customHeader ?? "",
    formCss: values?.formCss ?? "",
    formCssMobile: values?.formCssMobile ?? "",
    formSideHtml: values?.formSideHtml ?? "",
    formBackgroundUrl: values?.formBackgroundUrl?.trim() ?? "",
    formBackgroundUrlMobile: values?.formBackgroundUrlMobile?.trim() ?? "",
    formOffset: values?.formOffset ?? "2",
    formPosition: values?.formPosition?.trim() ?? "",
    headerHtml: values?.headerHtml ?? "",
    footerHtml: values?.footerHtml ?? "",
    signupHtml: values?.signupHtml ?? "",
    signinHtml: values?.signinHtml ?? "",
    signinUrl: values?.signinUrl?.trim() ?? "",
    logoutUrl: values?.logoutUrl?.trim() ?? "",
    logoutRedirectUrl: values?.logoutRedirectUrl?.trim() ?? "",
    isPortal: values?.isPortal ?? false,
    isVisible: values?.isVisible ?? true,
    promptUrl: values?.promptUrl?.trim() ?? "",
    samlMetadata: values?.samlMetadata ?? "",
    samlAttributes: JSON.stringify(values?.samlAttributes ?? []),
    samlHashAlgorithm: values?.samlHashAlgorithm?.trim() ?? "",
    samlC14nPrefix: values?.samlC14nPrefix?.trim() ?? "",
    enableSamlC14n10: values?.enableSamlC14n10 ?? false,
    enableSamlPostBinding: values?.enableSamlPostBinding ?? false,
    disableSamlAttributes: values?.disableSamlAttributes ?? false,
    enableSamlAssertionSignature: values?.enableSamlAssertionSignature ?? false,
    useEmailAsSamlNameId: values?.useEmailAsSamlNameId ?? false,
    termsOfUse: values?.termsOfUse?.trim() ?? "",
    privacyPolicy: values?.privacyPolicy?.trim() ?? "",
    consentMode: values?.consentMode?.trim() ?? "explicit",
    dpopRequired: values?.dpopRequired ?? false,
    pkceRequired: values?.pkceRequired ?? false,
    defaultGroup: values?.defaultGroup?.trim() ?? "",
    orgChoiceMode: values?.orgChoiceMode?.trim() ?? "",
    ipWhitelist: values?.ipWhitelist?.trim() ?? "",
    ipBlacklist: values?.ipBlacklist?.trim() ?? "",
    failedSigninLimit: values?.failedSigninLimit ?? 5,
    failedSigninFrozenTime: values?.failedSigninFrozenTime ?? 15,
    codeResendTimeout: values?.codeResendTimeout ?? 60,
    corsAllowedOrigins: values?.corsAllowedOrigins?.trim() ?? "",
    enableBetaOidc: values?.enableBetaOidc ?? false,
    backchannelLogoutUri: values?.backchannelLogoutUri?.trim() ?? "",
    frontchannelLogoutUri: values?.frontchannelLogoutUri?.trim() ?? "",
    frontchannelLogoutSessionRequired: values?.frontchannelLogoutSessionRequired ?? false,
    otherDomains: normalizeStringList(values?.otherDomains),
    upstreamHost: values?.upstreamHost?.trim() ?? "",
    sslMode: values?.sslMode?.trim() ?? "",
    sslCert: values?.sslCert?.trim() ?? "",
  };
}

/**
 * Map API ApplicationDetail to the config snapshot baseline used for dirty
 * detection on the edit form. Only the identity fields the form preloads are
 * included; normalizeConfigSnapshot fills the remaining fields with defaults.
 */
export function detailToConfigSnapshotValues(
  detail: ApplicationDetail,
): Partial<ApplicationConfigFormValues> {
  return {
    organizationId: detail.organizationId,
    slug: detail.slug,
    displayName: detail.displayName,
    category: detail.category,
    appType: detail.appType,
    description: detail.description,
    homepageUrl: detail.homepageUrl,
    domain: detail.domain,
    status: detail.status,
    clientId: detail.clientId,
    clientSecret: "",
    tokenEndpointAuthMethod: detail.tokenEndpointAuthMethod,
    jwksUri: detail.jwksUri ?? "",
    redirectUris: detail.redirectUris,
    grantTypes: detail.grantTypes,
    customScopes: detail.customScopes,
  };
}

/**
 * Map API ApplicationDetail to initial form values
 */
/** 表单位置统一为 "1".."4"；历史默认 "0"/空 与运行期 getFormPositionStyles 的居中 fallback 对齐为 "2"。 */
function normalizeFormOffset(value: string | null | undefined): string {
  const v = `${value ?? ""}`.trim();
  return v === "" || v === "0" ? "2" : v;
}

/** orgChoiceMode 唯一词表为 ""/direct/search（AuthenticationTab）；
 * 旧界面定制 Tab 曾写入 None/Select/Input，加载时归一，避免存量值落选。 */
function normalizeOrgChoiceMode(value: string | null | undefined): string {
  const v = `${value ?? ""}`.trim().toLowerCase();
  if (v === "none") return "";
  if (v === "select" || v === "direct") return "direct";
  if (v === "input" || v === "search") return "search";
  return "";
}

/** 界面定制 Tab 渲染的字段全集：撤销/重做与整组重置只作用于这些键，
 * 不波及基本信息/认证等其他 Tab 的编辑状态。 */
export const UI_CUSTOMIZATION_FIELDS = [
  "formOffset",
  "signinItems",
  "signupItems",
  "formBackgroundUrl",
  "formBackgroundUrlMobile",
  "formCss",
  "formCssMobile",
  "formSideHtml",
  "headerHtml",
  "footerHtml",
  "signupHtml",
  "signinHtml",
  "customCss",
  "customHeader",
  "logoutUrl",
  "logoutRedirectUrl",
  "termsOfUse",
  "frontchannelLogoutUri",
  "frontchannelLogoutSessionRequired",
  "privacyPolicy",
  "backchannelLogoutUri",
  "signinUrl",
  "signupUrl",
  "forgetUrl",
  "affiliationUrl",
  "promptUrl",
] as const satisfies ReadonlyArray<keyof ApplicationConfigFormValues>;

export function detailToConfigFormValues(
  detail: ApplicationDetail,
): Partial<ApplicationConfigFormValues> {
  return {
    revision: detail.revision,
    organizationId: detail.organizationId,
    slug: detail.slug,
    displayName: detail.displayName,
    category: detail.category,
    appType: detail.appType,
    description: detail.description,
    homepageUrl: detail.homepageUrl,
    domain: detail.domain,
    status: detail.status,
    clientId: detail.clientId,
    clientSecret: "",
    tokenEndpointAuthMethod: detail.tokenEndpointAuthMethod,
    jwksUri: detail.jwksUri ?? "",
    redirectUris: detail.redirectUris,
    grantTypes: detail.grantTypes,
    customScopes: detail.customScopes,
    logo: detail.logo ?? "",
    enableSamlCompress: detail.enableSamlCompress ?? false,
    enableSignup: detail.enableSignup ?? true,
    enableGuestSignin: detail.enableGuestSignin ?? false,
    enableExclusiveSignin: detail.enableExclusiveSignin ?? false,
    enableAutoSignin: detail.enableAutoSignin ?? true,
    enableLinkWithEmail: detail.enableLinkWithEmail ?? false,
    disableSignin: detail.disableSignin ?? false,
    enableSigninSession: detail.enableSigninSession ?? false,
    cookieExpireInHours: detail.cookieExpireInHours ?? 168,
    tags: detail.tags ?? [],
    signupItems: detail.signupItems ?? [],
    signinMethods: detail.signinMethods ?? [],
    tokenAttributes: detail.tokenAttributes ?? [],
    signupUrl: detail.signupUrl ?? "",
    forgetUrl: detail.forgetUrl ?? "",
    affiliationUrl: detail.affiliationUrl ?? "",
    tokenFormat: detail.tokenFormat ?? "JWT",
    tokenFields: detail.tokenFields ?? [],
    expireInHours: detail.expireInHours ?? 24,
    refreshTokenExpireInHours: detail.refreshTokenExpireInHours ?? 168,
    tokenSigningMethod: detail.tokenSigningMethod ?? "RS256",
    providerItems: detail.providerItems ?? [],
    providerIds: detail.providerIds ?? [],
    samlReplyUrl: detail.samlReplyUrl ?? "",
    certificate: detail.certificate ?? "",
    customCss: detail.customCss ?? "",
    customHeader: detail.customHeader ?? "",
    signinUrl: detail.signinUrl ?? "",
    logoutUrl: detail.logoutUrl ?? "",
    logoutRedirectUrl: detail.logoutRedirectUrl ?? "",
    isPortal: detail.isPortal ?? false,
    isVisible: detail.isVisible ?? true,
    promptUrl: detail.promptUrl ?? "",
    samlMetadata: detail.samlMetadata ?? "",
    isShared: detail.isShared ?? false,
    title: detail.title ?? "",
    favicon: detail.favicon ?? "",
    menuOrder: detail.menuOrder ?? 0,
    menuMode: detail.menuMode ?? "",
    forcedRedirectOrigin: detail.forcedRedirectOrigin ?? "",
    samlHashAlgorithm: detail.samlHashAlgorithm ?? "",
    samlC14nPrefix: detail.samlC14nPrefix ?? "",
    samlNameIdFormat: detail.samlNameIdFormat ?? "",
    samlNameIdContent: detail.samlNameIdContent ?? "",
    samlSignRequest: detail.samlSignRequest ?? false,
    samlMetadataUrl: detail.samlMetadataUrl ?? "",
    // "keep" means the operator has not switched source in this visit: submit
    // whatever both sides hold and let the backend resolve URL-first.
    samlMetadataSource: "keep",
    termsOfUse: detail.termsOfUse ?? "",
    privacyPolicy: detail.privacyPolicy ?? "",
    consentMode: detail.consentMode ?? "explicit",
    dpopRequired: detail.dpopRequired ?? false,
    pkceRequired: detail.pkceRequired ?? false,
    defaultGroup: detail.defaultGroup ?? "",
    orgChoiceMode: normalizeOrgChoiceMode(detail.orgChoiceMode),
    enableSamlC14n10: detail.enableSamlC14n10 ?? false,
    enableSamlPostBinding: detail.enableSamlPostBinding ?? false,
    disableSamlAttributes: detail.disableSamlAttributes ?? false,
    enableSamlAssertionSignature: detail.enableSamlAssertionSignature ?? false,
    useEmailAsSamlNameId: detail.useEmailAsSamlNameId ?? false,
    codeExpire: detail.codeExpire ?? 5,
    signinItems: detail.signinItems ?? [],
    formCss: detail.formCss ?? "",
    formCssMobile: detail.formCssMobile ?? "",
    formSideHtml: detail.formSideHtml ?? "",
    formBackgroundUrl: detail.formBackgroundUrl ?? "",
    formBackgroundUrlMobile: detail.formBackgroundUrlMobile ?? "",
    formOffset: normalizeFormOffset(detail.formOffset),
    formPosition: detail.formPosition ?? "",
    headerHtml: detail.headerHtml ?? "",
    footerHtml: detail.footerHtml ?? "",
    signupHtml: detail.signupHtml ?? "",
    signinHtml: detail.signinHtml ?? "",
    ipWhitelist: detail.ipWhitelist ?? "",
    ipBlacklist: detail.ipBlacklist ?? "",
    failedSigninLimit: detail.failedSigninLimit ?? 5,
    failedSigninFrozenTime: detail.failedSigninFrozenTime ?? 15,
    codeResendTimeout: detail.codeResendTimeout ?? 60,
    corsAllowedOrigins: detail.corsAllowedOrigins ?? "",
    enableBetaOidc: detail.enableBetaOidc ?? false,
    backchannelLogoutUri: detail.backchannelLogoutUri ?? "",
    frontchannelLogoutUri: detail.frontchannelLogoutUri ?? "",
    frontchannelLogoutSessionRequired: detail.frontchannelLogoutSessionRequired ?? false,
    otherDomains: detail.otherDomains ?? [],
    upstreamHost: detail.upstreamHost ?? "",
    sslMode: detail.sslMode ?? "",
    sslCert: detail.sslCert ?? "",
    samlAttributes: detail.samlAttributes ?? [],
  };
}

/** Auth methods whose credential material is deliberately excluded from exports. */
const IMPORT_METHODS_REQUIRING_CLIENT_MATERIAL = new Set([
  "private_key_jwt",
  "tls_client_auth",
  "self_signed_tls_client_auth",
]);

/**
 * True when an export document selects an auth method whose JWKS or client
 * certificate is not carried by the export. Importing it would create a client
 * the token endpoint can never authenticate, so the import flow stops before
 * the API call and points the operator at the console instead.
 */
export function importRequiresClientMaterial(payload: unknown): boolean {
  const exportSection = (payload as { export?: unknown } | null | undefined)?.export;
  const method = (exportSection as { tokenEndpointAuthMethod?: unknown } | null | undefined)
    ?.tokenEndpointAuthMethod;
  return (
    typeof method === "string" &&
    IMPORT_METHODS_REQUIRING_CLIENT_MATERIAL.has(method.trim().toLowerCase())
  );
}

/** Auth methods whose secret is deliberately excluded from exports. */
const IMPORT_METHODS_REQUIRING_SECRET_ROTATION = new Set([
  "client_secret_basic",
  "client_secret_post",
]);

/**
 * True when an imported client authenticates with a secret. The import itself
 * carries no secret material, so the client stays unusable until the operator
 * rotates a secret in the console; the import flows surface that as a warning.
 */
export function importRequiresSecretRotation(payload: unknown): boolean {
  const exportSection = (payload as { export?: unknown } | null | undefined)?.export;
  const method = (exportSection as { tokenEndpointAuthMethod?: unknown } | null | undefined)
    ?.tokenEndpointAuthMethod;
  return (
    typeof method === "string" &&
    IMPORT_METHODS_REQUIRING_SECRET_ROTATION.has(method.trim().toLowerCase())
  );
}

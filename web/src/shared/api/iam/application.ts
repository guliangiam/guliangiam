import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { captureError } from "@/shared/lib/sentry";
import {
  APPLICATIONS,
  APPLICATIONS_RESOLVE,
  APPLICATION_BY_CLIENT_ID,
  APPLICATION_BY_ID,
  APPLICATION_CLIENT_CERTIFICATES,
  APPLICATION_CLIENT_SECRET_ROTATE,
  APPLICATION_DELETE_SUMMARY,
  APPLICATION_EXPORT,
  APPLICATION_IMPORT,
  APPLICATION_NAMES,
  APPLICATION_PROVIDER_BINDINGS,
  APPLICATION_USER_APPLICATION,
} from "@/shared/api/paths";
import { pageMetaSchema } from "./schemas";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type ApplicationOption = {
  id: string;
  slug: string;
  displayName: string;
  homepageUrl?: string;
};

export type ApplicationTokenEndpointAuthMethod =
  | "none"
  | "client_secret_basic"
  | "client_secret_post"
  | "private_key_jwt"
  | "tls_client_auth"
  | "self_signed_tls_client_auth";

export type ApplicationCustomScope = {
  scope: string;
  displayName: string;
  description: string;
};

export type ApplicationClientJwkPublic = {
  kty: string;
  kid: string;
  use?: string;
  alg?: string;
  n?: string;
  e?: string;
  crv?: string;
  x?: string;
  y?: string;
};

/** Read-only public projection returned as `clientJwks`; never carries private members. */
export type ApplicationClientJwks = {
  kids: string[];
  keys: ApplicationClientJwkPublic[];
};

/** Read-only metadata of one registered mTLS client certificate. */
export type ApplicationClientCertificate = {
  fingerprintSha256: string;
  notAfter: string | null;
  selfSigned: boolean;
  status: string;
};

export type ApplicationClientCertificateInput = {
  fingerprintSha256: string;
  certificate: string;
  selfSigned: boolean;
};

export type SigninItem = {
  name: string;
  visible: boolean;
  label?: string;
  customCss?: string;
  placeholder?: string;
  rule?: string;
  isCustom?: boolean;
};

export type SignupItem = {
  name: string;
  /** Form control kind: Input / Single Choice / Multiple Choices. */
  type?: string;
  visible: boolean;
  required: boolean;
  prompted: boolean;
  rule?: string;
  regex?: string;
  options?: string[];
  label?: string;
  customCss?: string;
  placeholder?: string;
  isCustom?: boolean;
};

export type ProviderItem = {
  provider: string;
  canSignUp: boolean;
  canSignIn: boolean;
  canUnlink: boolean;
  alertType?: string;
  prompt?: string;
  bindingRule?: string[];
  countryCodes?: string[];
  prompted?: boolean;
  signupGroup?: string;
  rule?: string;
};

export type ApplicationDetail = {
  id: string;
  organizationId: string;
  slug: string;
  name: string;
  owner: string;
  displayName: string;
  category: string;
  appType: string;
  description: string;
  homepageUrl: string;
  clientId: string;
  tokenEndpointAuthMethod: ApplicationTokenEndpointAuthMethod;
  /** Read-only projection: whether an active client secret row exists. */
  clientSecretSet: boolean;
  /** Read-only masked prefix; the plaintext is never returned by GET. */
  clientSecretMasked: string;
  /** Read-only public projection of the registered client JWKS. */
  clientJwks: ApplicationClientJwks;
  /** Remote JWKS location for private_key_jwt; absent when only inline JWKS is registered. */
  jwksUri?: string;
  redirectUris: string[];
  grantTypes: string[];
  domain: string;
  status: string;
  customScopes: ApplicationCustomScope[];
  providerIds: string[];
  logo: string;
  enablePassword: boolean;
  enablePasswordLogin: boolean;
  enableCodeSignin: boolean;
  enableSamlCompress: boolean;
  signupItems: SignupItem[];
  tokenFormat: string;
  expireInHours: number;
  refreshTokenExpireInHours: number;
  tokenSigningMethod: string;
  providerItems: ProviderItem[];
  samlReplyUrl: string;
  certificate: string;
  customCss: string;
  customHeader: string;
  signinUrl: string;
  logoutUrl: string;
  logoutRedirectUrl: string;
  isPortal: boolean;
  isVisible: boolean;
  promptUrl: string;
  samlMetadata: string;
  isShared: boolean;
  title: string;
  favicon: string;
  menuOrder: number;
  menuMode: string;
  enableSignup: boolean;
  enableGuestSignin: boolean;
  enableExclusiveSignin: boolean;
  enableAutoSignin: boolean;
  enableLinkWithEmail: boolean;
  enableWebAuthn: boolean;
  enableDingtalk: boolean;
  enableWechat: boolean;
  enableLark: boolean;
  enableCas: boolean;
  cookieExpireInHours: number;
  signupUrl: string;
  forgetUrl: string;
  affiliationUrl: string;
  tokenFields: string[];
  forcedRedirectOrigin: string;
  termsOfUse: string;
  privacyPolicy: string;
  consentMode: string;
  dpopRequired: boolean;
  pkceRequired: boolean;
  defaultGroup: string;
  orgChoiceMode: string;
  enableSamlC14n10: boolean;
  enableSamlPostBinding: boolean;
  disableSamlAttributes: boolean;
  enableSamlAssertionSignature: boolean;
  useEmailAsSamlNameId: boolean;
  codeExpire: number;
  signinItems: SigninItem[];
  formCss: string;
  formCssMobile: string;
  formSideHtml: string;
  formBackgroundUrl: string;
  formBackgroundUrlMobile: string;
  formOffset: string;
  formPosition: string;
  headerHtml: string;
  footerHtml: string;
  signupHtml: string;
  signinHtml: string;
  ipWhitelist: string;
  ipBlacklist: string;
  failedSigninLimit: number;
  failedSigninFrozenTime: number;
  codeResendTimeout: number;
  corsAllowedOrigins: string;
  enableBetaOidc: boolean;
  backchannelLogoutUri: string;
  frontchannelLogoutUri: string;
  frontchannelLogoutSessionRequired: boolean;
  otherDomains: string[];
  upstreamHost: string;
  sslMode: string;
  sslCert: string;
  clientCert: string;
  samlNameIdFormat: string;
  samlNameIdContent: string;
  samlSignRequest: boolean;
  samlMetadataUrl: string;
  samlAttributes: Array<{ name: string; nameFormat?: string; value: string }>;
  samlHashAlgorithm: string;
  samlC14nPrefix: string;
  pageHtml: string;
  tokenAttributes: Array<{ name: string; sourceField: string; transform?: string }>;
  signinMethods: string[];
  disableSignin: boolean;
  enableSigninSession: boolean;
  tags: string[];
  createdAt: string;
  updatedAt: string;
  revision: number;
};

/**
 * Minimal public discovery projection served by
 * GET /api/v1/applications/by-client-id/{clientId}.
 *
 * The backend deliberately omits SAML material, callback allowlists, transport
 * certificates and token policy. Do not add management-only fields here.
 */
export type ApplicationPublicDetail = {
  id: string;
  organizationId: string;
  slug: string;
  name: string;
  displayName: string;
  description: string;
  homepageUrl: string;
  clientId: string;
  logo: string;
  title: string;
  favicon: string;
  isShared: boolean;
  enableAutoSignin: boolean;
  enableSignup: boolean;
  forcedRedirectOrigin: string;
  signupUrl: string;
  forgetUrl: string;
  termsOfUse: string;
  privacyPolicy: string;
  orgChoiceMode: string;
  signinHtml: string;
  signupHtml: string;
  headerHtml: string;
  footerHtml: string;
  customCss: string;
  customHeader: string;
  formCss: string;
  formCssMobile: string;
  formSideHtml: string;
  formBackgroundUrl: string;
  formBackgroundUrlMobile: string;
  formOffset: string;
  signinMethods: string[];
  signinItems: SigninItem[];
  signupItems: SignupItem[];
};

export type ApplicationDeleteSummary = {
  applicationId: string;
  slug: string;
  displayName: string;
  providerBindingCount: number;
  activeTokenCount: number;
  activeConsentCount: number;
};

export type CreateApplicationPayload = {
  organizationId: string;
  slug: string;
  displayName: string;
  category?: string;
  appType?: string;
  description?: string;
  homepageUrl?: string;
  domain?: string;
  status: string;
  clientId?: string;
  clientSecret?: string;
  tokenEndpointAuthMethod?: ApplicationTokenEndpointAuthMethod;
  /** Public JWKS object; required when tokenEndpointAuthMethod is private_key_jwt. */
  jwks?: Record<string, unknown>;
  /** Remote JWKS URL (HTTPS); alternative to inline jwks for private_key_jwt. */
  jwksUri?: string;
  /** Initial mTLS certificate set; required when tokenEndpointAuthMethod is an mTLS method. */
  clientCertificates?: ApplicationClientCertificateInput[];
  redirectUris?: string[];
  grantTypes?: string[];
  customScopes?: ApplicationCustomScope[];
};

export type CreateApplicationResult = {
  id: string;
  status: string;
  /** One-time plaintext secret; only present when the server generated one. */
  clientSecretPlain?: string;
};

export type ApplicationClientSecretRotation = {
  clientSecretPlain: string;
  clientSecretPrefix: string;
  rotatedAt: string;
  graceUntil: string;
};

export type UpdateApplicationPayload = {
  revision: number;
} & Record<string, unknown>;

/** 写操作返回的乐观锁结果；revision 为落库后的新值。 */
export type ApplicationMutationResult = {
  status: string;
  revision: number;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const applicationListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  slug: z.string(),
  name: z.string(),
  owner: z.string().default(""),
  displayName: z.string(),
  category: z.string(),
  appType: z.string(),
  domain: z.string(),
  status: z.string(),
  homepageUrl: z.string().default(""),
  logo: z.string().default(""),
  isVisible: z.boolean().default(true),
  certificate: z.string().default(""),
  clientId: z.string().default(""),
  redirectUris: z.array(z.string()).catch([]),
  providerIds: z.array(z.string()).catch([]),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const applicationListResponseSchema = z.object({
  items: z.array(applicationListItemSchema),
  page: pageMetaSchema,
});

const applicationClientJwkPublicSchema = z.object({
  kty: z.string(),
  kid: z.string(),
  use: z.string().optional(),
  alg: z.string().optional(),
  n: z.string().optional(),
  e: z.string().optional(),
  crv: z.string().optional(),
  x: z.string().optional(),
  y: z.string().optional(),
});

const applicationClientJwksSchema = z.object({
  kids: z.array(z.string()).catch([]),
  keys: z.array(applicationClientJwkPublicSchema).catch([]),
});

const applicationClientCertificateSchema = z.object({
  fingerprintSha256: z.string(),
  notAfter: z.string().nullable().default(null),
  selfSigned: z.boolean().default(false),
  status: z.string().default("active"),
});

const applicationClientCertificatesSchema = z.object({
  items: z.array(applicationClientCertificateSchema).catch([]),
});

const applicationSignupItemSchema = z.object({
  name: z.string(),
  visible: z.boolean().default(true),
  required: z.boolean().default(false),
  prompted: z.boolean().default(false),
  rule: z.string().optional(),
  regex: z.string().optional(),
  options: z.array(z.string()).optional(),
  type: z.string().optional(),
  label: z.string().optional(),
  customCss: z.string().optional(),
  placeholder: z.string().optional(),
  isCustom: z.boolean().optional(),
});

const applicationSigninItemSchema = z.object({
  name: z.string(),
  visible: z.boolean().default(true),
  label: z.string().optional(),
  customCss: z.string().optional(),
  placeholder: z.string().optional(),
  rule: z.string().optional(),
  isCustom: z.boolean().optional(),
});

export const applicationDetailSchema = applicationListItemSchema.extend({
  description: z.string(),
  homepageUrl: z.string(),
  clientId: z.string(),
  tokenEndpointAuthMethod: z.enum([
    "none",
    "client_secret_basic",
    "client_secret_post",
    "private_key_jwt",
    "tls_client_auth",
    "self_signed_tls_client_auth",
  ]),
  clientSecretSet: z.boolean().default(false),
  clientSecretMasked: z.string().default(""),
  clientJwks: applicationClientJwksSchema.default({ kids: [], keys: [] }),
  jwksUri: z.string().optional(),
  redirectUris: z.array(z.string()).catch([]),
  grantTypes: z.array(z.string()).catch([]),
  customScopes: z
    .array(
      z.object({
        scope: z.string(),
        displayName: z.string().default(""),
        description: z.string().default(""),
      }),
    )
    .catch([]),
  logo: z.string().default(""),
  enablePassword: z.boolean().default(true),
  enablePasswordLogin: z.boolean().default(true),
  enableCodeSignin: z.boolean().default(false),
  enableSamlCompress: z.boolean().default(false),
  signupItems: z.array(applicationSignupItemSchema).catch([]),
  tokenFormat: z.string().default("JWT"),
  expireInHours: z.number().default(24),
  refreshTokenExpireInHours: z.number().default(168),
  tokenSigningMethod: z.string().default("RS256"),
  providerItems: z
    .array(
      z.object({
        provider: z.string(),
        canSignUp: z.boolean().default(false),
        canSignIn: z.boolean().default(false),
        canUnlink: z.boolean().default(false),
        alertType: z.string().optional(),
        prompt: z.string().optional(),
        bindingRule: z.array(z.string()).optional(),
        countryCodes: z.array(z.string()).optional(),
        prompted: z.boolean().optional(),
        signupGroup: z.string().optional(),
        rule: z.string().optional(),
      }),
    )
    .catch([]),
  samlReplyUrl: z.string().default(""),
  certificate: z.string().default(""),
  customCss: z.string().default(""),
  customHeader: z.string().default(""),
  signinUrl: z.string().default(""),
  logoutUrl: z.string().default(""),
  logoutRedirectUrl: z.string().default(""),
  isPortal: z.boolean().default(false),
  isVisible: z.boolean().default(true),
  promptUrl: z.string().default(""),
  samlMetadata: z.string().default(""),
  isShared: z.boolean().default(false),
  title: z.string().default(""),
  favicon: z.string().default(""),
  menuOrder: z.number().default(0),
  menuMode: z.string().default(""),
  enableSignup: z.boolean().default(true),
  enableGuestSignin: z.boolean().default(false),
  enableExclusiveSignin: z.boolean().default(false),
  enableAutoSignin: z.boolean().default(true),
  enableLinkWithEmail: z.boolean().default(false),
  cookieExpireInHours: z.number().default(168),
  signupUrl: z.string().default(""),
  forgetUrl: z.string().default(""),
  affiliationUrl: z.string().default(""),
  tokenFields: z.array(z.string()).catch([]),
  forcedRedirectOrigin: z.string().default(""),
  samlNameIdFormat: z.string().default(""),
  samlNameIdContent: z.string().default(""),
  samlSignRequest: z.boolean().default(false),
  samlMetadataUrl: z.string().default(""),
  termsOfUse: z.string().default(""),
  privacyPolicy: z.string().default(""),
  consentMode: z.string().default("explicit"),
  dpopRequired: z.boolean().default(false),
  pkceRequired: z.boolean().default(false),
  defaultGroup: z.string().default(""),
  orgChoiceMode: z.string().default(""),
  enableWebAuthn: z.boolean().default(false),
  enableDingtalk: z.boolean().default(false),
  enableWechat: z.boolean().default(false),
  enableLark: z.boolean().default(false),
  enableCas: z.boolean().default(false),
  enableSamlC14n10: z.boolean().default(false),
  enableSamlPostBinding: z.boolean().default(false),
  disableSamlAttributes: z.boolean().default(false),
  enableSamlAssertionSignature: z.boolean().default(false),
  useEmailAsSamlNameId: z.boolean().default(false),
  codeExpire: z.number().default(5),
  signinItems: z.array(applicationSigninItemSchema).catch([]),
  formCss: z.string().default(""),
  formCssMobile: z.string().default(""),
  formSideHtml: z.string().default(""),
  formBackgroundUrl: z.string().default(""),
  formBackgroundUrlMobile: z.string().default(""),
  formOffset: z.string().default("0"),
  formPosition: z.string().default(""),
  headerHtml: z.string().default(""),
  footerHtml: z.string().default(""),
  signupHtml: z.string().default(""),
  signinHtml: z.string().default(""),
  ipWhitelist: z.string().default(""),
  ipBlacklist: z.string().default(""),
  failedSigninLimit: z.number().default(5),
  failedSigninFrozenTime: z.number().default(15),
  codeResendTimeout: z.number().default(60),
  corsAllowedOrigins: z.string().default(""),
  enableBetaOidc: z.boolean().default(false),
  backchannelLogoutUri: z.string().default(""),
  frontchannelLogoutUri: z.string().default(""),
  frontchannelLogoutSessionRequired: z.boolean().default(false),
  otherDomains: z.array(z.string()).catch([]),
  upstreamHost: z.string().default(""),
  sslMode: z.string().default(""),
  sslCert: z.string().default(""),
  clientCert: z.string().default(""),
  samlAttributes: z
    .array(z.object({ name: z.string(), nameFormat: z.string().optional(), value: z.string() }))
    .catch([]),
  samlHashAlgorithm: z.string().default(""),
  samlC14nPrefix: z.string().default(""),
  pageHtml: z.string().default(""),
  tokenAttributes: z
    .array(
      z.object({ name: z.string(), sourceField: z.string(), transform: z.string().optional() }),
    )
    .catch([]),
  signinMethods: z.array(z.string()).catch([]),
  disableSignin: z.boolean().default(false),
  enableSigninSession: z.boolean().default(false),
  tags: z.array(z.string()).catch([]),
  revision: z.number(),
});

const applicationPublicDetailSchema = z.object({
  id: z.string(),
  organizationId: z.string().default(""),
  slug: z.string(),
  name: z.string(),
  displayName: z.string().default(""),
  description: z.string().default(""),
  homepageUrl: z.string().default(""),
  clientId: z.string().default(""),
  logo: z.string().default(""),
  title: z.string().default(""),
  favicon: z.string().default(""),
  isShared: z.boolean().default(false),
  enableAutoSignin: z.boolean().default(false),
  enableSignup: z.boolean().default(false),
  forcedRedirectOrigin: z.string().default(""),
  signupUrl: z.string().default(""),
  forgetUrl: z.string().default(""),
  termsOfUse: z.string().default(""),
  privacyPolicy: z.string().default(""),
  orgChoiceMode: z.string().default(""),
  signinHtml: z.string().default(""),
  signupHtml: z.string().default(""),
  headerHtml: z.string().default(""),
  footerHtml: z.string().default(""),
  customCss: z.string().default(""),
  customHeader: z.string().default(""),
  formCss: z.string().default(""),
  formCssMobile: z.string().default(""),
  formSideHtml: z.string().default(""),
  formBackgroundUrl: z.string().default(""),
  formBackgroundUrlMobile: z.string().default(""),
  formOffset: z.string().default(""),
  signinMethods: z.array(z.string()).catch([]),
  signinItems: z.array(applicationSigninItemSchema).catch([]),
  signupItems: z.array(applicationSignupItemSchema).catch([]),
});

const applicationDeleteSummarySchema = z.object({
  applicationId: z.string(),
  slug: z.string().default(""),
  displayName: z.string().default(""),
  providerBindingCount: z.number().int().nonnegative(),
  activeTokenCount: z.number().int().nonnegative(),
  activeConsentCount: z.number().int().nonnegative(),
});

const createApplicationResultSchema = z.object({
  id: z.string(),
  status: z.string(),
  clientSecretPlain: z.string().optional(),
});

const applicationClientSecretRotationSchema = z.object({
  clientSecretPlain: z.string(),
  clientSecretPrefix: z.string().default(""),
  rotatedAt: z.string(),
  graceUntil: z.string(),
});

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

function toApplicationItem(item: z.infer<typeof applicationListItemSchema>) {
  return {
    ...item,
    name: item.slug,
    providers: item.providerIds,
  };
}

function toApplicationDetail(item: z.infer<typeof applicationDetailSchema>): ApplicationDetail {
  return {
    id: item.id,
    organizationId: item.organizationId,
    slug: item.slug,
    name: item.name,
    owner: item.owner,
    displayName: item.displayName,
    category: item.category,
    appType: item.appType,
    description: item.description,
    homepageUrl: item.homepageUrl,
    clientId: item.clientId,
    tokenEndpointAuthMethod: item.tokenEndpointAuthMethod,
    clientSecretSet: item.clientSecretSet,
    clientSecretMasked: item.clientSecretMasked,
    clientJwks: item.clientJwks,
    jwksUri: item.jwksUri,
    redirectUris: item.redirectUris,
    grantTypes: item.grantTypes,
    domain: item.domain,
    status: item.status,
    customScopes: item.customScopes,
    providerIds: item.providerIds,
    logo: item.logo,
    enablePassword: item.enablePassword,
    enablePasswordLogin: item.enablePasswordLogin,
    enableCodeSignin: item.enableCodeSignin,
    enableSamlCompress: item.enableSamlCompress,
    signupItems: item.signupItems,
    tokenFormat: item.tokenFormat,
    expireInHours: item.expireInHours,
    refreshTokenExpireInHours: item.refreshTokenExpireInHours,
    tokenSigningMethod: item.tokenSigningMethod,
    providerItems: item.providerItems,
    samlReplyUrl: item.samlReplyUrl,
    certificate: item.certificate,
    customCss: item.customCss,
    customHeader: item.customHeader,
    signinUrl: item.signinUrl,
    logoutUrl: item.logoutUrl,
    logoutRedirectUrl: item.logoutRedirectUrl,
    isPortal: item.isPortal,
    isVisible: item.isVisible,
    promptUrl: item.promptUrl,
    samlMetadata: item.samlMetadata,
    isShared: item.isShared,
    title: item.title,
    favicon: item.favicon,
    menuOrder: item.menuOrder,
    menuMode: item.menuMode,
    enableSignup: item.enableSignup,
    enableGuestSignin: item.enableGuestSignin,
    enableExclusiveSignin: item.enableExclusiveSignin,
    enableAutoSignin: item.enableAutoSignin,
    enableLinkWithEmail: item.enableLinkWithEmail,
    cookieExpireInHours: item.cookieExpireInHours,
    signupUrl: item.signupUrl,
    forgetUrl: item.forgetUrl,
    affiliationUrl: item.affiliationUrl,
    tokenFields: item.tokenFields,
    forcedRedirectOrigin: item.forcedRedirectOrigin,
    samlNameIdFormat: item.samlNameIdFormat,
    samlNameIdContent: item.samlNameIdContent,
    samlSignRequest: item.samlSignRequest,
    samlMetadataUrl: item.samlMetadataUrl,
    termsOfUse: item.termsOfUse,
    privacyPolicy: item.privacyPolicy,
    consentMode: item.consentMode,
    dpopRequired: item.dpopRequired,
    pkceRequired: item.pkceRequired,
    defaultGroup: item.defaultGroup,
    orgChoiceMode: item.orgChoiceMode,
    enableWebAuthn: item.enableWebAuthn,
    enableDingtalk: item.enableDingtalk,
    enableWechat: item.enableWechat,
    enableLark: item.enableLark,
    enableCas: item.enableCas,
    enableSamlC14n10: item.enableSamlC14n10,
    enableSamlPostBinding: item.enableSamlPostBinding,
    disableSamlAttributes: item.disableSamlAttributes,
    enableSamlAssertionSignature: item.enableSamlAssertionSignature,
    useEmailAsSamlNameId: item.useEmailAsSamlNameId,
    codeExpire: item.codeExpire,
    signinItems: item.signinItems,
    formCss: item.formCss,
    formCssMobile: item.formCssMobile,
    formSideHtml: item.formSideHtml,
    formBackgroundUrl: item.formBackgroundUrl,
    formBackgroundUrlMobile: item.formBackgroundUrlMobile,
    formOffset: item.formOffset,
    formPosition: item.formPosition,
    headerHtml: item.headerHtml,
    footerHtml: item.footerHtml,
    signupHtml: item.signupHtml,
    signinHtml: item.signinHtml,
    disableSignin: item.disableSignin,
    enableSigninSession: item.enableSigninSession,
    tags: item.tags,
    ipWhitelist: item.ipWhitelist,
    ipBlacklist: item.ipBlacklist,
    failedSigninLimit: item.failedSigninLimit,
    failedSigninFrozenTime: item.failedSigninFrozenTime,
    codeResendTimeout: item.codeResendTimeout,
    corsAllowedOrigins: item.corsAllowedOrigins,
    enableBetaOidc: item.enableBetaOidc,
    backchannelLogoutUri: item.backchannelLogoutUri,
    frontchannelLogoutUri: item.frontchannelLogoutUri,
    frontchannelLogoutSessionRequired: item.frontchannelLogoutSessionRequired,
    otherDomains: item.otherDomains,
    upstreamHost: item.upstreamHost,
    sslMode: item.sslMode,
    sslCert: item.sslCert,
    clientCert: item.clientCert,
    samlAttributes: item.samlAttributes,
    samlHashAlgorithm: item.samlHashAlgorithm,
    samlC14nPrefix: item.samlC14nPrefix,
    pageHtml: item.pageHtml,
    tokenAttributes: item.tokenAttributes,
    signinMethods: item.signinMethods,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    revision: item.revision,
  };
}

function toApplicationOption(item: z.infer<typeof applicationListItemSchema>): ApplicationOption {
  return {
    id: item.id,
    slug: item.slug,
    displayName: item.displayName,
    homepageUrl: item.homepageUrl,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

/**
 * 详情级端点在服务端强制可信租户：平台管理员作用域的会话没有固定组织，
 * 必须把目标资源所属组织显式作为 query 传入，否则请求会落到会话组织导致
 * 404（新建跨组织应用后「配置加载失败」的根因）。租户用户可不传。
 */
function withOrgQuery(base: string, organizationId?: string): string {
  if (!organizationId) return base;
  const separator = base.includes("?") ? "&" : "?";
  return `${base}${separator}organizationId=${encodeURIComponent(organizationId)}`;
}

export async function getApplications(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }

  const response = await requestDocumentJson(`${APPLICATIONS}?${query.toString()}`);
  return applicationListResponseSchema.parse(response).items.map(toApplicationItem);
}

export type ApplicationListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  category?: string;
  status?: string;
};

export async function getApplicationsPaginated(params?: ApplicationListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.category) query.set("category", params.category);
  if (params?.status) query.set("status", params.status);

  const response = await requestDocumentJson(`${APPLICATIONS}?${query.toString()}`);
  const parsed = applicationListResponseSchema.parse(response);
  return {
    items: parsed.items.map(toApplicationItem),
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}
export async function getApplicationOptions(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }

  const response = await requestDocumentJson(`${APPLICATIONS}?${query.toString()}`);
  return applicationListResponseSchema.parse(response).items.map(toApplicationOption);
}

export async function getApplication(
  id: string,
  organizationId?: string,
): Promise<ApplicationDetail> {
  // 详情 GET 在服务端带 `private, max-age=30`：保存后 Query 失效重取会命中
  // 浏览器缓存里的旧 revision/旧配置，编辑器必须绕开 HTTP 缓存。
  const response = await requestDocumentJson(withOrgQuery(APPLICATION_BY_ID(id), organizationId), {
    cache: "no-store",
  });
  return toApplicationDetail(applicationDetailSchema.parse(response));
}

export async function getApplicationDeleteSummary(
  id: string,
  organizationId?: string,
): Promise<ApplicationDeleteSummary> {
  const response = await requestDocumentJson(
    withOrgQuery(APPLICATION_DELETE_SUMMARY(id), organizationId),
    { cache: "no-store" },
  );
  return applicationDeleteSummarySchema.parse(response);
}

export async function createApplication(
  payload: CreateApplicationPayload,
): Promise<CreateApplicationResult> {
  const response = await requestDocumentJson(APPLICATIONS, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
  return createApplicationResultSchema.parse(response);
}

export async function updateApplication(
  id: string,
  payload: UpdateApplicationPayload,
  organizationId?: string,
): Promise<ApplicationMutationResult> {
  const response = (await requestDocumentJson(withOrgQuery(APPLICATION_BY_ID(id), organizationId), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  })) as { status?: unknown; revision?: unknown };
  return {
    status: typeof response?.status === "string" ? response.status : "updated",
    revision: typeof response?.revision === "number" ? response.revision : 0,
  };
}

/**
 * Rotate the client secret of an application that authenticates with
 * client_secret_basic or client_secret_post. The plaintext is returned only in
 * this response; callers must show it once and never persist it.
 */
export async function rotateApplicationClientSecret(
  id: string,
  organizationId?: string,
): Promise<ApplicationClientSecretRotation> {
  const response = await requestDocumentJson(
    withOrgQuery(APPLICATION_CLIENT_SECRET_ROTATE(id), organizationId),
    {
      method: "POST",
    },
  );
  return applicationClientSecretRotationSchema.parse(response);
}

export async function deleteApplication(id: string, organizationId?: string) {
  await requestDocumentJson(withOrgQuery(APPLICATION_BY_ID(id), organizationId), {
    method: "DELETE",
  });
}

export async function updateApplicationProviderBindings(
  id: string,
  providerIds: string[],
  organizationId?: string,
): Promise<ApplicationMutationResult> {
  const response = (await requestDocumentJson(
    withOrgQuery(APPLICATION_PROVIDER_BINDINGS(id), organizationId),
    {
      method: "PUT",
      body: JSON.stringify({
        items: providerIds.map((providerId) => ({ providerId })),
      }),
      headers: {
        "Content-Type": "application/json",
      },
    },
  )) as { status?: unknown; revision?: unknown };
  return {
    status: typeof response?.status === "string" ? response.status : "updated",
    revision: typeof response?.revision === "number" ? response.revision : 0,
  };
}

/** Get the minimal public discovery projection for an OAuth client ID. */
export async function getApplicationByClientId(clientId: string): Promise<ApplicationPublicDetail> {
  const response = await requestDocumentJson(APPLICATION_BY_CLIENT_ID(clientId));
  return applicationPublicDetailSchema.parse(response);
}

/** Resolve an application by owner slug and application slug.
 * Anonymous discovery: the backend serves the same public projection as
 * by-client-id, never the management detail. */
export async function resolveApplication(
  ownerSlug: string,
  appSlug: string,
): Promise<ApplicationPublicDetail | null> {
  const query = new URLSearchParams({ ownerSlug, appSlug });
  try {
    const response = await requestDocumentJson(`${APPLICATIONS_RESOLVE}?${query.toString()}`);
    return applicationPublicDetailSchema.parse(response);
  } catch (error) {
    captureError(error, { ownerSlug, appSlug });
    return null;
  }
}

/** Get the current provider bindings for an application. */
export async function getApplicationProviderBindings(
  id: string,
  organizationId?: string,
): Promise<string[]> {
  const response = await requestDocumentJson(
    withOrgQuery(APPLICATION_PROVIDER_BINDINGS(id), organizationId),
    { cache: "no-store" },
  );
  const data = response as Record<string, unknown>;
  const items = (data.items ?? data.data ?? []) as Array<{ providerId: string }>;
  return items.map((item) => item.providerId);
}

/** List registered mTLS client certificates (fingerprints only, never the PEM). */
export async function getApplicationClientCertificates(
  id: string,
  organizationId?: string,
): Promise<ApplicationClientCertificate[]> {
  const response = await requestDocumentJson(
    withOrgQuery(APPLICATION_CLIENT_CERTIFICATES(id), organizationId),
    { cache: "no-store" },
  );
  return applicationClientCertificatesSchema.parse(response).items;
}

/** Replace the whole set of mTLS client certificates. */
export async function replaceApplicationClientCertificates(
  id: string,
  items: ApplicationClientCertificateInput[],
  organizationId?: string,
): Promise<void> {
  await requestDocumentJson(withOrgQuery(APPLICATION_CLIENT_CERTIFICATES(id), organizationId), {
    method: "PUT",
    body: JSON.stringify({ items }),
  });
}

/** Export application configuration as JSON. */
export async function exportApplicationConfig(id: string, organizationId?: string): Promise<Blob> {
  const response = await requestDocumentJson(withOrgQuery(APPLICATION_EXPORT(id), organizationId), {
    headers: { Accept: "application/json" },
  });
  const json = JSON.stringify(response);
  return new Blob([json], { type: "application/json" });
}

/** Import application configuration from JSON. */
export async function importApplicationConfig(
  organizationId: string,
  payload: Record<string, unknown>,
): Promise<{ id: string; status: string }> {
  const response = await requestDocumentJson(APPLICATION_IMPORT, {
    method: "POST",
    body: JSON.stringify({ organizationId, ...payload }),
    headers: { "Content-Type": "application/json" },
  });
  const result = z.object({ id: z.string(), status: z.string() }).parse(response);
  return { id: result.id, status: result.status };
}

/** Get the default application for the current user's organization. */
export async function getUserApplication(): Promise<ApplicationDetail> {
  const response = await requestDocumentJson(APPLICATION_USER_APPLICATION);
  return toApplicationDetail(applicationDetailSchema.parse(response));
}

/** Get application names (lightweight listing for selects/dropdowns). */
export async function getApplicationNames(): Promise<ApplicationOption[]> {
  const response = await requestDocumentJson(APPLICATION_NAMES);
  const items = (response as Array<{ id: string; slug: string; displayName: string }>).map(
    (item) => ({ id: item.id, slug: item.slug, displayName: item.displayName }),
  );
  return items;
}

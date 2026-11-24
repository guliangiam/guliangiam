import { consoleGrantTypes } from "./grant-types.generated";
import type {
  ApplicationCustomScope,
  ApplicationTokenEndpointAuthMethod,
  SignupItem,
  ProviderItem,
} from "@/shared/api/iam/application";

export type {
  SignupItem,
  ProviderItem,
  ApplicationCustomScope,
  ApplicationTokenEndpointAuthMethod,
};

/**
 * Provider list row narrowed to the fields the application pages consume.
 * `id` is used as the binding value and `name` as the option label; every
 * other field of the providers list response is intentionally out of scope.
 */
export type ProviderRef = { id: string; name: string };

/**
 * Application list row narrowed to the fields the slug validator consumes:
 * `id` to exclude the record being edited and `name` (the slug) to detect
 * duplicates.
 */
export type ApplicationSlugRef = { id: string; name: string };

export type ApplicationConfigFormValues = {
  revision?: number;
  organizationId?: string;
  slug?: string;
  displayName: string;
  category?: string;
  appType?: string;
  description?: string;
  homepageUrl?: string;
  domain?: string;
  status: string;
  clientId?: string;
  clientSecret?: string;
  tokenEndpointAuthMethod: ApplicationTokenEndpointAuthMethod;
  /** Raw JWKS JSON pasted in the private_key_jwt editor; parsed on save. */
  jwks?: string;
  /** Remote JWKS URL (HTTPS) for private_key_jwt; alternative to inline jwks. Empty clears the stored URI on save. */
  jwksUri?: string;
  /**
   * mTLS 证书条目：调用方粘贴公钥证书 PEM，指纹在提交前由浏览器计算，
   * 服务端会重算比对。仅指纹无法登记（服务端拒绝只报指纹的请求）。
   */
  mtlsCertificates?: Array<{ certificate?: string }>;
  redirectUris?: string[];
  grantTypes?: string[];
  customScopes?: Array<{
    scope?: string;
    displayName?: string;
    description?: string;
  }>;
  logo?: string;
  isShared?: boolean;
  title?: string;
  favicon?: string;
  menuOrder?: number;
  menuMode?: string;
  enableSamlCompress?: boolean;
  enableSignup?: boolean;
  enableGuestSignin?: boolean;
  enableExclusiveSignin?: boolean;
  enableAutoSignin?: boolean;
  enableLinkWithEmail?: boolean;
  disableSignin?: boolean;
  enableSigninSession?: boolean;
  enableSamlC14n10?: boolean;
  enableSamlPostBinding?: boolean;
  disableSamlAttributes?: boolean;
  enableSamlAssertionSignature?: boolean;
  useEmailAsSamlNameId?: boolean;
  cookieExpireInHours?: number;
  tags?: string[];
  signupItems?: SignupItem[];
  signinMethods?: string[];
  signinItems?: Array<{
    name: string;
    visible: boolean;
    label?: string;
    customCss?: string;
    placeholder?: string;
    rule?: string;
    isCustom?: boolean;
  }>;
  signupUrl?: string;
  forgetUrl?: string;
  affiliationUrl?: string;
  tokenFormat?: string;
  tokenFields?: string[];
  tokenAttributes?: Array<{ name: string; sourceField: string; transform?: string }>;
  expireInHours?: number;
  refreshTokenExpireInHours?: number;
  tokenSigningMethod?: string;
  codeExpire?: number;
  forcedRedirectOrigin?: string;
  providerItems?: ProviderItem[];
  /** Bound provider ids: separate API/model, submitted in the unified save. */
  providerIds?: string[];
  samlReplyUrl?: string;
  certificate?: string;
  samlNameIdFormat?: string;
  samlNameIdContent?: string;
  samlSignRequest?: boolean;
  samlMetadataUrl?: string;
  /**
   * Which SAML metadata source the operator explicitly selected. `"keep"`
   * (default) submits both sides untouched; `"url"` / `"xml"` is only set by an
   * explicit switch and clears the opposite side at submit time.
   */
  samlMetadataSource?: "keep" | "url" | "xml";
  customCss?: string;
  customHeader?: string;
  formCss?: string;
  formCssMobile?: string;
  formSideHtml?: string;
  formBackgroundUrl?: string;
  formBackgroundUrlMobile?: string;
  formOffset?: string;
  formPosition?: string;
  headerHtml?: string;
  footerHtml?: string;
  signupHtml?: string;
  signinHtml?: string;
  signinUrl?: string;
  logoutUrl?: string;
  logoutRedirectUrl?: string;
  isPortal?: boolean;
  isVisible?: boolean;
  promptUrl?: string;
  samlMetadata?: string;
  samlAttributes?: Array<{ name: string; nameFormat?: string; value: string }>;
  samlHashAlgorithm?: string;
  samlC14nPrefix?: string;
  termsOfUse?: string;
  privacyPolicy?: string;
  consentMode?: string;
  dpopRequired?: boolean;
  pkceRequired?: boolean;
  defaultGroup?: string;
  orgChoiceMode?: string;
  ipWhitelist?: string;
  ipBlacklist?: string;
  failedSigninLimit?: number;
  failedSigninFrozenTime?: number;
  codeResendTimeout?: number;
  corsAllowedOrigins?: string;
  enableBetaOidc?: boolean;
  backchannelLogoutUri?: string;
  frontchannelLogoutUri?: string;
  frontchannelLogoutSessionRequired?: boolean;
  otherDomains?: string[];
  upstreamHost?: string;
  sslMode?: string;
  sslCert?: string;
};

export const grantTypeOptions = consoleGrantTypes.map((value) => ({
  label: value,
  value,
}));

/**
 * The two token endpoint methods that authenticate with a shared secret.
 * Credential rotation only applies to these; JWT and mTLS clients hold their
 * material elsewhere (app_client_jwks / app_client_certificates).
 */
export function isClientSecretAuthMethod(
  method: ApplicationTokenEndpointAuthMethod | undefined,
): method is "client_secret_basic" | "client_secret_post" {
  return method === "client_secret_basic" || method === "client_secret_post";
}

/** RFC 8705 methods: the client authenticates with a TLS certificate. */
export function isMTLSAuthMethod(
  method: ApplicationTokenEndpointAuthMethod | undefined,
): method is "tls_client_auth" | "self_signed_tls_client_auth" {
  return method === "tls_client_auth" || method === "self_signed_tls_client_auth";
}

/** i18n key suffix for the one-line security/scenario note of each method. */
export function tokenEndpointAuthMethodHintKey(method: ApplicationTokenEndpointAuthMethod) {
  switch (method) {
    case "client_secret_basic":
      return "application.tokenEndpointAuthMethodOption.clientSecretBasicHint";
    case "client_secret_post":
      return "application.tokenEndpointAuthMethodOption.clientSecretPostHint";
    case "private_key_jwt":
      return "application.tokenEndpointAuthMethodOption.privateKeyJwtHint";
    case "tls_client_auth":
      return "application.tokenEndpointAuthMethodOption.tlsClientAuthHint";
    case "self_signed_tls_client_auth":
      return "application.tokenEndpointAuthMethodOption.selfSignedTlsClientAuthHint";
    default:
      return "application.tokenEndpointAuthMethodOption.noneHint";
  }
}

/** Result of a best-effort JWKS parse used for live kid chips and validation. */
export type JwksParseResult = {
  kids: string[];
  error?: "invalidJson" | "invalidShape";
};

/**
 * Parse JWKS text without throwing. Empty input is not an error: whether a
 * JWKS is required depends on the selected auth method.
 */
export function parseJwksKids(raw?: string): JwksParseResult {
  const trimmed = raw?.trim() ?? "";
  if (trimmed === "") {
    return { kids: [] };
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return { kids: [], error: "invalidJson" };
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return { kids: [], error: "invalidShape" };
  }
  const keys = (parsed as { keys?: unknown }).keys;
  if (!Array.isArray(keys)) {
    return { kids: [], error: "invalidShape" };
  }
  const kids: string[] = [];
  for (const entry of keys) {
    if (typeof entry !== "object" || entry === null) continue;
    const kid = (entry as { kid?: unknown }).kid;
    if (typeof kid === "string" && kid.trim() !== "") {
      kids.push(kid.trim());
    }
  }
  return { kids };
}

/**
 * Build i18n-aware token endpoint auth method options.
 * Call this inside a component where useTranslation is available.
 */
export function getTokenEndpointAuthMethodOptions(tr: (key: string, fallback: string) => string) {
  return [
    {
      label: tr("application.tokenEndpointAuthMethodOption.none", "Public client (none)"),
      value: "none",
    },
    {
      label: tr(
        "application.tokenEndpointAuthMethodOption.clientSecretBasic",
        "client_secret_basic",
      ),
      value: "client_secret_basic",
    },
    {
      label: tr("application.tokenEndpointAuthMethodOption.clientSecretPost", "client_secret_post"),
      value: "client_secret_post",
    },
    {
      label: tr("application.tokenEndpointAuthMethodOption.privateKeyJwt", "private_key_jwt"),
      value: "private_key_jwt",
    },
    {
      label: tr("application.tokenEndpointAuthMethodOption.tlsClientAuth", "tls_client_auth"),
      value: "tls_client_auth",
    },
    {
      label: tr(
        "application.tokenEndpointAuthMethodOption.selfSignedTlsClientAuth",
        "self_signed_tls_client_auth",
      ),
      value: "self_signed_tls_client_auth",
    },
  ];
}

/** @deprecated Use getTokenEndpointAuthMethodOptions(tr) for i18n support */
export const tokenEndpointAuthMethodOptions = [
  { label: "Public client (none)", value: "none" },
  { label: "client_secret_basic", value: "client_secret_basic" },
  { label: "client_secret_post", value: "client_secret_post" },
  { label: "private_key_jwt", value: "private_key_jwt" },
  { label: "tls_client_auth", value: "tls_client_auth" },
  { label: "self_signed_tls_client_auth", value: "self_signed_tls_client_auth" },
];

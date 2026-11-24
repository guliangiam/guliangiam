import { useMemo } from "react";
import { useTranslation } from "react-i18next";

// ─────────────────────────────────────────────
// Provider Category Definitions
// ─────────────────────────────────────────────

type ProviderCategoryOption = { label: string; value: string };

/**
 * Backend kinds owned by dedicated consoles (LDAP / CAPTCHA / FaceID). The
 * unified `/api/v1/providers` entry point rejects create, kind/providerType
 * changes, activate and secret rotation for them (P1 422), so the unified
 * console must not guide users towards creating them.
 *
 * Single source for the UI-side closure; keep aligned with
 * `contracts/registries/provider-kinds.yaml` (`managed_via`) and
 * `internal/app/providers/delegated.go`.
 */
export const MANAGED_PROVIDER_KINDS = [
  "ldap",
  "recaptcha",
  "recaptcha_v3",
  "hcaptcha",
  "turnstile",
  "geetest",
  "aliyun_captcha",
  "tencent_captcha",
  "faceid",
  "aliyun_face_body",
] as const;

/** `reCAPTCHAv3` → `recaptchav3`, `recaptcha_v3` → `recaptchav3`. */
const managedKindKeys = new Set(MANAGED_PROVIDER_KINDS.map((kind) => kind.replace(/_/g, "")));

/**
 * Whether a UI category/providerType value resolves to a managed backend kind.
 * Separators and casing differ between the two (`reCAPTCHA`, `reCAPTCHAv3`,
 * `AliyunFaceBody`), so compare on a separator-free lowercase key.
 */
export function isManagedProviderKind(value: string): boolean {
  return managedKindKeys.has(
    value
      .trim()
      .toLowerCase()
      .replace(/[-_\s]/g, ""),
  );
}

type ProviderCategoryCatalogEntry = ProviderCategoryOption & {
  /** New providers may choose this category. */
  creatable: boolean;
  /** The list page offers this category as a filter. */
  filterable: boolean;
};

/**
 * Every category the UI knows how to render. `creatable: false` marks
 * historical-only categories: they remain readable/filterable so existing
 * rows stay visible, but they are no longer offered when creating a provider.
 */
const PROVIDER_CATEGORY_CATALOG: readonly ProviderCategoryCatalogEntry[] = [
  { label: "OAuth", value: "OAuth", creatable: true, filterable: true },
  { label: "SAML", value: "SAML", creatable: true, filterable: true },
  { label: "OIDC", value: "OIDC", creatable: true, filterable: false },
  { label: "Email", value: "Email", creatable: true, filterable: true },
  { label: "SMS", value: "SMS", creatable: true, filterable: true },
  { label: "Storage", value: "Storage", creatable: true, filterable: true },
  // Payment runtime credentials live in pay_channel_configs (sealed, versioned,
  // draft→testing→active); unified providers rows never drive the charge chain.
  // Creation routes to the channel-config console; historical rows stay filterable.
  { label: "Payment", value: "Payment", creatable: false, filterable: true },
  // Notification rows in the unified catalog are the provider source for Push
  // MFA (EnablePushMfa requires providers.category = Notification); outbound
  // notification channels live in the dedicated notification console. Keep
  // this category creatable here — redirecting it would sever Push MFA setup.
  { label: "Notification", value: "Notification", creatable: true, filterable: true },
  { label: "Captcha", value: "Captcha", creatable: true, filterable: true },
  { label: "MFA", value: "MFA", creatable: true, filterable: false },
  { label: "Face ID", value: "FaceId", creatable: false, filterable: true },
  // Wallet sign-in endpoints are gone (410); Web3 rows are historical-only.
  // Passkey (WebAuthn) is a separate sign-in channel, not this category.
  { label: "Web3", value: "Web3", creatable: false, filterable: true },
  // Scan/Log kinds are accepted by the unified endpoint but have no runtime
  // consumer (PendingKindRoutes marks them Routable:false with no caller), so
  // creating them is a dead end. Historical rows stay readable/filterable.
  { label: "Scan", value: "Scan", creatable: false, filterable: true },
  { label: "ID Verification", value: "IDVerification", creatable: true, filterable: true },
  { label: "Log", value: "Log", creatable: false, filterable: true },
  { label: "Audit", value: "Audit", creatable: true, filterable: false },
];

const toCategoryOption = ({ label, value }: ProviderCategoryOption): ProviderCategoryOption => ({
  label,
  value,
});

/** All known categories, including historical-only ones (read-only display). */
export const PROVIDER_CATEGORIES: readonly ProviderCategoryOption[] =
  PROVIDER_CATEGORY_CATALOG.map(toCategoryOption);

/** Historical-only categories; kept for list filters and row display. */
export const HISTORICAL_PROVIDER_CATEGORIES: readonly ProviderCategoryOption[] =
  PROVIDER_CATEGORY_CATALOG.filter((entry) => !entry.creatable).map(toCategoryOption);

/** Categories selectable in the create/edit form. */
export const CREATABLE_PROVIDER_CATEGORIES: readonly ProviderCategoryOption[] =
  PROVIDER_CATEGORY_CATALOG.filter((entry) => entry.creatable).map(toCategoryOption);

/** Categories offered by the list filter; includes historical-only categories. */
export const FILTERABLE_PROVIDER_CATEGORIES: readonly ProviderCategoryOption[] =
  PROVIDER_CATEGORY_CATALOG.filter((entry) => entry.filterable).map(toCategoryOption);

/** Alias used by the list page; new code should use FILTERABLE_PROVIDER_CATEGORIES. */
export const PROVIDER_FILTER_CATEGORIES = FILTERABLE_PROVIDER_CATEGORIES;

/**
 * Categories shown in the create-form category dropdown that route to a
 * dedicated console instead of creating through the unified `/api/v1/providers`
 * endpoint. FaceId is a managed kind (face_id_providers table, AccessKey-style
 * credentials) and must never be submitted to the unified endpoint; selecting it
 * navigates to its independent create page. This keeps FaceId independently
 * selectable alongside IDVerification while CREATABLE_PROVIDER_CATEGORIES stays
 * free of managed kinds (the unified endpoint rejects them, P1 422).
 * Payment routes to the channel-config console: only pay_channel_configs rows
 * (activated via draft→testing→active) can drive ChargeOrchestrator.
 */
export const PROVIDER_CONSOLE_REDIRECTS: Readonly<Record<string, string>> = {
  FaceId: "/face-id-providers/new",
  Payment: "/payments/channel-configs",
};

/** Redirect categories appended to the create-form category options. */
export const CONSOLE_REDIRECT_PROVIDER_CATEGORIES: readonly ProviderCategoryOption[] =
  PROVIDER_CATEGORIES.filter((entry) => entry.value in PROVIDER_CONSOLE_REDIRECTS);

/**
 * Provider types shown in the list page filter dropdown.
 * LDAP is kept even though it is a managed kind: historical unified rows must
 * remain filterable. Do not derive this list from the create options.
 */
export const PROVIDER_FILTER_TYPES = [
  { value: "Google", label: "Google" },
  { value: "GitHub", label: "GitHub" },
  { value: "Facebook", label: "Facebook" },
  { value: "WeChat", label: "WeChat" },
  { value: "DingTalk", label: "DingTalk" },
  { value: "LDAP", label: "LDAP" },
  { value: "AD", label: "Active Directory" },
  { value: "Custom", label: "Custom" },
] as const;

// ─────────────────────────────────────────────
// Provider Type Definitions per Category (i18n)
// ─────────────────────────────────────────────

type ProviderTypeDefinition = { i18nKey?: string; label?: string; value: string };

const providerTypeDefinitions: Record<string, ReadonlyArray<ProviderTypeDefinition>> = {
  OAuth: [
    { i18nKey: "provider.types.oauth.github", value: "GitHub" },
    { i18nKey: "provider.types.oauth.google", value: "Google" },
    { i18nKey: "provider.types.oauth.azureAd", value: "AzureAD" },
    { i18nKey: "provider.types.oauth.azureAdB2c", value: "AzureADB2C" },
    { i18nKey: "provider.types.oauth.wechat", value: "WeChat" },
    { i18nKey: "provider.types.oauth.wechatMiniProgram", value: "WeChatMiniProgram" },
    { i18nKey: "provider.types.oauth.dingtalk", value: "DingTalk" },
    { i18nKey: "provider.types.oauth.lark", value: "Lark" },
    { i18nKey: "provider.types.oauth.facebook", value: "Facebook" },
    { i18nKey: "provider.types.oauth.twitter", value: "Twitter" },
    { i18nKey: "provider.types.oauth.linkedin", value: "LinkedIn" },
    { i18nKey: "provider.types.oauth.gitlab", value: "GitLab" },
    { i18nKey: "provider.types.oauth.slack", value: "Slack" },
    { i18nKey: "provider.types.oauth.discord", value: "Discord" },
    { i18nKey: "provider.types.oauth.apple", value: "Apple" },
    { i18nKey: "provider.types.oauth.okta", value: "Okta" },
    { i18nKey: "provider.types.oauth.adfs", value: "ADFS" },
    { i18nKey: "provider.types.oauth.alipay", value: "Alipay" },
    { i18nKey: "provider.types.oauth.baidu", value: "Baidu" },
    { i18nKey: "provider.types.oauth.douyin", value: "Douyin" },
    { i18nKey: "provider.types.oauth.qq", value: "QQ" },
    { i18nKey: "provider.types.oauth.weibo", value: "Weibo" },
    { i18nKey: "provider.types.oauth.steam", value: "Steam" },
    { i18nKey: "provider.types.oauth.telegram", value: "Telegram" },
    { i18nKey: "provider.types.oauth.amazon", value: "Amazon" },
    { i18nKey: "provider.types.oauth.bilibili", value: "Bilibili" },
    { i18nKey: "provider.types.oauth.gitee", value: "Gitee" },
    { i18nKey: "provider.types.oauth.infoflow", value: "Infoflow" },
    { i18nKey: "provider.types.oauth.kwai", value: "Kwai" },
    { i18nKey: "provider.types.oauth.wecom", value: "WeCom" },
    // Third-party OAuth catalog. Brand names are not translated: the raw
    // provider_type doubles as the dropdown label and as the factory key.
    { label: "Casdoor", value: "Casdoor" },
    { label: "MicrosoftOnline", value: "MicrosoftOnline" },
    { label: "PayPal", value: "PayPal" },
    { label: "Line", value: "Line" },
    { label: "Auth0", value: "Auth0" },
    { label: "Bitbucket", value: "Bitbucket" },
    { label: "BattleNet", value: "BattleNet" },
    { label: "Box", value: "Box" },
    { label: "CloudFoundry", value: "CloudFoundry" },
    { label: "Dailymotion", value: "Dailymotion" },
    { label: "Deezer", value: "Deezer" },
    { label: "DigitalOcean", value: "DigitalOcean" },
    { label: "Dropbox", value: "Dropbox" },
    { label: "EveOnline", value: "EveOnline" },
    { label: "Fitbit", value: "Fitbit" },
    { label: "Gitea", value: "Gitea" },
    { label: "Heroku", value: "Heroku" },
    { label: "InfluxCloud", value: "InfluxCloud" },
    { label: "Instagram", value: "Instagram" },
    { label: "Intercom", value: "Intercom" },
    { label: "Kakao", value: "Kakao" },
    { label: "Lastfm", value: "Lastfm" },
    { label: "Mailru", value: "Mailru" },
    { label: "Meetup", value: "Meetup" },
    { label: "Naver", value: "Naver" },
    { label: "Nextcloud", value: "Nextcloud" },
    { label: "OneDrive", value: "OneDrive" },
    { label: "Oura", value: "Oura" },
    { label: "Patreon", value: "Patreon" },
    { label: "SalesForce", value: "SalesForce" },
    { label: "Shopify", value: "Shopify" },
    { label: "Soundcloud", value: "Soundcloud" },
    { label: "Spotify", value: "Spotify" },
    { label: "Strava", value: "Strava" },
    { label: "Stripe", value: "Stripe" },
    { label: "TikTok", value: "TikTok" },
    { label: "Tumblr", value: "Tumblr" },
    { label: "Twitch", value: "Twitch" },
    { label: "Typetalk", value: "Typetalk" },
    { label: "Uber", value: "Uber" },
    { label: "VK", value: "VK" },
    { label: "Wepay", value: "Wepay" },
    { label: "Xero", value: "Xero" },
    { label: "Yahoo", value: "Yahoo" },
    { label: "Yammer", value: "Yammer" },
    { label: "Yandex", value: "Yandex" },
    { label: "Zoom", value: "Zoom" },
    { i18nKey: "provider.types.oauth.custom", value: "Custom" },
  ],
  SAML: [
    { i18nKey: "provider.types.saml.saml20", value: "SAML 2.0" },
    { i18nKey: "provider.types.saml.keycloak", value: "Keycloak" },
    { label: "Aliyun IDaaS", value: "AliyunIDaaS" },
    { label: "Custom Flexible", value: "CustomFlexible" },
    { i18nKey: "provider.types.saml.custom", value: "Custom" },
  ],
  OIDC: [
    { i18nKey: "provider.types.oidc.oidc", value: "OIDC" },
    { i18nKey: "provider.types.oidc.custom", value: "Custom" },
  ],
  Email: [
    { i18nKey: "provider.types.email.smtp", value: "SMTP" },
    { i18nKey: "provider.types.email.sendgrid", value: "SendGrid" },
    { i18nKey: "provider.types.email.azureAcs", value: "AzureACS" },
    { i18nKey: "provider.types.email.resend", value: "Resend" },
    { i18nKey: "provider.types.email.customHttp", value: "CustomHTTP" },
    // Raw brand labels: these vendors have no i18n entry, and the value doubles
    // as the factory key. SUBMAIL Email is distinct from the SMS provider_type.
    { label: "SUBMAIL", value: "SUBMAIL Email" },
    { label: "Mailtrap", value: "Mailtrap" },
    { i18nKey: "provider.types.email.directMail", value: "DirectMail" },
    { i18nKey: "provider.types.email.tencentSes", value: "TencentSES" },
    { i18nKey: "provider.types.email.sendCloud", value: "SendCloud" },
    { i18nKey: "provider.types.email.mailgun", value: "Mailgun" },
    { i18nKey: "provider.types.email.amazonSes", value: "AmazonSES" },
    { i18nKey: "provider.types.email.postmark", value: "Postmark" },
    { i18nKey: "provider.types.email.mailerSend", value: "MailerSend" },
    { i18nKey: "provider.types.email.brevo", value: "Brevo" },
    { i18nKey: "provider.types.email.zohoMail", value: "ZohoMail" },
  ],
  SMS: [
    { i18nKey: "provider.types.sms.twilioSms", value: "TwilioSMS" },
    { label: "Twilio Verify", value: "Twilio Verify" },
    { i18nKey: "provider.types.sms.alibabaSms", value: "AlibabaSMS" },
    { label: "Alibaba Cloud PNVS", value: "Alibaba Cloud PNVS SMS" },
    { i18nKey: "provider.types.sms.tencentSms", value: "TencentSMS" },
    { i18nKey: "provider.types.sms.volcengineSms", value: "VolcengineSMS" },
    { i18nKey: "provider.types.sms.huaweiSms", value: "HuaweiSMS" },
    { i18nKey: "provider.types.sms.amazonSns", value: "AmazonSNS" },
    { i18nKey: "provider.types.sms.azureAcsSms", value: "AzureACSSMS" },
    { i18nKey: "provider.types.sms.infobip", value: "Infobip" },
    { i18nKey: "provider.types.sms.ucloudSms", value: "UCloudSMS" },
    { i18nKey: "provider.types.sms.emaySms", value: "EmaySMS" },
    { i18nKey: "provider.types.sms.baiduSms", value: "BaiduSMS" },
    { i18nKey: "provider.types.sms.submail", value: "Submail" },
    { i18nKey: "provider.types.sms.mailtrapSms", value: "MailtrapSMS" },
    { i18nKey: "provider.types.sms.osonSms", value: "OsonSMS" },
    { i18nKey: "provider.types.sms.mockSms", value: "MockSMS" },
    { label: "SmsBao", value: "SmsBao SMS" },
    { label: "Msg91", value: "Msg91 SMS" },
    { i18nKey: "provider.types.sms.unismsSms", value: "UnismsSMS" },
    { i18nKey: "provider.types.sms.yunpianSms", value: "YunpianSMS" },
    { i18nKey: "provider.types.sms.jdcloudSms", value: "JDCloudSMS" },
    { i18nKey: "provider.types.sms.vonageSms", value: "VonageSMS" },
    { i18nKey: "provider.types.sms.plivoSms", value: "PlivoSMS" },
    { i18nKey: "provider.types.sms.telnyxSms", value: "TelnyxSMS" },
    { i18nKey: "provider.types.sms.sinchSms", value: "SinchSMS" },
    { i18nKey: "provider.types.sms.birdSms", value: "BirdSMS" },
    { i18nKey: "provider.types.sms.clickatellSms", value: "ClickatellSMS" },
    { i18nKey: "provider.types.sms.africastalkingSms", value: "AfricastalkingSMS" },
    { i18nKey: "provider.types.sms.customHttpSms", value: "Custom HTTP SMS" },
    { i18nKey: "provider.types.sms.custom", value: "Custom" },
  ],
  Storage: [
    { i18nKey: "provider.types.storage.awsS3", value: "AWS S3" },
    { i18nKey: "provider.types.storage.azureBlob", value: "AzureBlob" },
    { i18nKey: "provider.types.storage.gcs", value: "GCS" },
    { i18nKey: "provider.types.storage.minio", value: "MinIO" },
    { i18nKey: "provider.types.storage.aliOss", value: "AliOSS" },
    { i18nKey: "provider.types.storage.tencentCos", value: "TencentCOS" },
    { i18nKey: "provider.types.storage.qiniu", value: "Qiniu" },
    { i18nKey: "provider.types.storage.cucloud", value: "CUCloud" },
    { i18nKey: "provider.types.storage.huaweiObs", value: "HuaweiOBS" },
    // OPEN-API-75 P7 S3-compatible kinds; raw labels avoid new i18n keys.
    { label: "Cloudflare R2", value: "CloudflareR2" },
    { label: "Backblaze B2", value: "BackblazeB2" },
    { label: "DigitalOcean Spaces", value: "DigitalOceanSpaces" },
    { i18nKey: "provider.types.storage.synology", value: "Synology" },
    { i18nKey: "provider.types.storage.local", value: "Local" },
    { i18nKey: "provider.types.storage.custom", value: "Custom" },
  ],
  Payment: [
    { i18nKey: "provider.types.payment.stripe", value: "Stripe" },
    { i18nKey: "provider.types.payment.alipay", value: "Alipay" },
    { i18nKey: "provider.types.payment.wechatPay", value: "WeChatPay" },
    { i18nKey: "provider.types.payment.paypal", value: "PayPal" },
    { i18nKey: "provider.types.payment.adyen", value: "Adyen" },
    { i18nKey: "provider.types.payment.airWallex", value: "AirWallex" },
    // OPEN-API-75 P8 payment kinds; raw labels avoid new i18n keys.
    { label: "Square", value: "Square" },
    { label: "Checkout.com", value: "Checkout" },
    { i18nKey: "provider.types.payment.paddle", value: "Paddle" },
    { i18nKey: "provider.types.payment.polar", value: "Polar" },
    { i18nKey: "provider.types.payment.fastSpring", value: "FastSpring" },
    { i18nKey: "provider.types.payment.lemonSqueezy", value: "LemonSqueezy" },
    { i18nKey: "provider.types.payment.gc", value: "GC" },
    { i18nKey: "provider.types.payment.balance", value: "Balance" },
    { i18nKey: "provider.types.payment.dummy", value: "Dummy" },
    { i18nKey: "provider.types.payment.custom", value: "Custom" },
  ],
  Notification: [
    { i18nKey: "provider.types.notification.dingtalk", value: "DingTalk" },
    { i18nKey: "provider.types.notification.lark", value: "Lark" },
    { i18nKey: "provider.types.notification.wecom", value: "WeCom" },
    { i18nKey: "provider.types.notification.slack", value: "Slack" },
    { i18nKey: "provider.types.notification.telegram", value: "Telegram" },
    { i18nKey: "provider.types.notification.discord", value: "Discord" },
    { i18nKey: "provider.types.notification.webPush", value: "WebPush" },
    { i18nKey: "provider.types.notification.teams", value: "Teams" },
    { i18nKey: "provider.types.notification.customHttp", value: "CustomHTTP" },
    // Remaining Casdoor notification options; raw labels avoid 11 new i18n keys.
    { label: "Bark", value: "Bark" },
    { label: "Pushover", value: "Pushover" },
    { label: "Pushbullet", value: "Pushbullet" },
    { label: "Google Chat", value: "GoogleChat" },
    { label: "Line", value: "Line" },
    { label: "Matrix", value: "Matrix" },
    { label: "Twitter", value: "Twitter" },
    { label: "Reddit", value: "Reddit" },
    { label: "Rocket Chat", value: "RocketChat" },
    { label: "Viber", value: "Viber" },
    { label: "CUCloud", value: "CUCloud" },
    { label: "Ntfy", value: "Ntfy" },
    { label: "ServerChan", value: "ServerChan" },
    { label: "PagerDuty", value: "PagerDuty" },
    { label: "OpsGenie", value: "OpsGenie" },
    { label: "Gotify", value: "Gotify" },
  ],
  Captcha: [
    { i18nKey: "provider.types.captcha.default", value: "Default" },
    { i18nKey: "provider.types.captcha.recaptcha", value: "reCAPTCHA" },
    { i18nKey: "provider.types.captcha.recaptchaV3", value: "reCAPTCHAv3" },
    { i18nKey: "provider.types.captcha.hcaptcha", value: "hCaptcha" },
    { i18nKey: "provider.types.captcha.turnstile", value: "Turnstile" },
    { i18nKey: "provider.types.captcha.geetest", value: "GeeTest" },
    { i18nKey: "provider.types.captcha.aliyunCaptcha", value: "AliyunCaptcha" },
    { i18nKey: "provider.types.captcha.tencentCaptcha", value: "TencentCaptcha" },
    // Brand names stay untranslated; the raw label doubles as the UI value.
    { label: "reCAPTCHA Enterprise", value: "reCAPTCHAEnterprise" },
    { label: "NetEaseYidun", value: "NetEaseYidun" },
    { label: "BaiduCaptcha", value: "BaiduCaptcha" },
    { label: "GoCaptcha", value: "GoCaptcha" },
  ],
  MFA: [
    { i18nKey: "provider.types.mfa.totp", value: "TOTP" },
    { i18nKey: "provider.types.mfa.sms", value: "SMS" },
    { i18nKey: "provider.types.mfa.email", value: "Email" },
    { i18nKey: "provider.types.mfa.radius", value: "RADIUS" },
    { i18nKey: "provider.types.mfa.push", value: "Push" },
  ],
  FaceId: [
    { i18nKey: "provider.types.faceId.aliyunFaceBody", value: "AliyunFaceBody" },
    { i18nKey: "provider.types.faceId.custom", value: "Custom" },
  ],
  Web3: [
    { i18nKey: "provider.types.web3.metaMask", value: "MetaMask" },
    { i18nKey: "provider.types.web3.custom", value: "Custom" },
  ],
  Scan: [
    { i18nKey: "provider.types.scan.wechatScan", value: "WeChatScan" },
    { i18nKey: "provider.types.scan.dingtalkScan", value: "DingTalkScan" },
    { i18nKey: "provider.types.scan.custom", value: "Custom" },
  ],
  IDVerification: [
    { i18nKey: "provider.types.idVerification.jumio", value: "Jumio" },
    { i18nKey: "provider.types.idVerification.aliyunIdv", value: "AliyunIdv" },
    { i18nKey: "provider.types.idVerification.tencentIdv", value: "TencentIdv" },
    { label: "Onfido", value: "Onfido" },
    { label: "Veriff", value: "Veriff" },
    { label: "Persona", value: "Persona" },
    { i18nKey: "provider.types.idVerification.custom", value: "Custom" },
  ],
  Log: [
    { i18nKey: "provider.types.log.elasticsearch", value: "Elasticsearch" },
    { i18nKey: "provider.types.log.loki", value: "Loki" },
    { i18nKey: "provider.types.log.customHttp", value: "CustomHTTP" },
  ],
  Audit: [{ label: "Syslog", value: "Syslog" }],
};

/**
 * IDVerification 类型筛选选项：与创建表单的 providerType 定义同源，避免列表
 * 筛选误用社交类型（Google/GitHub…）。原始值即列表「类型」列展示的值，
 * getUnifiedProvidersPaginated 的 normalizeProviderType 会归一大小写/分隔符。
 */
export const IDVERIFICATION_FILTER_TYPES: readonly ProviderCategoryOption[] =
  providerTypeDefinitions.IDVerification.map((d) => ({
    label: d.label ?? d.value,
    value: d.value,
  }));

const DEFAULT_PROVIDER_TYPES: ReadonlyArray<{ label: string; value: string }> = [
  { label: "Custom", value: "Custom" },
];

/**
 * UI Captcha providerType value ↔ backend `captcha_providers.provider_type`
 * kind. Keep aligned with `internal/domain/captchaproviders/provider_type.go`.
 */
const CAPTCHA_UI_KIND_MAP: Readonly<Record<string, string>> = {
  Default: "default",
  reCAPTCHA: "recaptcha",
  reCAPTCHAv3: "recaptcha_v3",
  reCAPTCHAEnterprise: "recaptcha_enterprise",
  hCaptcha: "hcaptcha",
  Turnstile: "turnstile",
  GeeTest: "geetest",
  AliyunCaptcha: "aliyun_captcha",
  TencentCaptcha: "tencent_captcha",
  NetEaseYidun: "netease_yidun",
  BaiduCaptcha: "baidu_captcha",
  GoCaptcha: "go_captcha",
};

/** UI value → backend kind (identity for unknown values). */
export function captchaUITypeToKind(value: string): string {
  return CAPTCHA_UI_KIND_MAP[value] ?? value;
}

/** Backend kind → UI value (identity for unknown kinds). */
export function captchaKindToUIType(kind: string): string {
  for (const [ui, k] of Object.entries(CAPTCHA_UI_KIND_MAP)) {
    if (k === kind) return ui;
  }
  return kind;
}

// ─────────────────────────────────────────────
// Category Utility Functions
// ─────────────────────────────────────────────

/** Which of clientId / clientSecret the common "credentials" section renders. */
export type ProviderCredentialFields = { clientId: boolean; clientSecret: boolean };

/**
 * The credentials section only renders fields that have no other editable
 * control:
 * - Captcha edits both clientId (siteKey) and clientSecret (secretKey) in its
 *   dynamic section, so the section is hidden for it.
 * - Scan edits clientSecret as "App Secret" in its dynamic section; its
 *   clientId has no dynamic control, so it stays here.
 * - OAuth/OIDC edit both fields in their dynamic section (the section is hidden).
 */
export function categoryCredentialFields(category: string): ProviderCredentialFields {
  if (category === "Scan") return { clientId: true, clientSecret: false };
  return { clientId: false, clientSecret: false };
}

/** Whether the category supports user mapping (OAuth/SAML/OIDC) */
export function categorySupportsUserMapping(category: string): boolean {
  return ["OAuth", "SAML", "OIDC"].includes(category);
}

// ─────────────────────────────────────────────
// Auth switch applicability (per category / provider type)
// ─────────────────────────────────────────────

export type ProviderAuthSwitchName =
  | "enablePkce"
  | "enableProxy"
  | "httpHeaders"
  | "emailRegex"
  | "customLogoutUrl"
  | "customLogo"
  | "state"
  | "requireMessageAuthenticator"
  | "enableSignAuthnRequest"
  | "cert";

/** Identity providers that run the shared social login flow (LoginWithProvider). */
const SOCIAL_LOGIN_CATEGORIES = new Set(["OAuth", "OIDC", "SAML"]);
/** Identity providers whose runtime HTTP calls go through providerHTTPClient. */
const SOCIAL_HTTP_CATEGORIES = new Set(["OAuth", "OIDC"]);

/**
 * Whether an "auth switch" is meaningful for the given provider category/type.
 *
 * Backend evidence:
 * - enablePkce/enableProxy/httpHeaders: consumed by the OAuth/OIDC runtime
 *   (internal/infra/idp/provider_http.go, factory.go OAuth branch). SAML builds
 *   no HTTP client there.
 * - emailRegex: validateProviderEmail in loginWithProviderInTx, which
 *   OAuth/OIDC/SAML all reach through LoginWithProvider.
 * - customLogoutUrl: read by invokeCustomProviderLogout, which only fires when
 *   the session sealed an upstream access token (OAuth/OIDC token exchange).
 * - state: ensureProviderUsable disables a provider whose state is "disabled",
 *   for every category.
 * - requireMessageAuthenticator: RADIUS MFA config only.
 * - enableSignAuthnRequest/cert: SAML SP adapter (cert is the IdP certificate).
 * - customLogo: no backend consumer found, so its applicable type is NOT
 *   confirmed. Per the no-field-loss rule it stays visible everywhere rather
 *   than guessing a filter.
 */
export function isAuthSwitchApplicable(
  field: ProviderAuthSwitchName,
  category: string,
  providerType?: string,
): boolean {
  switch (field) {
    case "enablePkce":
    case "enableProxy":
    case "httpHeaders":
    case "customLogoutUrl":
      return SOCIAL_HTTP_CATEGORIES.has(category);
    case "emailRegex":
      return SOCIAL_LOGIN_CATEGORIES.has(category);
    case "requireMessageAuthenticator":
      return category === "MFA" && String(providerType ?? "").toUpperCase() === "RADIUS";
    case "enableSignAuthnRequest":
    case "cert":
      return category === "SAML";
    // state is a cross-category gate; customLogo's applicable type is unconfirmed.
    case "state":
    case "customLogo":
    default:
      return true;
  }
}

/** Color mapping for provider category tags */
export function getCategoryColor(category: string): string {
  const colorMap: Record<string, string> = {
    OAuth: "blue",
    SAML: "purple",
    OIDC: "cyan",
    Email: "green",
    SMS: "orange",
    Storage: "geekblue",
    Payment: "gold",
    Notification: "lime",
    Captcha: "magenta",
    MFA: "red",
    FaceId: "volcano",
    Web3: "purple",
    Scan: "blue",
    IDVerification: "orange",
    Log: "default",
    Audit: "default",
  };
  return colorMap[category] ?? "default";
}

// ─────────────────────────────────────────────
// Hook: useProviderCategories
// ─────────────────────────────────────────────

export type UseProviderCategoriesOptions = {
  /** Currently selected category — drives the providerType options */
  category: string;
};

export type UseProviderCategoriesReturn = {
  /** Full category options for Select dropdown */
  categoryOptions: { label: string; value: string }[];
  /** Provider type options for the current category (i18n-resolved) */
  providerTypeOptions: { label: string; value: string }[];
  /** Which credential inputs the common section renders for the current category */
  credentialFields: ProviderCredentialFields;
  /** Whether the current category supports user mapping */
  showUserMapping: boolean;
  /** Color for the current category tag */
  categoryColor: string;
};

/**
 * Hook that provides provider category metadata and derived state.
 *
 * Centralizes the category → type mapping, feature flags (credentialFields,
 * showUserMapping), and tag color logic used across both the detail
 * and list pages.
 */
export function useProviderCategories(
  options: UseProviderCategoriesOptions,
): UseProviderCategoriesReturn {
  const { category } = options;
  const { t: tr } = useTranslation("resource");

  const providerTypeOptions = useMemo(() => {
    const defs = providerTypeDefinitions[category];
    if (!defs) return DEFAULT_PROVIDER_TYPES;
    // Historical rows may still carry a managed kind (e.g. reCAPTCHA), but the
    // unified entry point rejects creating or changing them (P1 422). Filter
    // them out so the form can never re-introduce a managed kind.
    // Captcha is exempt: selecting a captcha type routes the save to the
    // dedicated `/api/v1/captcha-providers` endpoint instead of the unified one.
    return defs
      .filter((d) => category === "Captcha" || !isManagedProviderKind(d.value))
      .map((d) => ({
        label: d.label ?? (d.i18nKey && tr(d.i18nKey) !== d.i18nKey ? tr(d.i18nKey) : d.value),
        value: d.value,
      }));
  }, [category, tr]);

  // 稳定引用：调用方（如 ProviderDetailPage 的预置类别 effect）会把该数组放进
  // effect 依赖；若每次 render 都返回新数组，effect 会反复重跑并把用户刚选中的
  // providerType 清回 undefined，导致「类型选了不生效」。
  const categoryOptions = useMemo(
    () => [...CREATABLE_PROVIDER_CATEGORIES, ...CONSOLE_REDIRECT_PROVIDER_CATEGORIES],
    [],
  );

  return {
    categoryOptions,
    providerTypeOptions: providerTypeOptions as { label: string; value: string }[],
    credentialFields: categoryCredentialFields(category),
    showUserMapping: categorySupportsUserMapping(category),
    categoryColor: getCategoryColor(category),
  };
}

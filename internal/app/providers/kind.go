package providers

import (
	"regexp"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

// identityKinds 是 providers.kind 在 family='identity' 下的闭集值（声明顺序）。
//
// 必须与 contracts/schema/20_identity.yaml 的 ck_providers_kind 保持一致：这里是
// 应用层的 fail-fast 校验（返回 422 而不是让 INSERT 撞 23514 CHECK 约束）。
// provider_type 保留厂商写法（AzureAD / AlibabaSMS / AWS S3 …），kind 只是注册键。
//
// The slice is the one declaration; the lookup map is derived from it, and the
// 422 `allowed` list reads the same slice so a rejection cannot name a value
// the map would reject.
// Pending kinds are named constants so the closed set, the runtime route table
// and the subsystem gates cannot drift on a spelling difference. Kinds that
// already have a consumer stay literal until their subsystem is touched.
const (
	identityKindAzureACS           = "azure_acs"
	identityKindCustomHTTP         = "custom_http"
	identityKindCustomHTTPSMS      = "custom_http_sms"
	identityKindSubmailEmail       = "submail_email"
	identityKindMailtrapEmail      = "mailtrap_email"
	identityKindDirectMail         = "directmail"
	identityKindTencentSES         = "tencent_ses"
	identityKindSendCloud          = "sendcloud"
	identityKindMailgun            = "mailgun"
	identityKindAmazonSES          = "amazon_ses"
	identityKindPostmark           = "postmark"
	identityKindMailerSend         = "mailersend"
	identityKindBrevo              = "brevo"
	identityKindZohoMail           = "zoho_mail"
	identityKindTwilioSMS          = "twilio_sms"
	identityKindTwilioVerify       = "twilio_verify"
	identityKindAlibabaPNVS        = "alibaba_pnvs"
	identityKindAlibabaSMS         = "alibaba_sms"
	identityKindTencentSMS         = "tencent_sms"
	identityKindVolcengineSMS      = "volcengine_sms"
	identityKindHuaweiSMS          = "huawei_sms"
	identityKindAmazonSNS          = "amazon_sns"
	identityKindAzureACSSMS        = "azure_acs_sms"
	identityKindInfobip            = "infobip"
	identityKindUCloudSMS          = "ucloud_sms"
	identityKindEmaySMS            = "emay_sms"
	identityKindBaiduSMS           = "baidu_sms"
	identityKindSubmail            = "submail"
	identityKindMailtrapSMS        = "mailtrap_sms"
	identityKindOsonSMS            = "oson_sms"
	identityKindMockSMS            = "mock_sms"
	identityKindSmsBao             = "smsbao"
	identityKindMsg91              = "msg91"
	identityKindUnismsSMS          = "unisms_sms"
	identityKindYunpianSMS         = "yunpian_sms"
	identityKindJDCloudSMS         = "jdcloud_sms"
	identityKindVonageSMS          = "vonage_sms"
	identityKindPlivoSMS           = "plivo_sms"
	identityKindTelnyxSMS          = "telnyx_sms"
	identityKindSinchSMS           = "sinch_sms"
	identityKindBirdSMS            = "bird_sms"
	identityKindClickatellSMS      = "clickatell_sms"
	identityKindAfricastalkingSMS  = "africastalking_sms"
	identityKindAWSS3              = "aws_s3"
	identityKindAzureBlob          = "azure_blob"
	identityKindAliOSS             = "ali_oss"
	identityKindTencentCOS         = "tencent_cos"
	identityKindCUCloud            = "cu_cloud"
	identityKindHuaweiOBS          = "huawei_obs"
	identityKindCloudflareR2       = "cloudflare_r2"
	identityKindBackblazeB2        = "backblaze_b2"
	identityKindDigitalOceanSpaces = "digitalocean_spaces"
	identityKindRecaptchaV3        = "recaptcha_v3"
	identityKindAliyunCaptcha      = "aliyun_captcha"
	identityKindTencentCaptcha     = "tencent_captcha"
	identityKindAliyunFaceBody     = "aliyun_face_body"
	identityKindWechatScan         = "wechat_scan"
	identityKindDingtalkScan       = "dingtalk_scan"
	identityKindElasticsearch      = "elasticsearch"
	identityKindLoki               = "loki"
)

var identityKinds = []string{
	// 身份联合 / 社交 / 企业 IdP
	"github", "google", "azure_ad", "azure_ad_b2c", "facebook", "linkedin", "twitter",
	"gitlab", "gitee", "okta", "wechat", "wechat_mobile", "wechat_miniprogram",
	"wecom_internal", "wecom_third", "dingtalk", "lark", "feishu", "qq", "weibo",
	"baidu", "douyin", "kwai", "bilibili", "infoflow_internal", "infoflow_third",
	"telegram", "ldap", "adfs", "saml", "oidc", "custom", "goth",
	// Web3
	"metamask", "web3onboard",
	// 支付类身份入口
	"alipay", "wechat_pay",
	// 邮件 / 通知渠道（能力型 provider 落在 identity family）
	"smtp", "default", "local", "faceid", "apple", "keycloak", "slack", "discord",
	"steam", "amazon", "sendgrid", identityKindAzureACS, "resend",
	identityKindSubmailEmail, identityKindMailtrapEmail,
	identityKindDirectMail, identityKindTencentSES, identityKindSendCloud,
	identityKindMailgun, identityKindAmazonSES, identityKindPostmark,
	identityKindMailerSend, identityKindBrevo, identityKindZohoMail,
	identityKindCustomHTTP,
	// SMS 厂商
	"sms", identityKindTwilioSMS, identityKindAlibabaSMS, identityKindAlibabaPNVS,
	identityKindTencentSMS,
	identityKindVolcengineSMS, identityKindHuaweiSMS, identityKindAmazonSNS,
	identityKindAzureACSSMS, identityKindInfobip, identityKindUCloudSMS,
	identityKindEmaySMS, identityKindBaiduSMS, identityKindSubmail,
	identityKindMailtrapSMS, identityKindOsonSMS, identityKindMockSMS,
	identityKindTwilioVerify, identityKindSmsBao, identityKindMsg91,
	identityKindCustomHTTPSMS,
	identityKindUnismsSMS, identityKindYunpianSMS, identityKindJDCloudSMS,
	identityKindVonageSMS, identityKindPlivoSMS, identityKindTelnyxSMS,
	identityKindSinchSMS, identityKindBirdSMS, identityKindClickatellSMS,
	identityKindAfricastalkingSMS,
	// 对象存储
	identityKindAWSS3, identityKindAzureBlob, "gcs", "minio", identityKindAliOSS,
	identityKindTencentCOS, "qiniu", identityKindCUCloud, identityKindHuaweiOBS,
	identityKindCloudflareR2, identityKindBackblazeB2, identityKindDigitalOceanSpaces, "synology",
	// 验证码 / MFA / 人脸 / 扫码 / 实名 / 审计 / 日志
	"syslog",
	"recaptcha", identityKindRecaptchaV3, "hcaptcha", "turnstile", "geetest",
	identityKindAliyunCaptcha, identityKindTencentCaptcha, "totp", "radius", "push", identityKindAliyunFaceBody,
	identityKindWechatScan, identityKindDingtalkScan, "jumio", "aliyun_idv", "tencent_idv",
	"onfido", "veriff", "persona",
	identityKindElasticsearch, identityKindLoki,
}

var identityClosedKinds = func() map[domain.Kind]bool {
	set := make(map[domain.Kind]bool, len(identityKinds))
	for _, kind := range identityKinds {
		set[domain.Kind(kind)] = true
	}
	return set
}()

// gothProviderTypes are the provider_type values served by the Goth adapter.
// They have no dedicated closed-set kind: the row keeps kind=goth and routes
// on provider_type. check-provider-kinds.py holds this list equal to
// idp.SupportedGothTypes() and to provider-kinds.yaml goth_types.
var gothProviderTypes = []string{
	"battlenet", "box", "cloudfoundry", "dailymotion", "deezer",
	"digitalocean", "dropbox", "eveonline", "fitbit", "gitea",
	"heroku", "influxcloud", "instagram", "intercom", "kakao",
	"lastfm", "mailru", "meetup", "microsoftonline", "naver", "nextcloud",
	"onedrive", "oura", "patreon", "paypal", "salesforce", "shopify",
	"soundcloud", "spotify", "strava", "stripe", "tiktok",
	"tumblr", "twitch", "typetalk", "uber", "vk",
	"wepay", "xero", "yahoo", "yammer", "yandex", "zoom",
}

// nativeThirdPartyOAuthTypes are native adapters that also have no dedicated
// closed-set kind. They share kind=goth so the row stays governed by the
// registry instead of falling back to kind=custom.
var nativeThirdPartyOAuthTypes = []string{"line", "auth0", "bitbucket", "casdoor"}

var thirdPartyOAuthTypes = func() map[string]bool {
	set := make(map[string]bool, len(gothProviderTypes)+len(nativeThirdPartyOAuthTypes))
	for _, name := range append(append([]string{}, gothProviderTypes...), nativeThirdPartyOAuthTypes...) {
		set[normalizeOAuthType(name)] = true
	}
	return set
}()

// normalizeOAuthType mirrors idp.normalizeGothType: lowercase and strip
// separators, so "Battle-Net", "battle_net" and "BattleNet" are one key.
func normalizeOAuthType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("-", "", "_", "").Replace(value)
	return value
}

// isThirdPartyOAuthType reports whether provider_type is served by the Goth
// adapter or one of the native catalog-only adapters.
func isThirdPartyOAuthType(providerType string) bool {
	return thirdPartyOAuthTypes[normalizeOAuthType(providerType)]
}

// paymentKinds 是 family='payment' 下可路由的渠道注册键；与支付工厂注册表一致。
// 值此前内联在 paymentStrategy.Validate 的 switch 里，这里提升为唯一声明，
// 让 422 的 `allowed` 列表与校验读取同一份数据。
var paymentKinds = []string{
	"alipay", "wechat_pay", "stripe", "paypal", "adyen", "airwallex", "square",
	"checkout", "paddle", "fastspring", "lemonsqueezy", "polar", "balance", "gc", "dummy",
}

// identityKindAliases 处理「厂商写法 ≠ 注册键」的不规则映射：驼峰拆分不足以
// 得到闭集值（AzureADB2C → azure_ad_b2c、reCAPTCHAv3 → recaptcha_v3 等）。
var identityKindAliases = map[string]domain.Kind{
	"azureadb2c":             "azure_ad_b2c",
	"wecom":                  "wecom_internal",
	"infoflow":               "infoflow_internal",
	"wechatpay":              "wechat_pay",
	"saml 2.0":               "saml",
	"recaptchav3":            "recaptcha_v3",
	"azureacssms":            "azure_acs_sms",
	"cucloud":                "cu_cloud",
	"digitaloceanspaces":     "digitalocean_spaces",
	"wechatscan":             "wechat_scan",
	"dingtalkscan":           "dingtalk_scan",
	"aliyuncaptcha":          "aliyun_captcha",
	"tencentcaptcha":         "tencent_captcha",
	"alibaba cloud pnvs sms": "alibaba_pnvs",
	"alibaba cloud pnvs":     "alibaba_pnvs",
	"smsbao sms":             "smsbao",
	"msg91 sms":              "msg91",
	"submail":                "submail",
	"submail sms":            "submail",
	"submail email":          "submail_email",
	"mailtrap":               "mailtrap_email",
	"mailtrap email":         "mailtrap_email",
}

var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// canonicalIdentityKind 按「闭集精确匹配 → 别名 → 驼峰转 snake → custom 兜底」解析。
//
// 兜底 custom 仍保留：闭集外的未知厂商依旧可以建档（厂商信息在 provider_type），
// 但新增厂商时应优先扩展 schema 闭集，否则会退化成 custom。
func canonicalIdentityKind(providerType string) domain.Kind {
	raw := strings.TrimSpace(providerType)
	name := strings.ToLower(raw)
	if alias, ok := identityKindAliases[name]; ok {
		return alias
	}
	if identityClosedKinds[domain.Kind(name)] {
		return domain.Kind(name)
	}
	if snake := toSnakeCase(raw); identityClosedKinds[domain.Kind(snake)] {
		return domain.Kind(snake)
	}
	// Third-party OAuth catalogs share kind=goth; provider_type keeps the
	// vendor name for the idp factory. This keeps them out of the custom
	// bucket and under the registry's goth route.
	if isThirdPartyOAuthType(raw) {
		return domain.Kind(domain.ProviderGoth)
	}
	return domain.Kind(domain.ProviderCustom)
}

// toSnakeCase 只处理驼峰与分隔符差异：AzureAD → azure_ad、AWS S3 → aws_s3、
// AlibabaSMS → alibaba_sms。不规则写法由 identityKindAliases 兜住。
func toSnakeCase(value string) string {
	value = strings.NewReplacer(" ", "_", "-", "_", "/", "_").Replace(value)
	value = camelBoundary.ReplaceAllString(value, "${1}_${2}")
	return strings.ToLower(value)
}

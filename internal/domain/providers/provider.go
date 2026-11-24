package providers

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ProviderType represents the type of identity provider.
// Covers OAuth2/OIDC, SAML, LDAP, social platforms, Web3, and enterprise IdPs.
type ProviderType string

const (
	// OAuth 2.0 / OIDC Providers
	ProviderGitHub     ProviderType = "github"
	ProviderGoogle     ProviderType = "google"
	ProviderAzureAD    ProviderType = "azure_ad"
	ProviderAzureADB2C ProviderType = "azure_ad_b2c"
	ProviderFacebook   ProviderType = "facebook"
	ProviderLinkedIn   ProviderType = "linkedin"
	ProviderTwitter    ProviderType = "twitter"
	ProviderGitLab     ProviderType = "gitlab"
	ProviderGitee      ProviderType = "gitee"
	ProviderOkta       ProviderType = "okta"

	// Chinese Social Platforms
	ProviderWeChat           ProviderType = "wechat"
	ProviderWeChatMobile     ProviderType = "wechat_mobile"
	ProviderWeChatMini       ProviderType = "wechat_miniprogram"
	ProviderWeComInternal    ProviderType = "wecom_internal"
	ProviderWeComThird       ProviderType = "wecom_third"
	ProviderDingTalk         ProviderType = "dingtalk"
	ProviderLark             ProviderType = "lark"
	ProviderFeishu           ProviderType = "feishu"
	ProviderQQ               ProviderType = "qq"
	ProviderWeibo            ProviderType = "weibo"
	ProviderBaidu            ProviderType = "baidu"
	ProviderDouyin           ProviderType = "douyin"
	ProviderKwai             ProviderType = "kwai"
	ProviderBilibili         ProviderType = "bilibili"
	ProviderInfoFlowInternal ProviderType = "infoflow_internal"
	ProviderInfoFlowThird    ProviderType = "infoflow_third"
	ProviderTelegram         ProviderType = "telegram"

	// Enterprise / LDAP / SAML
	ProviderLDAP   ProviderType = "ldap"
	ProviderADFS   ProviderType = "adfs"
	ProviderSAML   ProviderType = "saml"
	ProviderOIDC   ProviderType = "oidc"
	ProviderCustom ProviderType = "custom"
	ProviderGoth   ProviderType = "goth"

	// Web3 / Crypto
	ProviderMetamask    ProviderType = "metamask"
	ProviderWeb3Onboard ProviderType = "web3onboard"

	// Payment
	ProviderAlipay    ProviderType = "alipay"
	ProviderWeChatPay ProviderType = "wechat_pay"

	// Audit
	ProviderSyslog ProviderType = "syslog"

	// Storage (vendor spellings kept for the console; the registry kind is
	// derived by canonicalIdentityKind).
	ProviderAWSS3      ProviderType = "AWS S3"
	ProviderAzureBlob  ProviderType = "AzureBlob"
	ProviderAliOSS     ProviderType = "AliOSS"
	ProviderTencentCOS ProviderType = "TencentCOS"
	ProviderHuaweiOBS  ProviderType = "HuaweiOBS"
	ProviderCUCloud    ProviderType = "CUCloud"
	// OPEN-API-75 P7 新增 S3 兼容厂商（控制台拼写，kind 由 canonicalIdentityKind 派生）。
	ProviderCloudflareR2       ProviderType = "CloudflareR2"
	ProviderBackblazeB2        ProviderType = "BackblazeB2"
	ProviderDigitalOceanSpaces ProviderType = "DigitalOceanSpaces"
	ProviderMinIO              ProviderType = "MinIO"
	ProviderGCS                ProviderType = "GCS"
	ProviderQiniu              ProviderType = "Qiniu"
	ProviderSynology           ProviderType = "Synology"
	ProviderLocal              ProviderType = "Local"
)

// ProviderCategory represents the category of the identity provider.
type ProviderCategory string

const (
	CategoryOAuth          ProviderCategory = "OAuth"
	CategoryOIDC           ProviderCategory = "OIDC"
	CategorySAML           ProviderCategory = "SAML"
	CategoryLDAP           ProviderCategory = "LDAP"
	CategoryWeb3           ProviderCategory = "Web3"
	CategoryCustom         ProviderCategory = "Custom"
	CategorySocial         ProviderCategory = "Social"
	CategoryEnterprise     ProviderCategory = "Enterprise"
	CategoryPayment        ProviderCategory = "Payment"
	CategoryAudit          ProviderCategory = "Audit"
	CategoryStorage        ProviderCategory = "Storage"
	CategoryEmail          ProviderCategory = "Email"
	CategorySMS            ProviderCategory = "SMS"
	CategoryNotification   ProviderCategory = "Notification"
	CategoryCaptcha        ProviderCategory = "Captcha"
	CategoryMFA            ProviderCategory = "MFA"
	CategoryFaceID         ProviderCategory = "FaceId"
	CategoryScan           ProviderCategory = "Scan"
	CategoryIDVerification ProviderCategory = "IDVerification"
	CategoryLog            ProviderCategory = "Log"
)

// unknownProviderKindError is the explicit failure for a kind that is not
// registered in contracts/registries/provider-kinds.yaml. The old resolver
// silently returned OAuth here, which turned every new kind into a bogus
// "category does not match type (expected OAuth)" at activation time.
func unknownProviderKindError(kind Kind) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"unknown provider kind: "+string(kind),
		map[string]any{"field": "kind", "value": string(kind)},
	)
}

// providerKindCategories is the domain-side closure of
// contracts/registries/provider-kinds.yaml: every registered kind names the
// category (or categories) the console may store with it. The first entry is
// the canonical one returned by GetProviderCategory; the alternatives cover
// kinds the UI legitimately offers under more than one category (for example
// Keycloak under OAuth and SAML) and the legacy Go-only categories (Social /
// Enterprise) that older rows still carry.
//
// Kinds absent from this map are unknown and are rejected; there is deliberately
// no default branch.
var providerKindCategories = map[Kind][]ProviderCategory{
	// Identity federation, social login and enterprise IdPs.
	"github":             {CategoryOAuth},
	"google":             {CategoryOAuth},
	"azure_ad":           {CategoryOAuth, CategoryOIDC},
	"azure_ad_b2c":       {CategoryOAuth, CategoryOIDC},
	"facebook":           {CategoryOAuth},
	"linkedin":           {CategoryOAuth},
	"twitter":            {CategoryOAuth},
	"gitlab":             {CategoryOAuth},
	"gitee":              {CategoryOAuth},
	"okta":               {CategoryOAuth, CategoryOIDC},
	"wechat":             {CategoryOAuth, CategorySocial},
	"wechat_mobile":      {CategoryOAuth, CategorySocial},
	"wechat_miniprogram": {CategoryOAuth, CategorySocial},
	"wecom_internal":     {CategoryOAuth, CategorySocial},
	"wecom_third":        {CategoryOAuth, CategorySocial},
	"dingtalk":           {CategoryOAuth, CategorySocial},
	"lark":               {CategoryOAuth, CategorySocial},
	"feishu":             {CategoryOAuth, CategorySocial},
	"qq":                 {CategoryOAuth, CategorySocial},
	"weibo":              {CategoryOAuth, CategorySocial},
	"baidu":              {CategoryOAuth, CategorySocial},
	"douyin":             {CategoryOAuth, CategorySocial},
	"kwai":               {CategoryOAuth, CategorySocial},
	"bilibili":           {CategoryOAuth, CategorySocial},
	"infoflow_internal":  {CategoryOAuth, CategorySocial},
	"infoflow_third":     {CategoryOAuth, CategorySocial},
	"telegram":           {CategoryOAuth, CategorySocial},
	"adfs":               {CategoryOAuth, CategoryEnterprise},
	"apple":              {CategoryOAuth},
	"keycloak":           {CategoryOAuth, CategorySAML},
	"slack":              {CategoryOAuth},
	"discord":            {CategoryOAuth},
	"steam":              {CategoryOAuth},
	"amazon":             {CategoryOAuth},
	"alipay":             {CategoryOAuth, CategoryPayment},
	"wechat_pay":         {CategoryOAuth, CategoryPayment},

	// Protocol-level identity providers.
	"saml": {CategorySAML, CategoryEnterprise},
	"oidc": {CategoryOIDC},
	"ldap": {CategoryLDAP, CategoryEnterprise},

	// Generic buckets: the UI offers "Custom"/"Goth" under several categories,
	// so the kind carries no category constraint of its own.
	"custom": allProviderCategories,
	"goth":   allProviderCategories,

	// Web3 wallets.
	"metamask":    {CategoryWeb3},
	"web3onboard": {CategoryWeb3},

	// Email.
	"smtp":           {CategoryEmail},
	"sendgrid":       {CategoryEmail},
	"azure_acs":      {CategoryEmail},
	"resend":         {CategoryEmail},
	"submail_email":  {CategoryEmail},
	"mailtrap_email": {CategoryEmail},
	"directmail":     {CategoryEmail},
	"tencent_ses":    {CategoryEmail},
	"sendcloud":      {CategoryEmail},
	"mailgun":        {CategoryEmail},
	"amazon_ses":     {CategoryEmail},
	"postmark":       {CategoryEmail},
	"mailersend":     {CategoryEmail},
	"brevo":          {CategoryEmail},
	"zoho_mail":      {CategoryEmail},
	"custom_http":    {CategoryEmail, CategoryLog},

	// SMS.
	"sms":                {CategorySMS, CategoryMFA},
	"twilio_sms":         {CategorySMS},
	"alibaba_sms":        {CategorySMS},
	"tencent_sms":        {CategorySMS},
	"volcengine_sms":     {CategorySMS},
	"huawei_sms":         {CategorySMS},
	"amazon_sns":         {CategorySMS},
	"azure_acs_sms":      {CategorySMS},
	"infobip":            {CategorySMS},
	"ucloud_sms":         {CategorySMS},
	"emay_sms":           {CategorySMS},
	"baidu_sms":          {CategorySMS},
	"submail":            {CategorySMS},
	"mailtrap_sms":       {CategorySMS},
	"oson_sms":           {CategorySMS},
	"mock_sms":           {CategorySMS},
	"twilio_verify":      {CategorySMS},
	"alibaba_pnvs":       {CategorySMS},
	"smsbao":             {CategorySMS},
	"msg91":              {CategorySMS},
	"custom_http_sms":    {CategorySMS},
	"unisms_sms":         {CategorySMS},
	"yunpian_sms":        {CategorySMS},
	"jdcloud_sms":        {CategorySMS},
	"vonage_sms":         {CategorySMS},
	"plivo_sms":          {CategorySMS},
	"telnyx_sms":         {CategorySMS},
	"sinch_sms":          {CategorySMS},
	"bird_sms":           {CategorySMS},
	"clickatell_sms":     {CategorySMS},
	"africastalking_sms": {CategorySMS},

	// Object storage.
	"aws_s3":              {CategoryStorage},
	"azure_blob":          {CategoryStorage},
	"gcs":                 {CategoryStorage},
	"minio":               {CategoryStorage},
	"ali_oss":             {CategoryStorage},
	"tencent_cos":         {CategoryStorage},
	"qiniu":               {CategoryStorage},
	"cu_cloud":            {CategoryStorage},
	"huawei_obs":          {CategoryStorage},
	"cloudflare_r2":       {CategoryStorage},
	"backblaze_b2":        {CategoryStorage},
	"digitalocean_spaces": {CategoryStorage},
	"synology":            {CategoryStorage},
	"local":               {CategoryStorage},

	// Captcha.
	"default":         {CategoryCaptcha},
	"recaptcha":       {CategoryCaptcha},
	"recaptcha_v3":    {CategoryCaptcha},
	"hcaptcha":        {CategoryCaptcha},
	"turnstile":       {CategoryCaptcha},
	"geetest":         {CategoryCaptcha},
	"aliyun_captcha":  {CategoryCaptcha},
	"tencent_captcha": {CategoryCaptcha},

	// MFA.
	"totp":   {CategoryMFA},
	"radius": {CategoryMFA},
	"push":   {CategoryMFA},

	// Face verification.
	"faceid":           {CategoryFaceID},
	"aliyun_face_body": {CategoryFaceID},

	// QR code scan login.
	"wechat_scan":   {CategoryScan},
	"dingtalk_scan": {CategoryScan},

	// Identity verification.
	"jumio":       {CategoryIDVerification},
	"aliyun_idv":  {CategoryIDVerification},
	"tencent_idv": {CategoryIDVerification},
	"onfido":      {CategoryIDVerification},
	"veriff":      {CategoryIDVerification},
	"persona":     {CategoryIDVerification},

	// Audit / logs.
	"syslog":        {CategoryAudit},
	"elasticsearch": {CategoryLog},
	"loki":          {CategoryLog},
}

// allProviderCategories is the declared category set. It is also the value list
// used to reject unknown category strings, so a valid category can never be
// missing from the 422 metadata.
var allProviderCategories = []ProviderCategory{
	CategoryOAuth, CategoryOIDC, CategorySAML, CategoryLDAP, CategoryWeb3,
	CategoryCustom, CategorySocial, CategoryEnterprise, CategoryPayment,
	CategoryAudit, CategoryStorage, CategoryEmail, CategorySMS,
	CategoryNotification, CategoryCaptcha, CategoryMFA, CategoryFaceID,
	CategoryScan, CategoryIDVerification, CategoryLog,
}

// Valid reports whether the value is a declared provider category.
func (c ProviderCategory) Valid() bool {
	for _, allowed := range allProviderCategories {
		if c == allowed {
			return true
		}
	}
	return false
}

// ProviderCategoriesForKind returns every category the kind may be stored with,
// canonical first. An unregistered kind is an explicit 422, never a default.
func ProviderCategoriesForKind(kind Kind) ([]ProviderCategory, error) {
	categories, ok := providerKindCategories[kind]
	if !ok {
		return nil, unknownProviderKindError(kind)
	}
	return append([]ProviderCategory(nil), categories...), nil
}

type Provider struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Category       ProviderCategory
	ProviderType   ProviderType
	SubType        string
	Family         Family
	Kind           Kind
	Variant        Variant
	Method         string
	ClientID       string
	ClientSecret   string
	ClientID2      string
	ClientSecret2  string
	ProviderURL    string
	AuthUrl        string
	TokenUrl       string
	UserInfoUrl    string
	CallbackUrl    string
	SigningAlg     string
	Scopes         []string
	IdPEntityId    string
	SsoUrl         string
	SloUrl         string
	Metadata       string
	IssuerUrl      string
	Region         string
	Host           string
	Port           int
	DatabaseType   string
	UserMapping    map[string]any
	Template       string
	CustomApi      string
	CustomHeader   string
	CustomBody     string
	Admin          string
	Password       string
	AppId          string
	AppSecret      string // WeChat/WeCom AppSecret
	ContentType    string
	Sender         string
	Receiver       string
	AgentID        string // WeCom AgentID
	Enabled        bool

	// Casdoor-compatible provider options. They are persisted in
	// provider_configs under `_provider_settings_v1` rather than as native
	// columns, so the taxonomy can grow without a migration per field.
	EmailRegex                  string
	EnablePkce                  bool
	EnableProxy                 bool
	HttpHeaders                 map[string]string
	RequireMessageAuthenticator bool
	EnableSignAuthnRequest      bool
	CustomLogoutUrl             string
	CustomLogo                  string
	Cert                        string
	State                       string

	CreatedAt time.Time
	UpdatedAt time.Time
	Revision  int64
}

// GetSupportedProviderTypes returns all supported provider types.
func GetSupportedProviderTypes() []ProviderType {
	return []ProviderType{
		// OAuth/OIDC
		ProviderGitHub, ProviderGoogle, ProviderAzureAD, ProviderAzureADB2C,
		ProviderFacebook, ProviderLinkedIn, ProviderTwitter, ProviderGitLab,
		ProviderGitee, ProviderOkta,

		// Chinese Social
		ProviderWeChat, ProviderWeChatMobile, ProviderWeChatMini,
		ProviderWeComInternal, ProviderWeComThird,
		ProviderDingTalk, ProviderLark, ProviderFeishu,
		ProviderQQ, ProviderWeibo, ProviderBaidu, ProviderDouyin,
		ProviderKwai, ProviderBilibili,
		ProviderInfoFlowInternal, ProviderInfoFlowThird,
		ProviderTelegram,

		// Enterprise
		ProviderLDAP, ProviderADFS, ProviderSAML, ProviderOIDC, ProviderCustom, ProviderGoth,

		// Web3
		ProviderMetamask, ProviderWeb3Onboard,

		// Payment
		ProviderAlipay, ProviderWeChatPay,

		// Audit
		ProviderSyslog,
	}
}

// GetProviderCategory returns the canonical category registered for a kind.
// Unknown kinds are rejected instead of silently defaulting to OAuth.
func GetProviderCategory(kind Kind) (ProviderCategory, error) {
	categories, err := ProviderCategoriesForKind(kind)
	if err != nil {
		return "", err
	}
	return categories[0], nil
}

// validateCategory enforces that the stored category is one the provider's kind
// may use. It resolves the kind from Kind when present (the control plane
// always sets it before activation) and otherwise falls back to the
// lower-cased ProviderType so legacy detail rows still validate.
func (p *Provider) validateCategory() error {
	if p.Category == "" {
		return nil
	}
	if !p.Category.Valid() {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"unknown provider category: "+string(p.Category),
			map[string]any{
				"field":   "category",
				"value":   string(p.Category),
				"allowed": enumStrings(allProviderCategories),
			},
		)
	}
	kind := p.Kind
	if kind == "" {
		kind = Kind(strings.ToLower(strings.TrimSpace(string(p.ProviderType))))
	}
	if kind == "" {
		return apperrors.NewInvalidArgument("provider kind is required")
	}
	allowed, err := ProviderCategoriesForKind(kind)
	if err != nil {
		return err
	}
	for _, category := range allowed {
		if p.Category == category {
			return nil
		}
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		fmt.Sprintf("provider category %s does not match kind %s (allowed %s)", p.Category, kind, strings.Join(enumStrings(allowed), ", ")),
		map[string]any{
			"field":   "category",
			"value":   string(p.Category),
			"kind":    string(kind),
			"allowed": enumStrings(allowed),
		},
	)
}

// ValidateEmailRegex rejects a pattern that cannot be compiled. Callers that
// only run the registration validation (the create/patch strategy) must call
// this explicitly so the failure surfaces on write, not at the next login.
func (p *Provider) ValidateEmailRegex() error {
	if p == nil {
		return nil
	}
	regex := strings.TrimSpace(p.EmailRegex)
	if regex == "" {
		return nil
	}
	if _, err := regexp.Compile(regex); err != nil {
		return apperrors.NewInvalidArgumentf("emailRegex is invalid: %v", err)
	}
	return nil
}

// Validate checks the provider fields for business rule violations.
func (p *Provider) Validate() error {
	if p.Name == "" {
		return apperrors.NewInvalidArgument("provider name is required")
	}
	// Category+kind consistency (single source: providerKindCategories).
	if err := p.validateCategory(); err != nil {
		return err
	}
	// emailRegex is consumed at social-login time, so an invalid pattern must
	// be rejected on write instead of turning every login into a 500.
	if err := p.ValidateEmailRegex(); err != nil {
		return err
	}
	// Required fields based on category
	switch p.Category {
	case CategoryOAuth, CategoryOIDC:
		if p.ClientID == "" {
			return apperrors.NewInvalidArgument("clientId is required for OAuth/OIDC providers")
		}
		// Generic OIDC discovers its endpoints from the issuer, so authUrl and
		// tokenUrl are optional when an issuer is configured.
		discoveryIssuer := strings.EqualFold(string(p.ProviderType), string(ProviderOIDC)) &&
			(strings.TrimSpace(p.IssuerUrl) != "" || strings.TrimSpace(p.Host) != "")
		if p.AuthUrl == "" && !discoveryIssuer {
			return apperrors.NewInvalidArgument("authUrl is required for OAuth/OIDC providers")
		}
		if p.TokenUrl == "" && !discoveryIssuer {
			return apperrors.NewInvalidArgument("tokenUrl is required for OAuth/OIDC providers")
		}
	case CategorySAML:
		if p.IdPEntityId == "" {
			return apperrors.NewInvalidArgument("idpEntityId is required for SAML providers")
		}
	case CategoryLDAP:
		if p.Host == "" {
			return apperrors.NewInvalidArgument("host is required for LDAP providers")
		}
	case CategoryAudit:
		// A syslog sink needs an address; the protocol/facility/format keep
		// their defaults when blank.
		if strings.TrimSpace(p.Host) == "" {
			return apperrors.NewInvalidArgument("host is required for audit providers")
		}
		if p.Port <= 0 || p.Port > 65535 {
			return apperrors.NewInvalidArgumentf("port %d is out of range for audit providers", p.Port)
		}
	case CategoryStorage:
		switch p.ProviderType {
		case ProviderAWSS3, ProviderAzureBlob, ProviderAliOSS, ProviderTencentCOS,
			ProviderHuaweiOBS, ProviderCUCloud, ProviderMinIO, ProviderGCS, ProviderQiniu,
			ProviderCloudflareR2, ProviderBackblazeB2, ProviderDigitalOceanSpaces:
			if strings.TrimSpace(p.AppId) == "" {
				return apperrors.NewInvalidArgument("bucket/container is required for storage providers")
			}
			if strings.TrimSpace(p.Admin) == "" || strings.TrimSpace(p.Password) == "" {
				return apperrors.NewInvalidArgument("accessKeyId and accessKeySecret are required for storage providers")
			}
		}
	case CategorySocial, CategoryEnterprise, CategoryWeb3, CategoryCustom, CategoryPayment:
		// 这些分类无额外必填字段（OAuth/OIDC 与 SAML/LDAP 的校验见上）
	}
	return nil
}

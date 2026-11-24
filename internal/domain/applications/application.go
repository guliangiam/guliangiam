package applications

import (
	"strings"
	"time"
)

const (
	TokenEndpointAuthMethodNone              = "none"
	TokenEndpointAuthMethodClientSecretBasic = "client_secret_basic"
	TokenEndpointAuthMethodClientSecretPost  = "client_secret_post"

	// TokenSigningMethodRS256 is the default and the historical algorithm.
	TokenSigningMethodRS256 = "RS256"
	// TokenSigningMethodSM2SM3 signs with SM2 over an SM3 digest.
	TokenSigningMethodSM2SM3 = "SM2-SM3"
)

type Application struct {
	ID                        string
	OrganizationID            string
	Slug                      string
	DisplayName               string
	Category                  string
	AppType                   string
	Description               string
	HomepageURL               string
	Domain                    string
	Status                    string
	ClientID                  string
	ClientSecretHash          string
	TokenEndpointAuthMethod   string
	RedirectURIs              []string
	GrantTypes                []string
	DisableSignin             bool
	Tags                      []string
	EnableExclusiveSignin     bool
	EnableSigninSession       bool
	Logo                      string
	EnableSamlCompress        bool
	SignupItems               []SignupItem
	TokenFormat               string
	ExpireInHours             int
	RefreshTokenExpireInHours int
	TokenSigningMethod        string
	ProviderItems             []ProviderItem
	SamlReplyUrl              string
	Certificate               string
	CustomCss                 string
	// CustomJs 脚本能力已废弃：列保留兼容存量数据，无写入路径，任何投影都不暴露。
	CustomJs          string
	CustomHeader      string
	SigninUrl         string
	LogoutUrl         string
	LogoutRedirectUrl string
	IsPortal          bool
	IsVisible         bool
	PromptUrl         string
	SamlMetadata      string
	// Phase 2: Extended fields
	IsShared                          bool
	Title                             string
	Favicon                           string
	MenuOrder                         int
	MenuMode                          string
	EnableSignup                      bool
	EnableGuestSignin                 bool
	EnableAutoSignin                  bool
	EnableLinkWithEmail               bool
	CookieExpireInHours               int
	SignupUrl                         string
	ForgetUrl                         string
	AffiliationUrl                    string
	TokenFields                       string
	TokenAttributes                   []TokenAttribute
	ForcedRedirectOrigin              string
	CodeExpire                        int
	SamlNameIdFormat                  string
	SamlNameIdContent                 string
	SamlSignRequest                   bool
	SamlMetadataUrl                   string
	SamlAttributes                    []SamlAttribute
	EnableSamlC14n10                  bool
	EnableSamlPostBinding             bool
	DisableSamlAttributes             bool
	EnableSamlAssertionSignature      bool
	UseEmailAsSamlNameId              bool
	ConsentMode                       string
	DpopRequired                      bool
	PkceRequired                      bool
	IpWhitelist                       string
	IpBlacklist                       string
	FailedSigninLimit                 int
	FailedSigninFrozenTime            int
	CodeResendTimeout                 int
	CorsAllowedOrigins                string
	EnableBetaOidc                    bool
	DefaultGroup                      string
	OrgChoiceMode                     string
	FormCss                           string
	FormCssMobile                     string
	FormSideHtml                      string
	FormBackgroundUrl                 string
	FormBackgroundUrlMobile           string
	FormOffset                        string
	FormPosition                      string
	HeaderHtml                        string
	FooterHtml                        string
	SignupHtml                        string
	SigninHtml                        string
	TermsOfUse                        string
	PrivacyPolicy                     string
	BackchannelLogoutUri              string
	FrontchannelLogoutUri             string
	FrontchannelLogoutSessionRequired bool
	OtherDomains                      []string
	UpstreamHost                      string
	SslMode                           string
	SslCert                           string
	ClientCert                        string
	SamlHashAlgorithm                 string
	SamlC14nPrefix                    string
	PageHtml                          string
	SigninMethods                     []string
	SigninItems                       []SigninItemConfig
	CreatedAt                         time.Time
	UpdatedAt                         time.Time
	DeletedAt                         *time.Time
	Revision                          int64
	CustomScopes                      []ApplicationCustomScope
	ProviderBindings                  []ApplicationProviderBinding
}

// SignupItem controls which fields are shown during user registration.
type SignupItem struct {
	Name        string   `json:"name"`
	Type        string   `json:"type,omitempty"`
	Visible     bool     `json:"visible"`
	Required    bool     `json:"required"`
	Prompted    bool     `json:"prompted"`
	Rule        string   `json:"rule,omitempty"`
	Regex       string   `json:"regex,omitempty"`
	Options     []string `json:"options,omitempty"`
	Label       string   `json:"label,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	CustomCss   string   `json:"customCss,omitempty"`
}

// ProviderItem configures how a provider is used within an application.
type ProviderItem struct {
	Provider     string   `json:"provider"`
	CanSignUp    bool     `json:"canSignUp"`
	CanSignIn    bool     `json:"canSignIn"`
	CanUnlink    bool     `json:"canUnlink"`
	AlertType    string   `json:"alertType,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`
	BindingRule  []string `json:"bindingRule,omitempty"`
	CountryCodes []string `json:"countryCodes,omitempty"`
	Prompted     bool     `json:"prompted,omitempty"`
	SignupGroup  string   `json:"signupGroup,omitempty"`
	Rule         string   `json:"rule,omitempty"`
}

type ApplicationCustomScope struct {
	Scope       string
	DisplayName string
	Description string
}

type TokenAttribute struct {
	Name        string `json:"name"`
	SourceField string `json:"sourceField"`
	Transform   string `json:"transform,omitempty"`
}

type SamlAttribute struct {
	Name       string `json:"name"`
	NameFormat string `json:"nameFormat,omitempty"`
	Value      string `json:"value"`
}

type SigninMethod struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Rule        string `json:"rule,omitempty"`
}

type SigninItemConfig struct {
	Name        string `json:"name"`
	Visible     bool   `json:"visible"`
	Label       string `json:"label,omitempty"`
	CustomCss   string `json:"customCss,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Rule        string `json:"rule,omitempty"`
	IsCustom    bool   `json:"isCustom,omitempty"`
}

// EffectiveTokenEndpointAuthMethod 返回已注册的认证方式。
//
// 历史数据可能把 method 留空、仅在 app_oauth_config.client_secret_hash 写入哈希：
// 此时按“有哈希 → client_secret_basic，无哈希 → none”做只读兼容。
// 该列为待弃用（TODO(deprecate)），新写入一律进入 app_client_secrets。
func (a Application) EffectiveTokenEndpointAuthMethod() string {
	method := strings.ToLower(strings.TrimSpace(a.TokenEndpointAuthMethod))
	if IsRegisteredTokenEndpointAuthMethod(method) {
		return method
	}
	if strings.TrimSpace(a.ClientSecretHash) != "" {
		return TokenEndpointAuthMethodClientSecretBasic
	}
	return TokenEndpointAuthMethodNone
}

// RequiresClientSecret 报告该方式是否以 HTTP 请求中的 client_secret 作为凭据
// （client_secret_basic / client_secret_post）。JWT 与 mTLS 方式的凭据不在
// 请求体或 Basic 头里，不能复用本判断。
func (a Application) RequiresClientSecret() bool {
	switch a.EffectiveTokenEndpointAuthMethod() {
	case TokenEndpointAuthMethodClientSecretBasic, TokenEndpointAuthMethodClientSecretPost:
		return true
	default:
		return false
	}
}

// IsConfidential returns true if the application is not a public client.
func (a Application) IsConfidential() bool {
	return a.EffectiveTokenEndpointAuthMethod() != TokenEndpointAuthMethodNone
}

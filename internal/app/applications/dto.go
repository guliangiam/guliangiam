package applications

import (
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

type ApplicationListItem struct {
	ID             string   `json:"id"`
	OrganizationID string   `json:"organizationId"`
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Owner          string   `json:"owner"`
	DisplayName    string   `json:"displayName"`
	Category       string   `json:"category"`
	AppType        string   `json:"appType"`
	Domain         string   `json:"domain"`
	Status         string   `json:"status"`
	Tags           []string `json:"tags"`
	Logo           string   `json:"logo"`
	HomepageURL    string   `json:"homepageUrl"`
	IsVisible      bool     `json:"isVisible"`
	// Certificate 是应用签发 JWT 使用的证书名（与详情同值）；列表投影携带它
	// 是为了支持证书详情页的"关联应用"反查，不携带任何密钥材料。
	Certificate string    `json:"certificate"`
	ProviderIDs []string  `json:"providerIds"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ApplicationDetail struct {
	ApplicationListItem
	Description             string `json:"description"`
	ClientID                string `json:"clientId"`
	TokenEndpointAuthMethod string `json:"tokenEndpointAuthMethod"`
	// ClientSecretSet / ClientSecretMasked 是凭据的只读投影：
	// 明文只在创建/轮换响应出现一次，读接口永远不回显哈希或密文。
	ClientSecretSet           bool                         `json:"clientSecretSet"`
	ClientSecretMasked        string                       `json:"clientSecretMasked"`
	RedirectURIs              []string                     `json:"redirectUris"`
	GrantTypes                []string                     `json:"grantTypes"`
	DisableSignin             bool                         `json:"disableSignin"`
	EnableSigninSession       bool                         `json:"enableSigninSession"`
	EnablePassword            bool                         `json:"enablePassword"`
	EnablePasswordLogin       bool                         `json:"enablePasswordLogin"`
	EnableCodeSignin          bool                         `json:"enableCodeSignin"`
	EnableSamlCompress        bool                         `json:"enableSamlCompress"`
	SignupItems               []domain.SignupItem          `json:"signupItems"`
	TokenFormat               string                       `json:"tokenFormat"`
	ExpireInHours             int                          `json:"expireInHours"`
	RefreshTokenExpireInHours int                          `json:"refreshTokenExpireInHours"`
	TokenSigningMethod        string                       `json:"tokenSigningMethod"`
	ProviderItems             []domain.ProviderItem        `json:"providerItems"`
	SamlReplyUrl              string                       `json:"samlReplyUrl"`
	Certificate               string                       `json:"certificate"`
	CustomCss                 string                       `json:"customCss"`
	CustomHeader              string                       `json:"customHeader"`
	SigninUrl                 string                       `json:"signinUrl"`
	LogoutUrl                 string                       `json:"logoutUrl"`
	LogoutRedirectUrl         string                       `json:"logoutRedirectUrl"`
	IsPortal                  bool                         `json:"isPortal"`
	PromptUrl                 string                       `json:"promptUrl"`
	SamlMetadata              string                       `json:"samlMetadata"`
	CustomScopes              []ApplicationCustomScopeItem `json:"customScopes"`
	Revision                  int64                        `json:"revision"`
	// Phase 2: Extended fields
	IsShared                          bool                      `json:"isShared"`
	Title                             string                    `json:"title"`
	Favicon                           string                    `json:"favicon"`
	MenuOrder                         int                       `json:"menuOrder"`
	MenuMode                          string                    `json:"menuMode"`
	EnableSignup                      bool                      `json:"enableSignup"`
	EnableExclusiveSignin             bool                      `json:"enableExclusiveSignin"`
	EnableGuestSignin                 bool                      `json:"enableGuestSignin"`
	EnableAutoSignin                  bool                      `json:"enableAutoSignin"`
	EnableLinkWithEmail               bool                      `json:"enableLinkWithEmail"`
	EnableWebAuthn                    bool                      `json:"enableWebAuthn"`
	EnableDingtalk                    bool                      `json:"enableDingtalk"`
	EnableWechat                      bool                      `json:"enableWechat"`
	EnableLark                        bool                      `json:"enableLark"`
	EnableCas                         bool                      `json:"enableCas"`
	CookieExpireInHours               int                       `json:"cookieExpireInHours"`
	SignupUrl                         string                    `json:"signupUrl"`
	ForgetUrl                         string                    `json:"forgetUrl"`
	AffiliationUrl                    string                    `json:"affiliationUrl"`
	TokenFields                       string                    `json:"tokenFields"`
	TokenAttributes                   []domain.TokenAttribute   `json:"tokenAttributes"`
	ForcedRedirectOrigin              string                    `json:"forcedRedirectOrigin"`
	CodeExpire                        int                       `json:"codeExpire"`
	SamlNameIdFormat                  string                    `json:"samlNameIdFormat"`
	SamlNameIdContent                 string                    `json:"samlNameIdContent"`
	SamlSignRequest                   bool                      `json:"samlSignRequest"`
	SamlMetadataUrl                   string                    `json:"samlMetadataUrl"`
	SamlAttributes                    []domain.SamlAttribute    `json:"samlAttributes"`
	EnableSamlC14n10                  bool                      `json:"enableSamlC14n10"`
	EnableSamlPostBinding             bool                      `json:"enableSamlPostBinding"`
	DisableSamlAttributes             bool                      `json:"disableSamlAttributes"`
	EnableSamlAssertionSignature      bool                      `json:"enableSamlAssertionSignature"`
	UseEmailAsSamlNameId              bool                      `json:"useEmailAsSamlNameId"`
	ConsentMode                       string                    `json:"consentMode"`
	DpopRequired                      bool                      `json:"dpopRequired"`
	PkceRequired                      bool                      `json:"pkceRequired"`
	IpWhitelist                       string                    `json:"ipWhitelist"`
	IpBlacklist                       string                    `json:"ipBlacklist"`
	FailedSigninLimit                 int                       `json:"failedSigninLimit"`
	FailedSigninFrozenTime            int                       `json:"failedSigninFrozenTime"`
	CodeResendTimeout                 int                       `json:"codeResendTimeout"`
	CorsAllowedOrigins                string                    `json:"corsAllowedOrigins"`
	EnableBetaOidc                    bool                      `json:"enableBetaOidc"`
	DefaultGroup                      string                    `json:"defaultGroup"`
	OrgChoiceMode                     string                    `json:"orgChoiceMode"`
	FormCss                           string                    `json:"formCss"`
	FormCssMobile                     string                    `json:"formCssMobile"`
	FormSideHtml                      string                    `json:"formSideHtml"`
	FormBackgroundUrl                 string                    `json:"formBackgroundUrl"`
	FormBackgroundUrlMobile           string                    `json:"formBackgroundUrlMobile"`
	FormOffset                        string                    `json:"formOffset"`
	FormPosition                      string                    `json:"formPosition"`
	HeaderHtml                        string                    `json:"headerHtml"`
	FooterHtml                        string                    `json:"footerHtml"`
	SignupHtml                        string                    `json:"signupHtml"`
	SigninHtml                        string                    `json:"signinHtml"`
	TermsOfUse                        string                    `json:"termsOfUse"`
	PrivacyPolicy                     string                    `json:"privacyPolicy"`
	BackchannelLogoutUri              string                    `json:"backchannelLogoutUri"`
	FrontchannelLogoutUri             string                    `json:"frontchannelLogoutUri"`
	FrontchannelLogoutSessionRequired bool                      `json:"frontchannelLogoutSessionRequired"`
	OtherDomains                      []string                  `json:"otherDomains"`
	UpstreamHost                      string                    `json:"upstreamHost"`
	SslMode                           string                    `json:"sslMode"`
	SslCert                           string                    `json:"sslCert"`
	ClientCert                        string                    `json:"clientCert"`
	SamlHashAlgorithm                 string                    `json:"samlHashAlgorithm"`
	SamlC14nPrefix                    string                    `json:"samlC14nPrefix"`
	PageHtml                          string                    `json:"pageHtml"`
	SigninMethods                     []string                  `json:"signinMethods"`
	SigninItems                       []domain.SigninItemConfig `json:"signinItems"`
	// ClientJWKS 是 app_client_jwks 的只读公钥投影（见 projectClientJWKS）。
	// 任何私钥成员都不会出现在这里，即便库中存在越界数据。
	ClientJWKS ClientJWKSView `json:"clientJwks"`
	// JWKSURI 是 private_key_jwt 的远端 JWKS 地址（app_client_jwks_uris）；
	// 仅内嵌 JWKS 时为空。与写路径 request.JWKSURI 同口径回显。
	JWKSURI string `json:"jwksUri,omitempty"`
}

// ClientJWKPublicView 是一把已登记公钥的白名单投影。
type ClientJWKPublicView struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// ClientJWKSView 是详情响应的 clientJwks 字段；无密钥时返回空数组而非 null。
type ClientJWKSView struct {
	Kids []string              `json:"kids"`
	Keys []ClientJWKPublicView `json:"keys"`
}

// emptyClientJWKSView 是“未查询/无密钥”的统一形状。
func emptyClientJWKSView() ClientJWKSView {
	return ClientJWKSView{Kids: []string{}, Keys: []ClientJWKPublicView{}}
}

type ApplicationDeleteSummary struct {
	ApplicationID        string `json:"applicationId"`
	Slug                 string `json:"slug"`
	DisplayName          string `json:"displayName"`
	ProviderBindingCount int64  `json:"providerBindingCount"`
	ActiveTokenCount     int64  `json:"activeTokenCount"`
	ActiveConsentCount   int64  `json:"activeConsentCount"`
}

// ApplicationPublicDetail is the anonymous pre-login discovery projection
// served by GET /api/v1/applications/by-client-id/{clientId} and
// GET /api/v1/applications/resolve. It carries only fields the web
// login/signup/consent shell actually consumes plus public branding and
// UI customization (CSS/HTML are sanitized client-side before rendering;
// script execution is not supported).
//
// SAML material (certificate, samlMetadata, samlReplyUrl), callback allowlists
// (redirectUris, backchannelLogoutUri, frontchannelLogoutUri), internal override
// URLs (signinUrl, logoutUrl, promptUrl), transport configuration (sslMode,
// sslCert, clientCert, upstreamHost), token policy, IP/CORS policy and the
// client secret hash must never be added here. An absent field is the fix;
// blanking a value is not. Management surfaces keep using ApplicationDetail.
type ApplicationPublicDetail struct {
	ID                      string                    `json:"id"`
	OrganizationID          string                    `json:"organizationId"`
	Slug                    string                    `json:"slug"`
	Name                    string                    `json:"name"`
	DisplayName             string                    `json:"displayName"`
	Description             string                    `json:"description"`
	HomepageURL             string                    `json:"homepageUrl"`
	ClientID                string                    `json:"clientId"`
	Logo                    string                    `json:"logo"`
	Title                   string                    `json:"title"`
	Favicon                 string                    `json:"favicon"`
	IsShared                bool                      `json:"isShared"`
	EnableAutoSignin        bool                      `json:"enableAutoSignin"`
	EnableSignup            bool                      `json:"enableSignup"`
	ForcedRedirectOrigin    string                    `json:"forcedRedirectOrigin"`
	SignupURL               string                    `json:"signupUrl"`
	ForgetURL               string                    `json:"forgetUrl"`
	TermsOfUse              string                    `json:"termsOfUse"`
	PrivacyPolicy           string                    `json:"privacyPolicy"`
	OrgChoiceMode           string                    `json:"orgChoiceMode"`
	SigninHTML              string                    `json:"signinHtml"`
	SignupHTML              string                    `json:"signupHtml"`
	HeaderHTML              string                    `json:"headerHtml"`
	FooterHTML              string                    `json:"footerHtml"`
	CustomCSS               string                    `json:"customCss"`
	CustomHeader            string                    `json:"customHeader"`
	FormCSS                 string                    `json:"formCss"`
	FormCSSMobile           string                    `json:"formCssMobile"`
	FormSideHTML            string                    `json:"formSideHtml"`
	FormBackgroundURL       string                    `json:"formBackgroundUrl"`
	FormBackgroundURLMobile string                    `json:"formBackgroundUrlMobile"`
	FormOffset              string                    `json:"formOffset"`
	SigninMethods           []string                  `json:"signinMethods"`
	SigninItems             []domain.SigninItemConfig `json:"signinItems"`
	SignupItems             []domain.SignupItem       `json:"signupItems"`
}

type ApplicationCustomScopeItem struct {
	Scope       string `json:"scope"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

// ClientSecretState 是 secret 的只读投影，供详情映射使用。
type ClientSecretState struct {
	Set    bool
	Masked string
}

// CreateApplicationResult 携带一次性明文 secret。调用方必须在响应中返回
// ClientSecretPlain，并在任何日志/审计中排除它。
type CreateApplicationResult struct {
	Application        *domain.Application
	ClientSecretPlain  string
	ClientSecretPrefix string
}

type ApplicationProviderBindingItem struct {
	ProviderID string `json:"providerId"`
}

package idp

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/infra/saml"
)

// Factory creates IdP Provider instances based on Provider configuration.
type Factory struct{}

// NewFactory creates a new IdP factory.
func NewFactory() *Factory {
	return &Factory{}
}

// ProviderOptions carries per-call runtime values that are not persisted on
// the provider row (today: the PKCE verifier from the console callback plus the
// SAML SP-initiated callback context).
type ProviderOptions struct {
	CodeVerifier string
	// SAMLState is the RelayState echoed by the IdP. The AuthnRequest ID is
	// "_"+state, so the callback binds the response to the request we issued.
	SAMLState string
	// SAMLReplayStore records consumed Response/Assertion IDs. Nil fails closed.
	SAMLReplayStore saml.ReplayStore
	// SAMLReplayOrgID / SAMLReplayProviderID namespace replay keys.
	SAMLReplayOrgID      string
	SAMLReplayProviderID string
}

// GetProvider creates an IdP Provider based on the provider's category and type.
func (f *Factory) GetProvider(p *providers.Provider) (Provider, error) {
	return f.GetProviderWithOptions(p, ProviderOptions{})
}

// GetProviderWithOptions creates an IdP Provider with runtime options.
func (f *Factory) GetProviderWithOptions(p *providers.Provider, options ProviderOptions) (Provider, error) {
	if p == nil {
		return nil, apperrors.NewUnauthorized("provider is nil")
	}

	category := strings.ToLower(strings.TrimSpace(string(p.Category)))
	pType := strings.ToLower(strings.TrimSpace(string(p.ProviderType)))

	switch category {
	case "oauth", "idp", "oidc":
		return f.newOAuthProvider(pType, p, options)
	case "saml":
		config := &OAuthConfig{
			ClientID:     firstNonEmpty(p.IdPEntityId, p.ClientID),
			ClientSecret: p.ClientSecret,
			RedirectURI:  firstNonEmpty(p.CallbackUrl, p.ProviderURL),
			Host:         firstNonEmpty(p.SsoUrl, p.ProviderURL), // IdP SSO URL
			IdPCert:      firstNonEmpty(p.Cert, p.ClientSecret),
			IdPIssuer:    saml.IdPIssuerFromMetadata(p.Metadata),
		}
		config.SAMLExpectedAudience = config.ClientID
		config.SAMLExpectedRecipient = config.RedirectURI
		config.SAMLExpectedDestination = config.RedirectURI
		return NewSAMLIdPProviderWithOptions(config, options), nil
	default:
		return nil, apperrors.NewInvalidArgumentf("unsupported provider category: %s", category)
	}
}

func (f *Factory) newOAuthProvider(pType string, p *providers.Provider, options ProviderOptions) (Provider, error) {
	config := &OAuthConfig{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		RedirectURI:  firstNonEmpty(p.CallbackUrl, p.ProviderURL),
		Scopes:       append([]string{}, p.Scopes...),
		AuthURL:      p.AuthUrl,
		TokenURL:     p.TokenUrl,
		UserInfoURL:  p.UserInfoUrl,
		Host:         firstNonEmpty(p.IssuerUrl, p.Host), // Okta/Auth0/Casdoor/Gitea tenant domain
		AgentID:      p.AgentID,                          // WeCom internal application agentid
		HTTPClient:   providerHTTPClient(p),
		HTTPHeaders:  p.HttpHeaders,
		CodeVerifier: options.CodeVerifier,
	}

	switch pType {
	case "github":
		return NewGitHubProvider(config), nil
	case "google":
		return NewGoogleProvider(config), nil
	case "wechat":
		return NewWeChatProvider(config), nil
	case "dingtalk":
		return NewDingTalkProvider(config), nil
	case "lark":
		return NewLarkProvider(config), nil
	case "gitlab":
		return NewGitLabProvider(config), nil
	case "facebook":
		return NewFacebookProvider(config), nil
	case "linkedin":
		return NewLinkedInProvider(config), nil
	case "azuread", "azure":
		return NewAzureADProvider(config), nil
	case "custom":
		return NewCustomOAuthProvider(config), nil
	case "alipay":
		return NewAlipayProvider(config), nil
	case "twitter":
		return NewTwitterProvider(config), nil
	case "slack":
		return NewSlackProvider(config), nil
	case "discord":
		return NewDiscordProvider(config), nil
	case "wecom":
		return NewWeComProvider(config), nil
	case "infoflow":
		return NewInfoFlowProvider(config), nil
	case "web3", "metamask":
		return NewWeb3Provider(config), nil
	case "telegram":
		return NewTelegramProvider(config), nil
	case "line":
		return NewLineProvider(config), nil
	case "steam":
		return NewSteamProvider(config), nil
	case "reddit":
		return NewRedditProvider(config), nil
	case "apple":
		return NewAppleProvider(config), nil
	case "bitbucket":
		return NewBitbucketProvider(config), nil
	case "gitee":
		return NewGiteeProvider(config), nil
	case "weibo":
		return NewWeiboProvider(config), nil
	case "qq":
		return NewQQProvider(config), nil
	case "bilibili":
		return NewBilibiliProvider(config), nil
	case "douyin":
		return NewDouyinProvider(config), nil
	case "okta":
		return NewOktaProvider(config), nil
	case "auth0":
		return NewAuth0Provider(config), nil
	case "amazon":
		return NewAmazonProvider(config), nil
	case "wechat_miniprogram", "wechat-miniprogram":
		return NewWeChatMiniProgramProvider(config), nil
	case "wechat_mobile", "wechat-mobile":
		return NewWeChatMobileProvider(config), nil
	case "wecom_internal", "wecom-internal":
		return NewWeComInternalProvider(config), nil
	case "wecom_third_party", "wecom-third-party", "wecom_thirdparty":
		return NewWeComThirdPartyProvider(config), nil
	case "azuread_b2c", "azuread-b2c", "azureadb2c":
		return NewAzureADB2CProvider(config), nil
	case "adfs":
		return NewADFSProvider(config), nil
	case "baidu":
		return NewBaiduProvider(config), nil
	case "kwai":
		return NewKwaiProvider(config), nil
	case "casdoor":
		return NewCasdoorProvider(config), nil
	case "oidc":
		return NewOIDCProvider(config), nil
	default:
		// Try Goth framework for providers not natively implemented
		if IsGothProvider(pType) {
			gothProv, err := NewGothProvider(pType, config)
			if err == nil {
				if config.HTTPClient != nil {
					gothProv.SetHTTPClient(config.HTTPClient)
				}
				return gothProv, nil
			}
		}
		// For unknown types, try a generic OAuth2 provider.
		return NewCustomOAuthProvider(config), nil
	}
}

// GenerateState generates a random state string for CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

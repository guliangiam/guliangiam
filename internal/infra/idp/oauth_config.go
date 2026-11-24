package idp

import "net/http"

// OAuthConfig holds the common configuration for OAuth2-based IdP providers.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Host         string

	// Runtime provider options (enableProxy/httpHeaders). The client is built
	// per provider row and falls back to the SSRF-guarded default.
	HTTPClient  *http.Client
	HTTPHeaders map[string]string
	// CodeVerifier is the PKCE verifier for this callback, when the console
	// flow enabled PKCE for the provider.
	CodeVerifier string
	// IdPCert is the SAML IdP signing certificate (provider `cert`, falling
	// back to the legacy clientSecret column).
	IdPCert string
	// IdPIssuer is the expected SAML IdP entity ID (provider metadata/module).
	IdPIssuer string
	// AgentID is the WeCom application AgentID (required by the internal
	// application scan-to-login page; never reuse the corp secret here).
	AgentID string
	// SAML SP-side expectations. Empty values fail closed in the provider.
	SAMLExpectedAudience    string
	SAMLExpectedRecipient   string
	SAMLExpectedDestination string
}

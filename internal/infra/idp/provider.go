package idp

import "context"

// UserInfo contains the normalized user information returned by an IdP.
type UserInfo struct {
	ID          string `json:"id"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	OpenID      string `json:"openId,omitempty"`
	UnionID     string `json:"unionId,omitempty"`
	// AuthnContext is the SAML AuthnContextClassRef (acr) of a verified assertion.
	AuthnContext string `json:"authnContext,omitempty"`
}

// OAuthToken contains the OAuth token information from the IdP.
type OAuthToken struct {
	AccessToken  string `json:"accessToken"`
	TokenType    string `json:"tokenType,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int64  `json:"expiresIn,omitempty"`
	Scope        string `json:"scope,omitempty"`
	// ID carries the user identifier from the token exchange (e.g., OpenID for WeChat).
	ID string `json:"id,omitempty"`
	// IDToken is the OpenID Connect ID token (JWT) returned next to the access
	// token. OIDC adapters verify it against the issuer JWKS and read the user
	// claims from it.
	IDToken string `json:"idToken,omitempty"`
	// AuthnContext carries the verified SAML AuthnContextClassRef so the app
	// layer can set the session authentication level (amr). OAuth adapters leave
	// it empty.
	AuthnContext string `json:"authnContext,omitempty"`
	// OpenID and UnionID are WeChat-specific identifiers.
	OpenID  string `json:"openId,omitempty"`
	UnionID string `json:"unionId,omitempty"`
}

// AuthorizationRequestOptions carries per-request authorization parameters
// that are not persisted on the provider row. Generic OIDC uses them to bind
// the id_token nonce and the PKCE S256 challenge to the state.
type AuthorizationRequestOptions struct {
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// AuthorizationRequestProvider is the optional capability of adapters that
// accept per-request authorization options. BaseOAuthProvider implements it,
// so embedded adapters inherit a no-op default; only generic OIDC passes
// non-empty options today.
type AuthorizationRequestProvider interface {
	GetRedirectURLWithOptions(state string, options AuthorizationRequestOptions) (string, error)
}

// NonceBinder is implemented by adapters that verify an id_token nonce issued
// at authorization time. The bootstrap binds the stored nonce before the
// callback exchanges the code.
type NonceBinder interface {
	SetExpectedNonce(nonce string)
}

// Provider is the interface that all IdP adapters must implement.
type Provider interface {
	// GetRedirectURL returns the URL to redirect the user to for authentication.
	GetRedirectURL(state string) (string, error)

	// ExchangeCode exchanges the authorization code for an OAuth token.
	ExchangeCode(ctx context.Context, code string) (*OAuthToken, error)

	// GetUserInfo retrieves the user information using the OAuth token.
	GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error)
}

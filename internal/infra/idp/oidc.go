package idp

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

const oidcWellKnownPath = "/.well-known/openid-configuration"

// oidcDiscoveryTTL bounds how long a discovered document is reused. It is a
// variable so tests can shorten it.
var oidcDiscoveryTTL = 10 * time.Minute

// oidcHTTPClient performs discovery and JWKS requests. It is a variable so
// tests can inject an allowlist client; production keeps the SSRF-guarded one.
var oidcHTTPClient = ssrf.DefaultHTTPClient

// oidcDiscoveryDocument is the subset of OpenID Connect Discovery 1.0 the
// adapter consumes.
type oidcDiscoveryDocument struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	ScopesSupported       []string `json:"scopes_supported"`
}

type oidcDiscoveryEntry struct {
	document  *oidcDiscoveryDocument
	fetchedAt time.Time
}

var oidcDiscoveryCache = struct {
	mu      sync.RWMutex
	entries map[string]oidcDiscoveryEntry
}{entries: make(map[string]oidcDiscoveryEntry)}

// OIDCProvider implements OpenID Connect on top of the generic OAuth2 base.
// Endpoints come from explicit provider columns when set, otherwise from the
// issuer's discovery document. ID tokens are verified against the issuer JWKS
// before their claims are trusted.
type OIDCProvider struct {
	BaseOAuthProvider
	issuer        string
	expectedNonce string
	mu            sync.Mutex
}

// NewOIDCProvider builds an OIDC adapter. Host carries the issuer URL; explicit
// auth/token/userinfo URLs win over discovery.
func NewOIDCProvider(config *OAuthConfig) *OIDCProvider {
	if config == nil {
		config = &OAuthConfig{}
	}
	return &OIDCProvider{
		BaseOAuthProvider: newBaseOAuth(config, config.AuthURL, config.TokenURL, config.UserInfoURL),
		issuer:            normalizeIssuer(config.Host),
	}
}

func (p *OIDCProvider) GetRedirectURL(state string) (string, error) {
	return p.GetRedirectURLWithOptions(state, AuthorizationRequestOptions{})
}

// GetRedirectURLWithOptions resolves discovery endpoints before adding the
// nonce and PKCE parameters supplied by the caller.
func (p *OIDCProvider) GetRedirectURLWithOptions(state string, options AuthorizationRequestOptions) (string, error) {
	if err := p.ensureEndpoints(context.Background()); err != nil {
		return "", err
	}
	return p.BaseOAuthProvider.GetRedirectURLWithOptions(state, options)
}

// SetExpectedNonce binds the nonce issued at authorization time to this
// callback adapter. GetUserInfo refuses an id_token whose nonce is missing or
// different instead of falling back to the userinfo endpoint.
func (p *OIDCProvider) SetExpectedNonce(nonce string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expectedNonce = strings.TrimSpace(nonce)
}

func (p *OIDCProvider) currentExpectedNonce() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.expectedNonce
}

func (p *OIDCProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	if err := p.ensureEndpoints(ctx); err != nil {
		return nil, err
	}
	return p.BaseOAuthProvider.ExchangeCode(ctx, code)
}

func (p *OIDCProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	if token == nil {
		return nil, apperrors.NewUnauthorized("OIDC token is nil")
	}

	expectedNonce := p.currentExpectedNonce()
	hasIDToken := strings.TrimSpace(token.IDToken) != ""

	// The ID token is the authoritative identity assertion when present; verify
	// it before trusting claims. Without an issuer there is nothing to verify
	// against, so such a row falls back to the userinfo endpoint.
	if hasIDToken && p.issuer != "" {
		document, err := p.document(ctx)
		if err != nil {
			return nil, err
		}
		claims, err := verifyOIDCIDToken(ctx, token.IDToken, document.Issuer, p.config.ClientID, expectedNonce, func(ctx context.Context, kid string) (*rsa.PublicKey, error) {
			if strings.TrimSpace(document.JWKSURI) == "" {
				return nil, apperrors.NewInvalidArgument("OIDC discovery did not provide jwks_uri")
			}
			return oidcVerificationKey(ctx, document.JWKSURI, kid)
		})
		if err != nil {
			return nil, err
		}
		return userInfoFromOIDCClaims(claims)
	}

	// A nonce was issued for this authorization request, so skipping its
	// verification would downgrade the login to an unbound assertion.
	if expectedNonce != "" {
		if !hasIDToken {
			return nil, apperrors.NewUnauthorized("id_token is required to verify the OIDC nonce")
		}
		return nil, apperrors.NewUnauthorized("OIDC issuer is required to verify the id_token nonce")
	}

	if err := p.ensureEndpoints(ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.userURL) == "" {
		return nil, apperrors.NewInvalidArgument("OIDC provider returned no id_token and has no userinfo endpoint")
	}
	return p.BaseOAuthProvider.GetUserInfo(ctx, token)
}

// ensureEndpoints fills the missing endpoints from discovery. Explicit columns
// are never overwritten, so an admin can override a single broken endpoint.
func (p *OIDCProvider) ensureEndpoints(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.authURL != "" && p.tokenURL != "" && p.userURL != "" {
		return nil
	}
	if p.issuer == "" {
		return apperrors.NewInvalidArgument("OIDC issuer is required")
	}

	document, err := discoverOIDC(ctx, p.issuer)
	if err != nil {
		return err
	}
	if p.authURL == "" {
		p.authURL = document.AuthorizationEndpoint
	}
	if p.tokenURL == "" {
		p.tokenURL = document.TokenEndpoint
	}
	if p.userURL == "" {
		p.userURL = document.UserinfoEndpoint
	}
	if p.authURL == "" || p.tokenURL == "" {
		return apperrors.NewInvalidArgument("OIDC discovery did not provide authorization/token endpoints")
	}
	return nil
}

func (p *OIDCProvider) document(ctx context.Context) (*oidcDiscoveryDocument, error) {
	if p.issuer == "" {
		return nil, apperrors.NewInvalidArgument("OIDC issuer is required to verify the id_token")
	}
	return discoverOIDC(ctx, p.issuer)
}

func discoverOIDC(ctx context.Context, issuer string) (*oidcDiscoveryDocument, error) {
	normalized := normalizeIssuer(issuer)
	if normalized == "" {
		return nil, apperrors.NewInvalidArgument("OIDC issuer is required")
	}

	oidcDiscoveryCache.mu.RLock()
	entry, cached := oidcDiscoveryCache.entries[normalized]
	oidcDiscoveryCache.mu.RUnlock()
	if cached && time.Since(entry.fetchedAt) < oidcDiscoveryTTL {
		return entry.document, nil
	}

	document, err := fetchOIDCDiscovery(ctx, normalized)
	if err != nil {
		return nil, err
	}

	oidcDiscoveryCache.mu.Lock()
	oidcDiscoveryCache.entries[normalized] = oidcDiscoveryEntry{document: document, fetchedAt: time.Now().UTC()}
	oidcDiscoveryCache.mu.Unlock()
	return document, nil
}

func fetchOIDCDiscovery(ctx context.Context, issuer string) (*oidcDiscoveryDocument, error) {
	if err := ssrf.ValidateURL(issuer); err != nil {
		return nil, apperrors.WrapInvalidArgumentf(err, "invalid OIDC issuer")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+oidcWellKnownPath, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInvalidArgumentf(err, "build OIDC discovery request")
	}
	request.Header.Set("Accept", "application/json")

	response, err := oidcHTTPClient.Do(request)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "fetch OIDC discovery document")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, apperrors.NewUnavailablef("OIDC discovery failed (status %d)", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "read OIDC discovery document")
	}

	var document oidcDiscoveryDocument
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse OIDC discovery document")
	}

	// Discovery 1.0 §4.3: the document's issuer must be identical to the one
	// used to build the well-known URL, otherwise a compromised document could
	// redirect token verification to a different tenant.
	if normalizeIssuer(document.Issuer) != issuer {
		return nil, apperrors.NewInvalidArgument("OIDC discovery issuer does not match the configured issuer")
	}
	if document.AuthorizationEndpoint == "" || document.TokenEndpoint == "" {
		return nil, apperrors.NewInvalidArgument("OIDC discovery did not provide authorization/token endpoints")
	}
	for _, endpoint := range []string{document.AuthorizationEndpoint, document.TokenEndpoint, document.UserinfoEndpoint, document.JWKSURI} {
		if endpoint == "" {
			continue
		}
		if err := ssrf.ValidateURL(endpoint); err != nil {
			return nil, apperrors.WrapInvalidArgumentf(err, "invalid OIDC endpoint %s", endpoint)
		}
	}
	return &document, nil
}

func normalizeIssuer(issuer string) string {
	return strings.TrimRight(strings.TrimSpace(issuer), "/")
}

// verifyOIDCIDToken validates an RS256 ID token and returns its claims per
// OpenID Connect Core 1.0 §3.1.3.7 items 1-12: signature, iss, aud (client_id
// present, single-audience strictness), exp (and iat freshness left to exp),
// and nonce when one was issued. keyLookup resolves the signing key for the
// token's kid; it is injected so tests can verify without HTTP. A non-empty
// expectedNonce must match the token's nonce claim exactly; a missing claim is
// rejected.
func verifyOIDCIDToken(ctx context.Context, rawToken, expectedIssuer, clientID, expectedNonce string, keyLookup func(context.Context, string) (*rsa.PublicKey, error)) (map[string]interface{}, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, apperrors.NewUnauthorized("invalid id_token format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid id_token header encoding")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, apperrors.NewUnauthorized("invalid id_token header")
	}
	if header.Alg != "RS256" {
		return nil, apperrors.NewUnauthorized("id_token must be signed with RS256")
	}
	if strings.TrimSpace(header.Kid) == "" {
		return nil, apperrors.NewUnauthorized("id_token header is missing kid")
	}

	publicKey, err := keyLookup(ctx, header.Kid)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "resolve id_token signing key")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid id_token signature encoding")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, apperrors.NewUnauthorized("id_token signature verification failed")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid id_token payload encoding")
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, apperrors.NewUnauthorized("invalid id_token claims")
	}

	if normalizeIssuer(stringClaim(claims, "iss")) != normalizeIssuer(expectedIssuer) {
		return nil, apperrors.NewUnauthorized("id_token issuer does not match the configured issuer")
	}
	if clientID != "" && !containsString(oidcAudiences(claims["aud"]), clientID) {
		return nil, apperrors.NewUnauthorized("id_token audience does not match the client id")
	}
	expiresAt, ok := claims["exp"].(float64)
	if !ok {
		return nil, apperrors.NewUnauthorized("id_token is missing exp")
	}
	if !time.Unix(int64(expiresAt), 0).After(time.Now().UTC()) {
		return nil, apperrors.NewUnauthorized("id_token has expired")
	}
	if expectedNonce != "" {
		actualNonce := stringClaim(claims, "nonce")
		if actualNonce == "" {
			return nil, apperrors.NewUnauthorized("id_token is missing nonce")
		}
		if subtle.ConstantTimeCompare([]byte(actualNonce), []byte(expectedNonce)) != 1 {
			return nil, apperrors.NewUnauthorized("id_token nonce does not match")
		}
	}
	return claims, nil
}

func userInfoFromOIDCClaims(claims map[string]interface{}) (*UserInfo, error) {
	subject := stringClaim(claims, "sub")
	if subject == "" {
		return nil, apperrors.NewUnauthorized("id_token is missing sub")
	}
	return &UserInfo{
		ID:          subject,
		Username:    stringClaim(claims, "preferred_username"),
		DisplayName: stringClaim(claims, "name"),
		Email:       stringClaim(claims, "email"),
		Phone:       stringClaim(claims, "phone_number"),
		AvatarURL:   stringClaim(claims, "picture"),
	}, nil
}

func stringClaim(claims map[string]interface{}, key string) string {
	value, ok := claims[key].(string)
	if !ok {
		return ""
	}
	return value
}

func oidcAudiences(raw interface{}) []string {
	switch audience := raw.(type) {
	case string:
		return []string{audience}
	case []interface{}:
		result := make([]string, 0, len(audience))
		for _, item := range audience {
			if value, ok := item.(string); ok {
				result = append(result, value)
			}
		}
		return result
	default:
		return nil
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var (
	oidcJWKSOnce sync.Once
	oidcJWKS     *platformjwks.RemoteJWKSCache
)

func oidcVerificationKey(ctx context.Context, jwksURI, kid string) (*rsa.PublicKey, error) {
	oidcJWKSOnce.Do(func() {
		oidcJWKS = platformjwks.NewRemoteJWKSCache(5*time.Minute, 30*time.Minute, oidcHTTPClient)
	})
	return oidcJWKS.GetPublicKey(ctx, jwksURI, kid)
}

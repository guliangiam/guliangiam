package idp

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AppleProvider implements IdP authentication via Apple Sign-In.
// Apple's OAuth2 client_secret is an ES256-signed JWT minted from the
// Service Key (TeamID/KeyID/PrivateKey); the console must supply it as
// ClientSecret because signing it here would require the raw private key.
// The authorize endpoint defaults to response_mode=form_post for the code
// flow; the mode is pinned explicitly so the callback contract cannot drift.
// There is no user info endpoint: the profile lives in the ID token.
// https://developer.apple.com/documentation/sign_in_with_apple/sign_in_with_apple_js/incorporating_sign_in_with_apple_into_other_platforms
type AppleProvider struct {
	BaseOAuthProvider
}

// NewAppleProvider creates a new Apple OAuth2 provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：端点与 form_post 回调契约已按上方官方文档复核。
func NewAppleProvider(config *OAuthConfig) *AppleProvider {
	if config != nil && len(config.Scopes) == 0 {
		config.Scopes = []string{"name", "email"}
	}
	return &AppleProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://appleid.apple.com/auth/authorize",
			"https://appleid.apple.com/auth/token",
			"",
		),
	}
}

// GetRedirectURL returns the Apple authorization URL with response_mode=form_post.
func (p *AppleProvider) GetRedirectURL(state string) (string, error) {
	return p.GetRedirectURLWithOptions(state, AuthorizationRequestOptions{})
}

// GetRedirectURLWithOptions appends the pinned response_mode to the base URL.
func (p *AppleProvider) GetRedirectURLWithOptions(state string, options AuthorizationRequestOptions) (string, error) {
	redirect, err := p.BaseOAuthProvider.GetRedirectURLWithOptions(state, options)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(redirect)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}
	q := u.Query()
	q.Set("response_mode", "form_post")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// appleIDTokenClaims are the Sign-In with Apple ID token claims. The `user`
// claim is a JSON string only present on the first authorization response.
type appleIDTokenClaims struct {
	Sub            string `json:"sub"`
	Email          string `json:"email"`
	EmailVerified  any    `json:"email_verified"`
	IsPrivateEmail any    `json:"is_private_email"`
	TransferSub    string `json:"transfer_sub"`
	User           string `json:"user"`
}

// GetUserInfo extracts the profile from the ID token returned by the token
// exchange. Signature verification is performed by the OIDC/JWT layer; this
// adapter only reads the claims payload.
func (p *AppleProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	if strings.TrimSpace(token.IDToken) == "" {
		return nil, apperrors.NewUnauthorized("apple token response did not include an id_token")
	}

	parts := strings.Split(token.IDToken, ".")
	if len(parts) != 3 {
		return nil, apperrors.NewUnauthorized("apple id_token is not a JWT")
	}
	payload, err := decodeBase64URL(parts[1])
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode id_token payload")
	}

	var claims appleIDTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse id_token claims")
	}
	if claims.Sub == "" {
		return nil, apperrors.NewUnauthorized("apple id_token is missing the sub claim")
	}

	info := &UserInfo{
		ID:      claims.Sub,
		OpenID:  claims.Sub,
		Email:   claims.Email,
		UnionID: claims.TransferSub,
	}

	// The name is only delivered with the first authorization (form_post body
	// or the transient `user` claim); parse it when present.
	if claims.User != "" {
		var userPayload struct {
			Name struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
			} `json:"name"`
		}
		if err := json.Unmarshal([]byte(claims.User), &userPayload); err == nil {
			displayName := strings.TrimSpace(userPayload.Name.FirstName + " " + userPayload.Name.LastName)
			if displayName != "" {
				info.DisplayName = displayName
				info.Username = displayName
			}
		}
	}

	return info, nil
}

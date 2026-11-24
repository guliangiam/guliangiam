package tokens

import (
	"context"
	"time"
)

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	UserID         string
	ApplicationID  string
	ActiveOnly     *bool
	Keyword        string
}

type RevokeQuery struct {
	OrganizationID string
	UserID         string
	ApplicationID  string
	// TokenPrefix restricts the revoke to tokens minted with this prefix
	// (e.g. "imp" for impersonation tokens). Empty means every token that
	// matches the other predicates.
	TokenPrefix string
}

type Repository interface {
	// GetByID is the system-scoped lookup used by the RFC 8693 exchange and the
	// refresh rotation path, which resolve an access token from material they
	// already hold rather than from a request-scoped id.
	GetByID(ctx context.Context, id string) (*Token, error)
	// GetByIDForOrganization returns the token only when the user it belongs to
	// belongs to organizationID. access_tokens has no organization_id column,
	// so the predicate is bound through the users join; a foreign or missing
	// id is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Token, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*Token, error)
	GetByDeviceSecretHash(ctx context.Context, deviceSecretHash string) (*Token, error)
	GetRefreshByTokenHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	List(ctx context.Context, query ListQuery) ([]*Token, int64, error)
	Create(ctx context.Context, token *Token) error
	// UpdateDeviceSecret persists the hashed device secret for a token issued
	// with the device_sso scope (Native SSO, RFC 8693 §4.1). The access token
	// row already exists by the time the secret is minted, so this is an UPDATE
	// rather than a second INSERT.
	UpdateDeviceSecret(ctx context.Context, tokenID, deviceSecretHash string, expiresIn int) error
	CreateRefresh(ctx context.Context, token *RefreshToken) error
	Revoke(ctx context.Context, id string, revokedAt time.Time) error
	RevokeRefresh(ctx context.Context, id string, revokedAt time.Time) error
	RevokeByQuery(ctx context.Context, query RevokeQuery, revokedAt time.Time) (int64, error)
	RevokeByFamilyID(ctx context.Context, familyID string, revokedAt time.Time) (int64, error)

	// RevokeRefreshByUser bulk-revokes every refresh token for a user. Used when
	// a credential changes (password reset, MFA add/remove, WebAuthn delete) —
	// AC-AUTH-009 requires the entire refresh family to die alongside sessions.
	RevokeRefreshByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error)

	// RevokeByAuthorizationCodeID revokes every token that was issued by redeeming
	// the given authorization code. It is the OAuth 2.1 response to authorization
	// code reuse: a second redemption means the code leaked, so everything the
	// first redemption handed out must stop working.
	RevokeByAuthorizationCodeID(ctx context.Context, authorizationCodeID string, revokedAt time.Time) (int64, error)

	Delete(ctx context.Context, id string) error
}

type IssueIDTokenParams struct {
	Issuer             string
	Algorithm          string
	Subject            string
	Audience           []string
	Nonce              string
	AccessTokenHash    string
	CodeHash           string
	AuthenticationTime time.Time
	// SessionID is the OIDC session identifier (设计报告 14.12.3). The logout
	// token carries the same value, which is the only thing that lets a relying
	// party connect an ID token to the logout event for it.
	SessionID     string
	JWTID         string
	Name          string
	Username      string
	GivenName     string
	FamilyName    string
	MiddleName    string
	Nickname      string
	Email         string
	EmailVerified *bool
	Phone         string
	PhoneVerified *bool
	Picture       string
	Organization  string
}

type IssueAccessTokenParams struct {
	Issuer         string
	Algorithm      string
	Subject        string
	Audience       []string
	ClientID       string
	OrganizationID string
	Username       string
	Scope          []string
	Resource       []string
	TTL            time.Duration
	JWTID          string
}

type TokenIssuer interface {
	IssueIDToken(params IssueIDTokenParams) (string, error)
	IssueAccessToken(params IssueAccessTokenParams) (string, error)
	ValidateToken(tokenString string) (map[string]interface{}, error)
}

type AccessTokenClaims struct {
	Issuer         string   `json:"iss"`
	Subject        string   `json:"sub"`
	Audience       []string `json:"aud"`
	Expiry         int64    `json:"exp"`
	IssuedAt       int64    `json:"iat"`
	JWTID          string   `json:"jti,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	ClientID       string   `json:"client_id,omitempty"`
	OrganizationID string   `json:"org_id,omitempty"`
	Username       string   `json:"username,omitempty"`
	Resource       []string `json:"resource,omitempty"`
}

type IDTokenClaims struct {
	Issuer             string   `json:"iss"`
	Subject            string   `json:"sub"`
	Audience           []string `json:"aud"`
	Expiry             int64    `json:"exp"`
	IssuedAt           int64    `json:"iat"`
	Nonce              string   `json:"nonce,omitempty"`
	AccessTokenHash    string   `json:"at_hash,omitempty"`
	CodeHash           string   `json:"c_hash,omitempty"`
	AuthenticationTime int64    `json:"auth_time,omitempty"`
	JWTID              string   `json:"jti,omitempty"`
	// SessionID is the `sid` claim (OIDC Back-Channel Logout 1.0 §2.4).
	SessionID     string `json:"sid,omitempty"`
	Name          string `json:"name,omitempty"`
	Username      string `json:"preferred_username,omitempty"`
	GivenName     string `json:"given_name,omitempty"`
	FamilyName    string `json:"family_name,omitempty"`
	MiddleName    string `json:"middle_name,omitempty"`
	Nickname      string `json:"nickname,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified *bool  `json:"email_verified,omitempty"`
	Phone         string `json:"phone_number,omitempty"`
	PhoneVerified *bool  `json:"phone_number_verified,omitempty"`
	Picture       string `json:"picture,omitempty"`
	Organization  string `json:"org,omitempty"`
}

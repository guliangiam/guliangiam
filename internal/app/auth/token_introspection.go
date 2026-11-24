package auth

import (
	"context"
	"strings"

	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// IntrospectTokenCommand contains parameters for token introspection per RFC 7662.
type IntrospectTokenCommand struct {
	Token     string // The token value to introspect
	TokenType string // Optional hint: "access_token" or oauthRefreshToken
	ClientID  string // Client identifier
}

// TokenIntrospectionResponse represents the introspection response per RFC 7662.
type TokenIntrospectionResponse struct {
	Active    bool     `json:"active"`
	Scope     string   `json:"scope,omitempty"`
	ClientID  string   `json:"client_id,omitempty"`
	Username  string   `json:"username,omitempty"`
	TokenType string   `json:"token_type,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
	IssuedAt  int64    `json:"iat,omitempty"`
	Sub       string   `json:"sub,omitempty"`
	Aud       []string `json:"aud,omitempty"`
	Iss       string   `json:"iss,omitempty"`
	JTI       string   `json:"jti,omitempty"`
	OrgID     string   `json:"org_id,omitempty"`
}

// IntrospectToken returns the active/inactive status and claims of a token per RFC 7662.
func (s *Service) IntrospectToken(ctx context.Context, command IntrospectTokenCommand) (*TokenIntrospectionResponse, error) {
	if strings.TrimSpace(command.Token) == "" {
		return &TokenIntrospectionResponse{Active: false}, nil
	}

	tokenValue := strings.TrimSpace(command.Token)

	// Try to introspect as access token first.
	tokenHash := platformcrypto.HashTokenValue(tokenValue)
	token, err := s.UOW.Tokens().GetByTokenHash(ctx, tokenHash)
	if err == nil {
		return s.introspectAccessToken(token), nil
	}

	// Try to introspect as refresh token.
	refreshToken, err := s.UOW.Tokens().GetRefreshByTokenHash(ctx, tokenHash)
	if err == nil {
		return s.introspectRefreshToken(refreshToken), nil
	}

	// Token not found or invalid.
	return &TokenIntrospectionResponse{Active: false}, nil
}

// introspectAccessToken builds the introspection response for an access token.
func (s *Service) introspectAccessToken(token *tokendomain.Token) *TokenIntrospectionResponse {
	now := s.Clock.Now().UTC()

	// Check if the token is active.
	active := token.RevokedAt == nil && token.ExpiresAt.After(now)

	if !active {
		return &TokenIntrospectionResponse{Active: false}
	}

	return &TokenIntrospectionResponse{
		Active:    true,
		Scope:     strings.Join(token.Scopes, " "),
		ClientID:  token.ApplicationSlug,
		Username:  token.Username,
		TokenType: tokenTypeBearer,
		ExpiresAt: token.ExpiresAt.Unix(),
		IssuedAt:  token.CreatedAt.Unix(),
		Sub:       token.UserID,
		Aud:       token.Audience,
		JTI:       token.ID,
		OrgID:     token.OrganizationID,
	}
}

// introspectRefreshToken builds the introspection response for a refresh token.
func (s *Service) introspectRefreshToken(token *tokendomain.RefreshToken) *TokenIntrospectionResponse {
	now := s.Clock.Now().UTC()

	// Check if the token is active.
	active := token.RevokedAt == nil && token.ExpiresAt.After(now)

	if !active {
		return &TokenIntrospectionResponse{Active: false}
	}

	return &TokenIntrospectionResponse{
		Active:    true,
		Scope:     strings.Join(token.Scopes, " "),
		ClientID:  token.ApplicationSlug,
		Username:  token.Username,
		TokenType: oauthRefreshToken,
		ExpiresAt: token.ExpiresAt.Unix(),
		IssuedAt:  token.CreatedAt.Unix(),
		Sub:       token.UserID,
		Aud:       token.Audience,
		JTI:       token.ID,
		OrgID:     token.OrganizationID,
	}
}

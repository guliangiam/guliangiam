package tokens

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Introspection struct {
	Active    bool
	Scope     string
	Username  string
	TokenType string
	Exp       int64
	Iat       int64
	Sub       string
	Aud       []string
	Jti       string
}

func (s *Service) IntrospectToken(ctx context.Context, command IntrospectTokenCommand) (*Introspection, error) {
	if strings.TrimSpace(command.Token) == "" {
		return nil, apperrors.NewInvalidArgument("token is required")
	}

	tokenTypeHint := strings.TrimSpace(command.TokenTypeHint)

	if tokenTypeHint == "" || tokenTypeHint == "access_token" {
		token, err := s.UOW.Tokens().GetByTokenHash(ctx, hashTokenValue(command.Token))
		if err != nil {
			if appErr, ok := err.(*apperrors.Error); ok && appErr.Code == apperrors.CodeNotFound {
				return &Introspection{Active: false}, nil
			}
			slog.ErrorContext(ctx, "tokens.IntrospectToken failed",
				"lookup_kind", "access_token",
				"error", err,
			)
			return nil, err
		}
		if !tokenBelongsToTenant(command.OrganizationID, token.OrganizationID) {
			return &Introspection{Active: false}, nil
		}

		now := s.Clock.Now()
		if token.RevokedAt != nil || !token.ExpiresAt.After(now) {
			return &Introspection{Active: false}, nil
		}

		return &Introspection{
			Active:    true,
			Scope:     strings.Join(token.Scopes, " "),
			Username:  token.Username,
			TokenType: "access_token",
			Exp:       token.ExpiresAt.Unix(),
			Iat:       token.CreatedAt.Unix(),
			Sub:       token.UserID,
			Aud:       append([]string(nil), token.Audience...),
			Jti:       token.ID,
		}, nil
	}

	if tokenTypeHint == "refresh_token" {
		refreshToken, err := s.UOW.Tokens().GetRefreshByTokenHash(ctx, hashTokenValue(command.Token))
		if err != nil {
			if appErr, ok := err.(*apperrors.Error); ok && appErr.Code == apperrors.CodeNotFound {
				return &Introspection{Active: false}, nil
			}
			slog.ErrorContext(ctx, "tokens.IntrospectToken failed",
				"lookup_kind", "refresh_token",
				"error", err,
			)
			return nil, err
		}
		if !tokenBelongsToTenant(command.OrganizationID, refreshToken.OrganizationID) {
			return &Introspection{Active: false}, nil
		}

		now := s.Clock.Now()
		if refreshToken.RevokedAt != nil || !refreshToken.ExpiresAt.After(now) {
			return &Introspection{Active: false}, nil
		}

		return &Introspection{
			Active:    true,
			Scope:     strings.Join(refreshToken.Scopes, " "),
			Username:  refreshToken.Username,
			TokenType: "refresh_token",
			Exp:       refreshToken.ExpiresAt.Unix(),
			Iat:       refreshToken.CreatedAt.Unix(),
			Sub:       refreshToken.UserID,
			Aud:       append([]string(nil), refreshToken.Audience...),
			Jti:       refreshToken.ID,
		}, nil
	}

	return &Introspection{Active: false}, nil
}

func hashTokenValue(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

package tokens

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const tokenPrefixMaxLength = 18

func (s *Service) CreateToken(ctx context.Context, command CreateTokenCommand) (*CreateTokenIssued, error) {
	// HTTP handlers replace the body organization with TrustedOrganizationID;
	// non-HTTP callers must likewise supply an already-authorized tenant.
	if err := validateCreateTokenCommand(command); err != nil {
		return nil, err
	}

	var issued *CreateTokenIssued
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		organization, err := tx.Organizations().GetByID(ctx, command.OrganizationID)
		if err != nil {
			return err
		}

		user, err := tx.Users().GetByID(ctx, command.UserID)
		if err != nil {
			return err
		}
		if user.OrganizationID != command.OrganizationID {
			return apperrors.NewInvalidArgument("user does not belong to organizationId")
		}

		application, err := tx.Applications().GetByID(ctx, command.ApplicationID)
		if err != nil {
			return err
		}
		if application.OrganizationID != command.OrganizationID {
			return apperrors.NewInvalidArgument("application does not belong to organizationId")
		}

		tokenValue, tokenPrefix, tokenHash, err := s.newTokenSecret()
		if err != nil {
			return err
		}

		tokenID, err := s.UUID.NewString()
		if err != nil {
			return err
		}

		now := s.Clock.Now()
		token := &domain.Token{
			ID:               tokenID,
			UserID:           user.ID,
			Username:         user.Username,
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			ApplicationID:    application.ID,
			ApplicationSlug:  application.Slug,
			TokenPrefix:      tokenPrefix,
			TokenHash:        tokenHash,
			Scopes:           normalizeStringList(command.Scopes),
			Audience:         normalizeStringList(command.Audience),
			Metadata:         cloneMap(command.Metadata),
			ExpiresAt:        now.Add(time.Duration(command.ExpiresInSeconds) * time.Second),
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}

		if err := tx.Tokens().Create(ctx, token); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			token.OrganizationID,
			// BE-D-13：actor 是操作者（ctx 回落），token 主体只进 payload。
			"",
			"token.created",
			"Token",
			token.ID,
			"Token",
			"token.created",
			map[string]any{"userId": token.UserID},
		); err != nil {
			return err
		}
		refreshTokenValue := ""
		if command.IssueRefreshToken {
			refreshTokenID, err := s.UUID.NewString()
			if err != nil {
				return err
			}

			refreshTokenHash := ""
			refreshTokenValue, refreshTokenHash, err = s.newRefreshTokenSecret()
			if err != nil {
				return err
			}

			refreshTTL := time.Duration(command.RefreshExpiresInSeconds) * time.Second
			if refreshTTL <= 0 {
				refreshTTL = 30 * 24 * time.Hour
			}

			refreshToken := &domain.RefreshToken{
				ID:               refreshTokenID,
				AccessTokenID:    token.ID,
				UserID:           user.ID,
				Username:         user.Username,
				OrganizationID:   organization.ID,
				OrganizationSlug: organization.Slug,
				ApplicationID:    application.ID,
				ApplicationSlug:  application.Slug,
				RefreshTokenHash: refreshTokenHash,
				Scopes:           append([]string(nil), token.Scopes...),
				Audience:         append([]string(nil), token.Audience...),
				// First generation is its own family root: family_id has a
				// self-referencing FK; leaving it empty would store NULL and
				// disable reuse-triggered family revocation.
				FamilyID:  refreshTokenID,
				ExpiresAt: now.Add(refreshTTL),
				CreatedAt: now,
				UpdatedAt: now,
				Revision:  1,
			}

			if err := tx.Tokens().CreateRefresh(ctx, refreshToken); err != nil {
				return err
			}

			if err := s.embedAuditEvent(
				tx,
				ctx,
				refreshToken.OrganizationID,
				// BE-D-13：actor 是操作者（ctx 回落），token 主体只进 payload。
				"",
				"refresh_token.created",
				"RefreshToken",
				refreshToken.ID,
				"RefreshToken",
				"refresh_token.created",
				map[string]any{"userId": refreshToken.UserID},
			); err != nil {
				return err
			}
			issued = &CreateTokenIssued{
				TokenDetail:  *toDetail(token),
				TokenValue:   tokenValue,
				RefreshToken: refreshTokenValue,
			}
			return nil
		}

		issued = &CreateTokenIssued{
			TokenDetail: *toDetail(token),
			TokenValue:  tokenValue,
		}
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "tokens.CreateToken failed",
			"organization_id", command.OrganizationID,
			"user_id", command.UserID,
			"application_id", command.ApplicationID,
			"error", err,
		)
		return nil, err
	}

	slog.InfoContext(ctx, "tokens.CreateToken ok",
		"organization_id", command.OrganizationID,
		"user_id", command.UserID,
		"application_id", command.ApplicationID,
		"row_id", issued.ID,
	)
	return issued, nil
}

func validateCreateTokenCommand(command CreateTokenCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.UserID) == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if strings.TrimSpace(command.ApplicationID) == "" {
		return apperrors.NewInvalidArgument("applicationId is required")
	}
	if command.ExpiresInSeconds <= 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "expiresInSeconds must be greater than 0", map[string]any{"field": "expiresInSeconds", "value": command.ExpiresInSeconds, "min": 1})
	}
	if len(normalizeStringList(command.Scopes)) == 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "scopes must contain at least one item", map[string]any{"field": "scopes", "min": 1})
	}

	return nil
}

// tokenSecretBytes is the entropy of a bearer secret: 32 bytes (256 bit) from
// crypto/rand, well above the 128-bit convention. UUIDv7-derived secrets only
// carried ~74 random bits plus a guessable time prefix.
const tokenSecretBytes = 32

// newTokenSecretValue returns a URL-safe random secret with the given prefix.
// The value is never stored (only its SHA-256 hash); the 43-char base64url body
// keeps the whole secret far below the token_prefix column width.
func newTokenSecretValue(prefix string) (string, error) {
	buf := make([]byte, tokenSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", apperrors.WrapInternal(err, "generate token secret")
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *Service) newTokenSecret() (value, prefix, hash string, err error) {
	tokenValue, err := newTokenSecretValue("gl_pat_")
	if err != nil {
		return "", "", "", err
	}
	prefixLength := minInt(len(tokenValue), tokenPrefixMaxLength)
	tokenPrefix := tokenValue[:prefixLength]

	return tokenValue, tokenPrefix, hashTokenValue(tokenValue), nil
}

func (s *Service) newRefreshTokenSecret() (value, hash string, err error) {
	tokenValue, err := newTokenSecretValue("gl_prt_")
	if err != nil {
		return "", "", err
	}
	return tokenValue, hashTokenValue(tokenValue), nil
}

func normalizeStringList(values []string) []string {
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func minInt(value, fallback int) int {
	if value < fallback {
		return value
	}
	return fallback
}

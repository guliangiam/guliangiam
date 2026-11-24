package tokens

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// refreshReuseGraceWindow bounds the concurrent-rotation window in which an
// already-rotated refresh token is treated as a loser instead of a stolen
// replay. Older reuse still revokes the whole family.
const refreshReuseGraceWindow = 10 * time.Second

func (s *Service) RefreshAccessToken(ctx context.Context, command RefreshAccessTokenCommand) (*RefreshTokenIssued, error) {
	if strings.TrimSpace(command.RefreshToken) == "" {
		return nil, apperrors.NewInvalidArgument("refreshToken is required")
	}

	var issued *RefreshTokenIssued
	// rejection is returned only after the transaction commits. Reuse
	// detection revokes the family inside the transaction, and returning that
	// error from the closure would roll the revocation back — leaving the
	// stolen family alive even though the caller was told it had been revoked.
	var rejection error
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()
		refreshToken, err := tx.Tokens().GetRefreshByTokenHash(ctx, hashTokenValue(command.RefreshToken))
		if err != nil {
			if appErr, ok := err.(*apperrors.Error); ok && appErr.Code == apperrors.CodeNotFound {
				return apperrors.NewInvalidArgument("refresh token is invalid or expired")
			}
			return err
		}
		if !tokenBelongsToTenant(command.OrganizationID, refreshToken.OrganizationID) {
			return apperrors.NewInvalidArgument("refresh token is invalid or expired")
		}
		// Legacy rows may carry family_id NULL (issued before family roots were
		// self-referencing). Treat the row itself as the family root so reuse
		// detection still revokes it and every descendant rotated from it.
		if refreshToken.FamilyID == "" {
			refreshToken.FamilyID = refreshToken.ID
		}
		if command.ApplicationID != "" && refreshToken.ApplicationID != command.ApplicationID {
			return apperrors.NewInvalidArgument("refresh token is invalid or expired")
		}
		if refreshToken.RevokedAt != nil {
			// A refresh token revoked moments ago is most likely a concurrent
			// rotation loser, not an attacker replaying a stolen token. Inside
			// the grace window reject the loser WITHOUT revoking the family so
			// the winner's freshly issued pair (returned with 200) stays usable
			// (OPEN-API-47). Older reuse still triggers whole-family revocation.
			if now.Sub(*refreshToken.RevokedAt) <= refreshReuseGraceWindow {
				rejection = apperrors.NewUnauthorized("refresh token was already used")
				return nil
			}
			if refreshToken.FamilyID != "" {
				if err := revokeTokenFamilyByUoW(ctx, tx, refreshToken.FamilyID); err != nil {
					return apperrors.NewInternal("failed to revoke token family after reuse detection: " + err.Error())
				}
			}
			s.auditTokenFamilyReuse(ctx, tx, refreshToken)
			rejection = apperrors.NewUnauthorized("token reuse detected: all tokens in the family have been revoked")
			return nil
		}
		if !refreshToken.ExpiresAt.After(now) {
			return apperrors.NewInvalidArgument("refresh token is invalid or expired")
		}

		currentAccessToken, err := tx.Tokens().GetByID(ctx, refreshToken.AccessTokenID)
		if err != nil {
			if appErr, ok := err.(*apperrors.Error); ok && appErr.Code == apperrors.CodeNotFound {
				return apperrors.NewInvalidArgument("refresh token is invalid or expired")
			}
			return err
		}
		if !tokenBelongsToTenant(command.OrganizationID, currentAccessToken.OrganizationID) {
			return apperrors.NewInvalidArgument("refresh token is invalid or expired")
		}
		if command.ApplicationID != "" && currentAccessToken.ApplicationID != command.ApplicationID {
			return apperrors.NewInvalidArgument("refresh token is invalid or expired")
		}
		if currentAccessToken.RevokedAt != nil {
			return apperrors.NewInvalidArgument("refresh token is invalid or expired")
		}

		organization, err := tx.Organizations().GetByID(ctx, currentAccessToken.OrganizationID)
		if err != nil {
			return err
		}
		user, err := tx.Users().GetByID(ctx, currentAccessToken.UserID)
		if err != nil {
			return err
		}
		application, err := tx.Applications().GetByID(ctx, currentAccessToken.ApplicationID)
		if err != nil {
			return err
		}

		// Rotation is the reuse-detection gate. GetRefreshByTokenHash locked the
		// refresh_tokens row (FOR UPDATE) and the check above proved it was live at
		// that point, so both revokes must affect exactly one row; anything else
		// means a concurrent rotation won and this request must not mint a second
		// pair.

		if err := tx.Tokens().Revoke(ctx, currentAccessToken.ID, now); err != nil {
			return apperrors.WrapInternal(err, "revoke rotated access token")
		}

		if err := tx.Tokens().RevokeRefresh(ctx, refreshToken.ID, now); err != nil {
			if appErr, ok := err.(*apperrors.Error); ok && appErr.Code == apperrors.CodeUnauthorized {
				if refreshToken.FamilyID != "" {
					if revokeErr := revokeTokenFamilyByUoW(ctx, tx, refreshToken.FamilyID); revokeErr != nil {
						return apperrors.NewInternal("failed to revoke token family after reuse detection: " + revokeErr.Error())
					}
				}
				s.auditTokenFamilyReuse(ctx, tx, refreshToken)
				rejection = domain.ErrRefreshAlreadyRevoked
				return nil
			}
			return apperrors.WrapInternal(err, "revoke rotated refresh token")
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			currentAccessToken.OrganizationID,
			currentAccessToken.UserID,
			"token.revoked",
			"Token",
			currentAccessToken.ID,
			"Token",
			"token.revoked",
			map[string]any{},
		); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			refreshToken.OrganizationID,
			refreshToken.UserID,
			"refresh_token.revoked",
			"RefreshToken",
			refreshToken.ID,
			"RefreshToken",
			"refresh_token.revoked",
			map[string]any{},
		); err != nil {
			return err
		}

		newAccessTokenValue, newAccessTokenPrefix, newAccessTokenHash, err := s.newTokenSecret()
		if err != nil {
			return err
		}
		newAccessTokenID, err := s.UUID.NewString()
		if err != nil {
			return err
		}

		accessTTL := currentAccessToken.ExpiresAt.Sub(currentAccessToken.CreatedAt)
		if accessTTL <= 0 {
			accessTTL = time.Hour
		}

		newAccessToken := &domain.Token{
			ID:               newAccessTokenID,
			UserID:           user.ID,
			Username:         user.Username,
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			ApplicationID:    application.ID,
			ApplicationSlug:  application.Slug,
			TokenPrefix:      newAccessTokenPrefix,
			TokenHash:        newAccessTokenHash,
			Scopes:           append([]string(nil), currentAccessToken.Scopes...),
			Audience:         append([]string(nil), currentAccessToken.Audience...),
			Metadata:         cloneMap(currentAccessToken.Metadata),
			ExpiresAt:        now.Add(accessTTL),
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := tx.Tokens().Create(ctx, newAccessToken); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			newAccessToken.OrganizationID,
			newAccessToken.UserID,
			"token.created",
			"Token",
			newAccessToken.ID,
			"Token",
			"token.created",
			map[string]any{},
		); err != nil {
			return err
		}
		newRefreshTokenValue, newRefreshTokenHash, err := s.newRefreshTokenSecret()
		if err != nil {
			return err
		}
		newRefreshTokenID, err := s.UUID.NewString()
		if err != nil {
			return err
		}

		refreshTTL := refreshToken.ExpiresAt.Sub(refreshToken.CreatedAt)
		if refreshTTL <= 0 {
			refreshTTL = 30 * 24 * time.Hour
		}

		newRefreshToken := &domain.RefreshToken{
			ID:               newRefreshTokenID,
			AccessTokenID:    newAccessToken.ID,
			UserID:           user.ID,
			Username:         user.Username,
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			ApplicationID:    application.ID,
			ApplicationSlug:  application.Slug,
			RefreshTokenHash: newRefreshTokenHash,
			Scopes:           append([]string(nil), newAccessToken.Scopes...),
			Audience:         append([]string(nil), newAccessToken.Audience...),
			FamilyID:         refreshToken.FamilyID,
			ExpiresAt:        now.Add(refreshTTL),
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := tx.Tokens().CreateRefresh(ctx, newRefreshToken); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			newRefreshToken.OrganizationID,
			newRefreshToken.UserID,
			"refresh_token.created",
			"RefreshToken",
			newRefreshToken.ID,
			"RefreshToken",
			"refresh_token.created",
			map[string]any{},
		); err != nil {
			return err
		}
		issued = &RefreshTokenIssued{
			TokenDetail:  *toDetail(newAccessToken),
			TokenValue:   newAccessTokenValue,
			RefreshToken: newRefreshTokenValue,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if rejection != nil {
		return nil, rejection
	}

	return issued, nil
}

// auditTokenFamilyReuse records the OAuth 2.1 reuse-detection event in the same
// transaction that revoked the family. The revocation is the security action,
// so an audit outage must not roll it back: the error is logged and the caller
// still answers with the reuse rejection.
func (s *Service) auditTokenFamilyReuse(ctx context.Context, tx appuow.UnitOfWork, refreshToken *domain.RefreshToken) {
	if refreshToken == nil || refreshToken.FamilyID == "" {
		return
	}
	if err := s.embedAuditEvent(
		tx,
		ctx,
		refreshToken.OrganizationID,
		refreshToken.UserID,
		"token.reuse_detected",
		"TokenFamily",
		refreshToken.FamilyID,
		"TokenFamily",
		"token.reuse_detected",
		map[string]any{"refresh_token_id": refreshToken.ID},
	); err != nil {
		slog.WarnContext(ctx, "failed to audit token reuse detection",
			"family_id", refreshToken.FamilyID,
			"error", err,
		)
	}
}

func revokeTokenFamilyByUoW(ctx context.Context, uow appuow.UnitOfWork, familyID string) error {
	if familyID == "" {
		return nil
	}
	now := time.Now()
	affected, err := uow.Tokens().RevokeByFamilyID(ctx, familyID, now)
	if err != nil {
		return apperrors.WrapInternal(err, "revoke token family")
	}
	slog.InfoContext(ctx, "token family revoked due to reuse detection",
		"family_id", familyID,
		"revoked_count", affected,
	)
	return nil
}

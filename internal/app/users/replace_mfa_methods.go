package users

import (
	"context"
	"fmt"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ReplaceUserMfaMethodCommand struct {
	ID          string
	MethodType  string
	Enabled     bool
	Verified    bool
	DisplayName string
}

// ReplaceUserMfaMethods replaces the complete factor set for a user.
//
// revision is the users.revision returned by the read API and is mandatory:
// two admin tabs (or two admins) editing concurrently can no longer overwrite
// each other silently because a stale value is rejected with 409.
//
// verified is never taken from the caller. It is derived from the stored
// method (false for a factor that did not exist) because only a server-side
// proof-of-possession may set it. Callers that assert verified=true are
// rejected with 422 instead of being silently ignored, so a client cannot
// believe an unverified factor was accepted.
func (s *Service) ReplaceUserMfaMethods(ctx context.Context, organizationID, userID string, revision int64, commands []ReplaceUserMfaMethodCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if revision <= 0 {
		return apperrors.NewWithMeta(
			apperrors.CodeConflict,
			"revision is required for mfa method replacement",
			map[string]any{"field": "revision"},
		)
	}
	if err := validateReplaceUserMfaMethodsCommands(commands); err != nil {
		return err
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// MFA secrets are tenant data too: resolve ownership before replacing
		// methods so a foreign user id cannot rotate or clear its factors.
		user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}
		if user.Revision != revision {
			return apperrors.NewWithMeta(
				apperrors.CodeConflict,
				"user record changed (possibly from MFA or other profile edits); refresh and retry",
				map[string]any{
					"field":    "revision",
					"expected": user.Revision,
					"provided": revision,
				},
			)
		}

		existingByID := make(map[string]userdomain.UserMfaMethod, len(user.MfaMethods))
		existingByType := make(map[string]userdomain.UserMfaMethod, len(user.MfaMethods))
		for _, method := range user.MfaMethods {
			existingByID[method.ID] = method
			key := normalizeExistingUserMfaMethodType(method.MethodType)
			if _, ok := existingByType[key]; !ok {
				existingByType[key] = method
			}
		}

		methods := make([]userdomain.UserMfaMethod, 0, len(commands))
		for index, command := range commands {
			if command.Verified {
				return apperrors.NewWithMeta(
					apperrors.CodeUnprocessableEntity,
					"verified is determined by the server and cannot be set by the client",
					map[string]any{"field": fmt.Sprintf("items[%d].verified", index)},
				)
			}

			existing, found := existingByID[command.ID]
			if !found {
				existing, found = existingByType[normalizeExistingUserMfaMethodType(command.MethodType)]
			}

			// A factor can only be enabled after a server-side proof of
			// possession marked it verified. This also keeps legacy
			// enabled-but-unverified rows closable (Enabled=false bypasses the gate).
			if command.Enabled && (!found || !existing.Verified) {
				return apperrors.NewWithMeta(
					apperrors.CodeUnprocessableEntity,
					"mfa method must be verified before it can be enabled",
					map[string]any{"field": fmt.Sprintf("items[%d].enabled", index)},
				)
			}

			methodID := ""
			if found {
				methodID = existing.ID
			}
			if methodID == "" {
				generatedID, err := s.UUID.NewString()
				if err != nil {
					return err
				}
				methodID = generatedID
			}

			method := userdomain.UserMfaMethod{
				ID:          methodID,
				UserID:      userID,
				MethodType:  command.MethodType,
				Enabled:     command.Enabled,
				Verified:    found && existing.Verified,
				DisplayName: command.DisplayName,
			}
			// Keeping the last used window preserves TOTP replay protection
			// across an unrelated toggle of the factor list.
			if found {
				method.LastUsedAt = existing.LastUsedAt
			}
			methods = append(methods, method)
		}

		user.MfaMethods = methods
		preferredStillEnabled := false
		for _, method := range methods {
			if normalizeExistingUserMfaMethodType(method.MethodType) == normalizeExistingUserMfaMethodType(user.PreferredMfaType) && method.Enabled {
				preferredStillEnabled = true
				break
			}
		}
		if !preferredStillEnabled {
			user.PreferredMfaType = ""
		}
		// Server-owned mirror: disabling or removing a factor clears its
		// collections entries, enabling a verified factor re-derives them.
		if err := deriveUserMfaCollections(ctx, tx, user); err != nil {
			return err
		}

		user.UpdatedAt = s.Clock.Now()
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, organizationID); err != nil {
			return err
		}

		if err := tx.Users().ReplaceMfaMethods(ctx, userID, methods); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.mfa.replaced",
			"User",
			user.ID,
			"user",
			"user.mfa.replaced",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

package users

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

const (
	adminRebindAuditAction   = "mfa.totp.admin_rebind"
	adminRebindReasonMaxLen  = 500
	adminRebindCooldown      = 24 * time.Hour
	adminRebindPlatformAdmin = string(scopeauthorizer.PrincipalPlatformAdmin)
)

// AdminRebindTOTPCommand resets another user's TOTP factor on behalf of a
// platform administrator. It is cross-tenant by design: the target user's
// organization is read from the user row, never from the request.
type AdminRebindTOTPCommand struct {
	ActorID            string
	ActorPrincipalType string
	TargetUserID       string
	Reason             string
	Confirm            bool
}

// AdminRebindTOTP clears the target user's TOTP secret and recovery codes,
// removes the stored totp MFA method, and records an audit event. Platform
// administrators only; a 24h cooldown per target user prevents repeated resets.
func (s *Service) AdminRebindTOTP(ctx context.Context, command AdminRebindTOTPCommand) error {
	if err := validateAdminRebindTOTPCommand(command); err != nil {
		return err
	}

	now := s.Clock.Now().UTC()
	since := now.Add(-adminRebindCooldown)

	return s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByID(ctx, command.TargetUserID)
		if err != nil {
			return err
		}

		// Records() is nil in lightweight unit tests; the cooldown check is
		// skipped there instead of failing the use case.
		if records := tx.Records(); records != nil {
			_, total, err := records.Search(ctx, recordsdomain.SearchCriteria{
				Page:           1,
				PageSize:       1,
				OrganizationID: user.OrganizationID,
				Action:         adminRebindAuditAction,
				ResourceType:   "User",
				ResourceID:     command.TargetUserID,
				StartTime:      &since,
			})
			if err != nil {
				return err
			}
			if err := adminRebindCoolingDown(total, since); err != nil {
				return err
			}
		}

		user.TotpSecret = ""
		user.RecoveryCodeHashes = nil
		user.MfaMethods = removeTOTPMfaMethod(user.MfaMethods)

		// Server-owned mirror: the cleared TOTP factor must disappear from the
		// collections entries together with the mfa-methods row.
		if err := deriveUserMfaCollections(ctx, tx, user); err != nil {
			return err
		}

		user.UpdatedAt = now
		user.Revision++

		if err := tx.Users().Update(ctx, user); err != nil {
			return err
		}
		if err := tx.Users().ReplaceMfaMethods(ctx, user.ID, user.MfaMethods); err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			adminRebindAuditAction,
			"User",
			user.ID,
			"User",
			adminRebindAuditAction,
			map[string]any{
				"actorId":      command.ActorID,
				"targetUserId": user.ID,
				"reason":       strings.TrimSpace(command.Reason),
			},
		)
	})
}

// validateAdminRebindTOTPCommand is the pure, side-effect-free guard so the
// authorization and confirmation rules can be tested without a UnitOfWork.
func validateAdminRebindTOTPCommand(command AdminRebindTOTPCommand) error {
	if command.ActorPrincipalType != adminRebindPlatformAdmin {
		return apperrors.NewForbidden("platform administrator required")
	}
	if !command.Confirm {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"confirm must be true",
			map[string]any{"field": "confirm", "value": command.Confirm, "expected": true},
		)
	}
	reason := strings.TrimSpace(command.Reason)
	if reason == "" {
		return apperrors.NewInvalidArgument("reason is required")
	}
	if utf8.RuneCountInString(reason) > adminRebindReasonMaxLen {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"reason must be at most 500 characters",
			map[string]any{
				"field": "reason",
				"value": utf8.RuneCountInString(reason),
				"max":   adminRebindReasonMaxLen,
			},
		)
	}
	return nil
}

// adminRebindCoolingDown reports whether the target user already had a TOTP
// rebind inside the cooldown window.
func adminRebindCoolingDown(total int64, since time.Time) error {
	if total <= 0 {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeTooManyRequests,
		"totp rebind is cooling down",
		map[string]any{"since": since.Format(time.RFC3339)},
	)
}

func removeTOTPMfaMethod(methods []userdomain.UserMfaMethod) []userdomain.UserMfaMethod {
	kept := make([]userdomain.UserMfaMethod, 0, len(methods))
	for _, method := range methods {
		if strings.EqualFold(strings.TrimSpace(method.MethodType), mfaMethodTOTP) {
			continue
		}
		kept = append(kept, method)
	}
	return kept
}

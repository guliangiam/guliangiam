package auth

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// InitiatePushMfaCommand represents the command to initiate a push MFA request.
type InitiatePushMfaCommand struct {
	OrganizationID string
	// UserID is the authenticated session user. Transport binds it to
	// SessionContext.UserID and rejects an explicit body mismatch (403); the
	// service never accepts a caller-chosen foreign target.
	UserID        string
	ApplicationID string
	IPAddress     string
}

// InitiatePushMfaResult represents the result of initiating a push MFA request.
type InitiatePushMfaResult struct {
	ChallengeID    string
	ChallengeState string
	ExpiresAt      time.Time
}

// InitiatePushMfa sends a push notification to the user's configured receiver
// via their push MFA provider and creates a challenge for tracking the approval.
func (s *Service) InitiatePushMfa(ctx context.Context, command InitiatePushMfaCommand) (*InitiatePushMfaResult, error) {
	// The target always comes from the verified session. An empty subject is
	// rejected before any lookup, and a session principal without a local row
	// is reported as unauthenticated rather than as a distinguishable 404.
	sessionUserID := strings.TrimSpace(command.UserID)
	if sessionUserID == "" {
		return nil, apperrors.NewUnauthorized("authentication required")
	}
	command.UserID = sessionUserID

	var result *InitiatePushMfaResult
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()

		user, err := tx.Users().GetByID(ctx, command.UserID)
		if err != nil {
			if appErr := apperrors.FromError(err); appErr.Code == apperrors.CodeNotFound {
				return apperrors.NewUnauthorized("authentication required")
			}
			return err
		}

		receiver := strings.TrimSpace(user.MfaPushReceiver)
		if receiver == "" {
			return apperrors.NewUnauthorized("push mfa receiver is not configured for the user")
		}
		providerID := strings.TrimSpace(user.MfaPushProvider)
		if providerID == "" {
			return apperrors.NewUnauthorized("push mfa provider is not configured for the user")
		}

		config, err := resolvePushProvider(ctx, tx, providerID)
		if err != nil {
			return err
		}

		// Build notification content with contextual information.
		notification := BuildMfaPushNotification(MfaPushNotificationParams{
			LoginTime:  now,
			IPAddress:  command.IPAddress,
			ActionType: "sign-in",
		})

		if s.PushNotifier != nil {
			err = s.PushNotifier.Send(ctx, config.providerType, config, receiver, notification.Title, notification.Body)
		} else {
			err = sendPushAuthentication(ctx, config, receiver, notification.Title, notification.Body)
		}
		if err != nil {
			return err
		}

		// Create a challenge to track the push approval.
		challengeID, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate challenge id")
		}

		challenge := &mfachallengedomain.Challenge{
			ID:             challengeID,
			OrganizationID: command.OrganizationID,
			UserID:         command.UserID,
			ApplicationID:  command.ApplicationID,
			State:          mfachallengedomain.StatePushPending,
			MethodType:     mfaTypePush,
			ExpiresAt:      now.Add(mfaChallengeTTL),
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		if err := tx.MfaChallenges().Create(ctx, challenge); err != nil {
			return err
		}
		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			"",
			"",
			"mfa.challenge.created",
			loginMethodMFA,
			challenge.ID,
			loginMethodMFA,
			"mfa.challenge.created",
			map[string]any{},
		); err != nil {
			return err
		}

		result = &InitiatePushMfaResult{
			ChallengeID:    challenge.ID,
			ChallengeState: challenge.State,
			ExpiresAt:      challenge.ExpiresAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// VerifyPushMfaCommand represents the command to verify a push MFA approval.
type VerifyPushMfaCommand struct {
	ChallengeID string
	// UserID is the authenticated session user; transport binds it. The
	// challenge must belong to this user before it can be read or consumed.
	UserID string
}

// VerifyPushMfaResult represents the result of verifying a push MFA approval.
type VerifyPushMfaResult struct {
	Approved bool
}

// VerifyPushMfa checks whether a push MFA challenge has been approved by the user.
func (s *Service) VerifyPushMfa(ctx context.Context, command VerifyPushMfaCommand) (*VerifyPushMfaResult, error) {
	challengeID := strings.TrimSpace(command.ChallengeID)
	if challengeID == "" {
		return nil, apperrors.NewInvalidArgument("challengeId is required")
	}
	// The caller can only observe a challenge minted for its own session. A
	// foreign challenge shares the login-challenge module's invalid/expired
	// rejection shape with a missing one, so verify is not an existence oracle.
	sessionUserID := strings.TrimSpace(command.UserID)
	if sessionUserID == "" {
		return nil, invalidChallengeError()
	}

	var result *VerifyPushMfaResult
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()

		challenge, err := tx.MfaChallenges().GetByID(ctx, challengeID)
		if err != nil {
			if appErr := apperrors.FromError(err); appErr.Code == apperrors.CodeNotFound {
				auditPushMfaRejection(ctx, challengeID, sessionUserID, "challenge_missing_or_consumed")
				return invalidChallengeError()
			}
			return err
		}
		if challenge.UserID != sessionUserID {
			auditPushMfaRejection(ctx, challenge.ID, sessionUserID, "challenge_not_owned")
			return invalidChallengeError()
		}

		if !challenge.ExpiresAt.After(now) {
			if err := tx.MfaChallenges().Delete(ctx, challenge.ID); err != nil {
				slog.WarnContext(ctx, "push mfa: delete expired challenge failed", "challenge_id", challenge.ID, "error", err)
			}
			auditPushMfaRejection(ctx, challenge.ID, sessionUserID, "challenge_expired")
			if err := s.embedAuditOutboxPlain(
				tx,
				ctx,
				"",
				"",
				"",
				"",
				"",
				"mfa.challenge.deleted",
				loginMethodMFA,
				challenge.ID,
				loginMethodMFA,
				"mfa.challenge.deleted",
				map[string]any{},
			); err != nil {
				return err
			}
			return apperrors.NewUnauthorized("push mfa challenge has expired")
		}

		switch challenge.State {
		case mfachallengedomain.StateApproved:
			// Consume exactly once: a concurrent verifier that loses the delete
			// race sees not-found and is rejected instead of approving twice.
			if err := tx.MfaChallenges().Delete(ctx, challenge.ID); err != nil {
				if appErr := apperrors.FromError(err); appErr.Code == apperrors.CodeNotFound {
					auditPushMfaRejection(ctx, challenge.ID, sessionUserID, "challenge_already_consumed")
					return invalidChallengeError()
				}
				return err
			}
			if err := s.embedAuditOutboxPlain(
				tx,
				ctx,
				"",
				"",
				"",
				"",
				"",
				"mfa.challenge.deleted",
				loginMethodMFA,
				challenge.ID,
				loginMethodMFA,
				"mfa.challenge.deleted",
				map[string]any{},
			); err != nil {
				return err
			}
			result = &VerifyPushMfaResult{Approved: true}
			return nil
		case mfachallengedomain.StateRejected:
			if err := tx.MfaChallenges().Delete(ctx, challenge.ID); err != nil {
				slog.WarnContext(ctx, "push mfa: delete rejected challenge failed", "challenge_id", challenge.ID, "error", err)
			}
			auditPushMfaRejection(ctx, challenge.ID, sessionUserID, "challenge_rejected")
			if err := s.embedAuditOutboxPlain(
				tx,
				ctx,
				"",
				"",
				"",
				"",
				"",
				"mfa.challenge.deleted",
				loginMethodMFA,
				challenge.ID,
				loginMethodMFA,
				"mfa.challenge.deleted",
				map[string]any{},
			); err != nil {
				return err
			}
			return apperrors.NewUnauthorized("push approval rejected")
		case mfachallengedomain.StatePushPending:
			result = &VerifyPushMfaResult{Approved: false}
			return nil
		default:
			return apperrors.NewInvalidArgumentf("unexpected push challenge state: %q", challenge.State)
		}
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// auditPushMfaRejection records a refused push-MFA verification on the platform
// audit stream. It reuses the auth login-failure category because this endpoint
// is the second-factor step of the same authentication flow.
func auditPushMfaRejection(ctx context.Context, challengeID, actorID, reason string) {
	audit.SlogAudit(ctx, audit.EventAuthLoginFailed,
		"mfa.challenge_id", challengeID,
		"mfa.actor_id", actorID,
		"mfa.reason", reason,
	)
}

// EnablePushMfaCommand represents the command to enable push MFA for a user.
type EnablePushMfaCommand struct {
	// UserID is the authenticated session user. Transport binds it to
	// SessionContext.UserID and rejects an explicit body mismatch (403); the
	// service itself must never receive a caller-chosen foreign target.
	UserID            string
	MfaPushReceiver   string
	MfaPushProvider   string
	PreviousSessionID string // S9-sensitive
}

// EnablePushMfa enables Push MFA for the user, storing the receiver and provider
// configuration and adding a push method to the user's MFA methods.
func (s *Service) EnablePushMfa(ctx context.Context, command EnablePushMfaCommand) (string, error) {
	newSessionID := ""
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByID(ctx, command.UserID)
		if err != nil {
			return err
		}

		receiver := strings.TrimSpace(command.MfaPushReceiver)
		if receiver == "" {
			return apperrors.NewInvalidArgument("mfaPushReceiver is required")
		}
		providerID := strings.TrimSpace(command.MfaPushProvider)
		if providerID == "" {
			return apperrors.NewInvalidArgument("mfaPushProvider is required")
		}

		// Validate that the provider exists and is a notification provider.
		provider, err := tx.Providers().GetByID(ctx, providerID)
		if err != nil {
			return apperrors.NewInvalidArgument("push mfa provider not found")
		}
		if strings.TrimSpace(string(provider.Category)) != "Notification" {
			return apperrors.NewInvalidArgument("push mfa provider must be a notification provider")
		}

		// Update user push MFA fields.
		user.MfaPushEnabled = true
		user.MfaPushReceiver = receiver
		user.MfaPushProvider = providerID

		// Add or update the push MFA method in the user's MFA methods.
		found := false
		for i := range user.MfaMethods {
			if user.MfaMethods[i].MethodType != mfaTypePush {
				continue
			}
			user.MfaMethods[i].Enabled = true
			user.MfaMethods[i].Verified = true
			user.MfaMethods[i].DisplayName = "Push Notification"
			found = true
			break
		}
		if !found {
			methodID, err := s.UUIDGenerator.NewString()
			if err != nil {
				return apperrors.NewInternal("generate mfa method id")
			}
			user.MfaMethods = append(user.MfaMethods, userdomain.UserMfaMethod{
				ID:          methodID,
				UserID:      user.ID,
				MethodType:  mfaTypePush,
				Enabled:     true,
				Verified:    true,
				DisplayName: "Push Notification",
			})
		}

		if err := tx.Users().Update(ctx, user); err != nil {
			return err
		}

		if err := tx.Users().ReplaceMfaMethods(ctx, user.ID, user.MfaMethods); err != nil {
			return err
		}

		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			user.OrganizationID,
			user.ID,
			"user.updated",
			"user",
			user.ID,
			"user",
			"user.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		// S9-sensitive: enabling a second factor elevates the account's assurance level.
		// Revoke the old session and issue a fresh opaque ID — any pre-planted cookie
		// is now dead, preventing session fixation on the just-hardened account.
		if strings.TrimSpace(command.PreviousSessionID) != "" {
			now := s.Clock.Now().UTC()
			oldSess, getErr := tx.Sessions().GetByID(ctx, command.PreviousSessionID)
			if getErr == nil {
				if err := s.embedAuditOutboxPlain(
					tx,
					ctx,
					"",
					"",
					"",
					"",
					"",
					"session.revoked",
					"session",
					command.PreviousSessionID,
					"session",
					"session.revoked",
					map[string]any{},
				); err != nil {
					return err
				}
				// Not best-effort: a failed revoke rolls the whole rotation back
				// instead of leaving the old session alive next to the new one.
				if err := revokePreviousSessionForRotation(ctx, tx.Sessions(), command.PreviousSessionID, now); err != nil {
					return err
				}
				sessID, idErr := s.UUIDGenerator.NewString()
				if idErr != nil {
					return apperrors.NewInternal("generate rotated session id")
				}
				sid, sidErr := newSessionSid()
				if sidErr != nil {
					return sidErr
				}
				if cErr := tx.Sessions().Create(ctx, &sessionsdomain.Session{
					ID:               sessID,
					Sid:              sid,
					UserID:           oldSess.UserID,
					Username:         oldSess.Username,
					OrganizationID:   oldSess.OrganizationID,
					OrganizationSlug: oldSess.OrganizationSlug,
					ApplicationID:    oldSess.ApplicationID,
					ApplicationSlug:  oldSess.ApplicationSlug,
					SessionState:     statusActive,
					DeviceInfo:       oldSess.DeviceInfo,
					IPAddress:        oldSess.IPAddress,
					UserAgent:        oldSess.UserAgent,
					ExpiresAt:        now.Add(30 * 24 * time.Hour),
					CreatedAt:        now,
					LastSeenAt:       now,
					UpdatedAt:        now,
					Revision:         1,
				}); cErr != nil {
					return cErr
				}
				newSessionID = sessID
			}
		}
		return nil
	})
	return newSessionID, err
}

// DisablePushMfaCommand represents the command to disable push MFA for a user.
type DisablePushMfaCommand struct {
	// UserID is the authenticated session user; transport rejects an explicit
	// body mismatch before this command is built.
	UserID            string
	PreviousSessionID string // S9-sensitive
}

// DisablePushMfa disables Push MFA for the user, clearing the receiver and provider
// configuration and removing the push method from the user's MFA methods.
func (s *Service) DisablePushMfa(ctx context.Context, command DisablePushMfaCommand) (string, error) {
	newSessionID := ""
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByID(ctx, command.UserID)
		if err != nil {
			return err
		}

		// Clear user push MFA fields.
		user.MfaPushEnabled = false
		user.MfaPushReceiver = ""
		user.MfaPushProvider = ""

		// Remove the push MFA method from the user's MFA methods.
		methods := make([]userdomain.UserMfaMethod, 0, len(user.MfaMethods))
		for _, method := range user.MfaMethods {
			if method.MethodType != mfaTypePush {
				methods = append(methods, method)
			}
		}
		user.MfaMethods = methods

		if err := tx.Users().Update(ctx, user); err != nil {
			return err
		}

		if err := tx.Users().ReplaceMfaMethods(ctx, user.ID, user.MfaMethods); err != nil {
			return err
		}

		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			user.OrganizationID,
			user.ID,
			"user.updated",
			"user",
			user.ID,
			"user",
			"user.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		// AC-AUTH-009: removing MFA is a credential-downgrade. Cascade revoke
		// the user's ENTIRE active session set + refresh family + access tokens.
		// Capture caller session context BEFORE RevokeByUser kills it.
		var oldSess *sessionsdomain.Session
		if strings.TrimSpace(command.PreviousSessionID) != "" {
			if sess, getErr := tx.Sessions().GetByID(ctx, command.PreviousSessionID); getErr == nil {
				oldSess = sess
			}
		}

		now := s.Clock.Now().UTC()
		if _, err := tx.Sessions().RevokeByUser(ctx, user.ID, now); err != nil {
			return err
		}
		if _, err := tx.Tokens().RevokeRefreshByUser(ctx, user.ID, now); err != nil {
			return err
		}
		if _, err := tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{UserID: user.ID}, now); err != nil {
			return err
		}

		// Issue fresh session for caller so they stay logged in after the downgrade.
		if oldSess != nil {
			sessID, idErr := s.UUIDGenerator.NewString()
			if idErr != nil {
				return apperrors.NewInternal("generate rotated session id")
			}
			sid, sidErr := newSessionSid()
			if sidErr != nil {
				return sidErr
			}
			if cErr := tx.Sessions().Create(ctx, &sessionsdomain.Session{
				ID:               sessID,
				Sid:              sid,
				UserID:           oldSess.UserID,
				Username:         oldSess.Username,
				OrganizationID:   oldSess.OrganizationID,
				OrganizationSlug: oldSess.OrganizationSlug,
				ApplicationID:    oldSess.ApplicationID,
				ApplicationSlug:  oldSess.ApplicationSlug,
				SessionState:     statusActive,
				DeviceInfo:       oldSess.DeviceInfo,
				IPAddress:        oldSess.IPAddress,
				UserAgent:        oldSess.UserAgent,
				ExpiresAt:        now.Add(30 * 24 * time.Hour),
				CreatedAt:        now,
				LastSeenAt:       now,
				UpdatedAt:        now,
				Revision:         1,
			}); cErr != nil {
				return cErr
			}
			newSessionID = sessID
		}
		return nil
	})
	return newSessionID, err
}

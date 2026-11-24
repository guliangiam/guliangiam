package auth

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	platformssf "github.com/guliangiam/guliangiam/internal/platform/ssf"
)

// Logout revokes the session and records the logout atomically: the revoke,
// the audit Record and the SSF revocation OutboxEvent share one transaction,
// so a receiver can never be told about a logout that did not commit (BE-06).
// Provider logout and back-channel notifications stay best-effort outside the
// transaction — they contact external systems and must not hold it open (S-12).
func (s *Service) Logout(ctx context.Context, command LogoutCommand) error {
	sessionID := strings.TrimSpace(command.SessionID)
	if sessionID == "" {
		return nil
	}

	session, err := s.UOW.Sessions().GetByID(ctx, sessionID)
	if err != nil && !isIgnorableLogoutError(err) {
		return apperrors.WrapInternalf(err, "lookup session %s for logout", sessionID)
	}

	now := s.Clock.Now().UTC()
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if revokeErr := tx.Sessions().Revoke(ctx, sessionID, now); revokeErr != nil && !isIgnorableLogoutError(revokeErr) {
			return apperrors.WrapInternalf(revokeErr, "revoke session %s for logout", sessionID)
		}

		// A logout that changed nothing (absent/expired session) writes no audit
		// row — same rule as CAS logout.
		if session == nil {
			return nil
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			session.OrganizationID, session.UserID,
			"user.logout", "user", session.UserID, "user",
			audit.EventAuthLogout,
			map[string]any{"sessionId": sessionID},
		); err != nil {
			return err
		}

		// CAEP/SSF: the revocation event commits with the revoke. An enqueue
		// failure rolls the logout back instead of warning and reporting success.
		if outbox := tx.Outbox(); outbox != nil && session.OrganizationID != "" {
			if issuer := s.resolveIssuer(command.IssuerURL); issuer != "" {
				event, buildErr := platformssf.NewRevocationEvent(
					issuer, session.OrganizationID, session.UserID, sessionID, "logout", now,
				)
				if buildErr != nil {
					return apperrors.WrapInternal(buildErr, "build SSF logout event")
				}
				if enqueueErr := outbox.Enqueue(ctx, event); enqueueErr != nil {
					return apperrors.WrapInternal(enqueueErr, "enqueue SSF logout event")
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if session != nil {
		// Best-effort upstream logout for custom OAuth providers. The provider
		// access token was sealed into the session during social login. Only the
		// local enqueue happens here; the outbox worker performs the POST so the
		// logout request never waits on an external endpoint (S-12).
		s.enqueueProviderLogout(ctx, session)

		if session.UserID != "" && s.JWKSManager != nil {
			s.sendBackchannelLogoutNotifications(ctx, session.UserID, command.IssuerURL)
		}
	}

	return nil
}

package auth

import (
	"context"
	"log/slog"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationsdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// SsoLogoutResult contains the logout URLs for front-channel SSO logout.
type SsoLogoutResult struct {
	LogoutUrls []string `json:"logoutUrls"`
}

// SsoLogout performs single sign-out by revoking the current session and
// collecting front-channel logout URLs from all SSO-enabled applications
// in the user's organization.
//
// The session is revoked (revoked_at stamped, row retained for the audit
// trail), never physically deleted, and the revoke plus its audit record
// commit in one transaction; a repository failure surfaces to the caller
// instead of being logged and reported as success (BE-07).
func (s *Service) SsoLogout(ctx context.Context, sessionID string) (*SsoLogoutResult, error) {
	if sessionID == "" {
		return &SsoLogoutResult{LogoutUrls: []string{}}, nil
	}

	session, err := s.UOW.Sessions().GetByID(ctx, sessionID)
	if err != nil {
		if isIgnorableLogoutError(err) {
			return &SsoLogoutResult{LogoutUrls: []string{}}, nil
		}
		return nil, apperrors.WrapInternalf(err, "lookup session %s for SSO logout", sessionID)
	}
	if session == nil {
		return &SsoLogoutResult{LogoutUrls: []string{}}, nil
	}

	now := s.Clock.Now().UTC()
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if revokeErr := tx.Sessions().Revoke(ctx, sessionID, now); revokeErr != nil && !isIgnorableLogoutError(revokeErr) {
			return apperrors.WrapInternalf(revokeErr, "revoke session %s for SSO logout", sessionID)
		}
		return s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			session.OrganizationID, session.UserID,
			"user.sso_logout", "user", session.UserID, "user",
			audit.EventAuthLogout,
			map[string]any{"sessionId": sessionID},
		)
	}); err != nil {
		return nil, err
	}

	// Front-channel logout URLs are best-effort reads after the local session
	// is already revoked; a listing failure degrades to an empty list.
	logoutUrls := []string{}
	apps, _, appErr := s.UOW.Applications().List(ctx, applicationsdomain.ListQuery{
		OrganizationID: session.OrganizationID,
		Page:           1,
		PageSize:       100,
	})
	if appErr != nil {
		slog.WarnContext(ctx, "failed to list SSO applications during logout", "error", appErr)
		return &SsoLogoutResult{LogoutUrls: logoutUrls}, nil
	}

	for _, app := range apps {
		if app.LogoutUrl != "" {
			logoutUrls = append(logoutUrls, app.LogoutUrl)
		}
	}

	return &SsoLogoutResult{LogoutUrls: logoutUrls}, nil
}

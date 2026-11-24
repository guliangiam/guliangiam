package iga

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PrivilegedSessionDTO struct {
	SessionID    string    `json:"sessionId"`
	LifecycleID  string    `json:"lifecycleId"`
	TicketID     string    `json:"ticketId"`
	Privileged   bool      `json:"privileged"`
	SessionState string    `json:"sessionState"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func toPrivilegedSessionDTO(session *sessionsdomain.Session, now time.Time) PrivilegedSessionDTO {
	return PrivilegedSessionDTO{
		SessionID: session.ID, LifecycleID: session.LifecycleID, TicketID: session.TicketID,
		Privileged: session.IsPrivileged && session.IsActive(now), SessionState: string(session.SessionState), ExpiresAt: session.ExpiresAt,
	}
}

func (s Service) StartPrivilegedSession(ctx context.Context, cmd AccessRequestActionCommand, sessionID string) (*PrivilegedSessionDTO, error) {
	if sessionID == "" {
		return nil, apperrors.NewInvalidArgument("sessionId is required")
	}
	var result *PrivilegedSessionDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		request, err := s.lockRequest(ctx, tx, cmd, domain.RequestProvisioned)
		if err != nil {
			return err
		}
		if request.RequesterID != cmd.ActorID || request.LifecycleID == "" {
			return apperrors.NewForbidden("privileged session requires an approved provisioned request")
		}
		now := s.now()
		if now.Before(request.StartsAt) || !now.Before(request.ExpiresAt) {
			return apperrors.NewForbidden("privileged request is outside its time window")
		}
		lifecycle, err := tx.IGA().GetLifecycleForUpdate(ctx, cmd.OrganizationID, request.LifecycleID)
		if err != nil {
			return err
		}
		if lifecycle.Status != domain.LifecycleActive || now.Before(lifecycle.StartsAt) || !now.Before(lifecycle.ExpiresAt) {
			return apperrors.NewForbidden("privileged grant is not active")
		}
		session, err := tx.Sessions().GetByID(ctx, sessionID)
		if err != nil {
			return err
		}
		if session.UserID != cmd.ActorID || session.OrganizationID != cmd.OrganizationID || !session.IsActive(now) {
			return apperrors.NewForbidden("session cannot be elevated")
		}
		if session.IsPrivileged {
			if session.LifecycleID != lifecycle.ID {
				return apperrors.NewConflict("session is already elevated by another lifecycle")
			}
			dto := toPrivilegedSessionDTO(session, now)
			result = &dto
			return nil
		}
		session.IsPrivileged = true
		session.LifecycleID = lifecycle.ID
		session.TicketID = request.ID
		if session.ExpiresAt.After(lifecycle.ExpiresAt) {
			session.ExpiresAt = lifecycle.ExpiresAt
		}
		session.UpdatedAt = now
		if err := tx.Sessions().Update(ctx, session); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			cmd.OrganizationID,
			cmd.ActorID,
			"iga.pam.session_started",
			"session",
			session.ID,
			"privileged_session",
			"iga.pam.session_started",
			map[string]any{"sessionId": session.ID, keyTicketID: request.ID, keyLifecycleID: lifecycle.ID, keyPrivileged: true, keyExpiresAt: session.ExpiresAt},
		); err != nil {
			return err
		}
		dto := toPrivilegedSessionDTO(session, now)
		result = &dto
		return nil
	})
	return result, err
}

func (s Service) ListPrivilegedSessions(ctx context.Context, orgID, userID string) ([]PrivilegedSessionDTO, error) {
	if orgID == "" || userID == "" {
		return nil, apperrors.NewInvalidArgument("organization and user are required")
	}
	var sessions []*sessionsdomain.Session
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		sessions, _, err = tx.Sessions().List(ctx, sessionsdomain.ListQuery{OrganizationID: orgID, UserID: userID, Page: 1, PageSize: 500})
		return err
	})
	if err != nil {
		return nil, err
	}
	now := s.now()
	result := make([]PrivilegedSessionDTO, 0)
	for _, session := range sessions {
		if session.LifecycleID == "" {
			continue
		}
		result = append(result, toPrivilegedSessionDTO(session, now))
	}
	return result, nil
}

// AuditPrivilegedOperation is called by the HTTP middleware for every allowed
// operation made by a privileged session. It rechecks the live session and JIT
// window before writing the audit record and real-time Outbox alert.
func (s Service) AuditPrivilegedOperation(ctx context.Context, orgID, sessionID, actorID, action, resource string) error {
	if orgID == "" || sessionID == "" || actorID == "" {
		return apperrors.NewInvalidArgument("organization, session and actor are required")
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		session, err := tx.Sessions().GetByID(ctx, sessionID)
		if err != nil {
			return err
		}
		now := s.now()
		if !session.IsPrivileged || session.LifecycleID == "" || session.UserID != actorID || session.OrganizationID != orgID || !session.IsActive(now) {
			return apperrors.NewForbidden("privileged session is not active")
		}
		lifecycle, err := tx.IGA().GetLifecycle(ctx, session.LifecycleID, orgID)
		if err != nil {
			return err
		}
		if lifecycle.Status != domain.LifecycleActive || now.Before(lifecycle.StartsAt) || !now.Before(lifecycle.ExpiresAt) {
			return apperrors.NewForbidden("privileged grant is outside its time window")
		}
		return s.embedAuditEvent(
			tx,
			ctx,
			orgID,
			actorID,
			"iga.pam.operation",
			"session",
			sessionID,
			"privileged_session",
			"iga.pam.operation",
			map[string]any{"sessionId": sessionID, keyTicketID: session.TicketID, keyLifecycleID: lifecycle.ID, keyPrivileged: true, "action": action, "resource": resource},
		)
	})
}

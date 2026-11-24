package sessions

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

type RevokeOtherSessionsResult struct {
	RevokedCount int64
}

func (s *Service) RevokeOtherSessions(ctx context.Context, currentSessionID string) (*RevokeOtherSessionsResult, error) {
	currentSession, err := s.requireActiveSession(ctx, currentSessionID)
	if err != nil {
		return nil, err
	}

	var count int64
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		c, err := tx.Sessions().RevokeByUserExcept(
			ctx, currentSession.UserID, currentSessionID, s.Clock.Now().UTC(),
		)
		if err != nil {
			return err
		}
		count = c

		if err := s.enqueueSSFRevoked(ctx, tx, currentSession.OrganizationID, currentSession.UserID, "", "other_sessions_revoked"); err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			currentSession.OrganizationID,
			currentSession.UserID,
			"session.revoked.others",
			"session",
			currentSessionID,
			"session",
			"session.revoked.others",
			map[string]any{
				"currentSessionId": currentSessionID,
				"revokedCount":     count,
				keyOrgID:           currentSession.OrganizationID,
			},
		)
	})
	if err != nil {
		return nil, err
	}

	return &RevokeOtherSessionsResult{RevokedCount: count}, nil
}

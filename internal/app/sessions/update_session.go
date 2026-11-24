package sessions

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// UpdateSessionCommand contains parameters for updating a session.
type UpdateSessionCommand struct {
	ID           string
	SessionState *string
	Revision     int64
}

// UpdateSession updates mutable session fields such as state.
func (s *Service) UpdateSession(ctx context.Context, cmd UpdateSessionCommand) error {
	return s.updateSession(ctx, cmd, "")
}

// UpdateSessionForOrganization updates a session only when its owning user
// belongs to organizationID. The scoped read runs inside the same transaction,
// so a foreign session is rejected (404) before any write.
func (s *Service) UpdateSessionForOrganization(ctx context.Context, cmd UpdateSessionCommand, organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return s.updateSession(ctx, cmd, organizationID)
}

func (s *Service) updateSession(ctx context.Context, cmd UpdateSessionCommand, organizationID string) error {
	if strings.TrimSpace(cmd.ID) == "" {
		return apperrors.NewInvalidArgument("session id is required")
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var (
			session *domainsessions.Session
			err     error
		)
		if organizationID == "" {
			session, err = tx.Sessions().GetByID(ctx, cmd.ID)
		} else {
			session, err = tx.Sessions().GetByIDForOrganization(ctx, cmd.ID, organizationID)
		}
		if err != nil {
			return err
		}

		if cmd.Revision > 0 && session.Revision != cmd.Revision {
			return apperrors.NewConflict("session revision mismatch")
		}

		changed := false
		if cmd.SessionState != nil && *cmd.SessionState != string(session.SessionState) {
			if err := session.SetState(domainsessions.State(*cmd.SessionState)); err != nil {
				return err
			}
			changed = true
		}

		if !changed {
			return nil
		}

		session.UpdatedAt = s.Clock.Now()
		session.Revision++

		if err := tx.Sessions().Update(ctx, session); err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			session.OrganizationID,
			session.UserID,
			"session.updated",
			"session",
			session.ID,
			"session",
			"session.updated",
			map[string]any{keySessionID: session.ID, "newState": string(session.SessionState)},
		)
	})
}

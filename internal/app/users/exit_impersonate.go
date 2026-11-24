package users

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ExitImpersonateCommand is the command for exiting an impersonation session.
type ExitImpersonateCommand struct {
	OrganizationID string
	UserID         string
	// ActorID is the operator user id (empty for system actions).
	ActorID string
}

// ExitImpersonate revokes the impersonation token for a user, ending the
// impersonation session. It only touches tokens minted by ImpersonateUser;
// the user's own tokens must survive.
func (s *Service) ExitImpersonate(ctx context.Context, command ExitImpersonateCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// A foreign user must be indistinguishable from a missing one: resolve
		// ownership before revoking any token.
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		// Impersonation tokens are identified by the "imp" token prefix set in
		// ImpersonateUser. Revoking by org+user alone would also kill the user's
		// regular tokens, which is a forced logout rather than an exit-impersonate.
		// The organization predicate keeps MySQL (no RLS) on the same tenant boundary.
		if _, err := tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{
			OrganizationID: command.OrganizationID,
			UserID:         command.UserID,
			TokenPrefix:    "imp",
		}, s.Clock.Now()); err != nil {
			return apperrors.NewInternal("revoke impersonation tokens")
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			command.ActorID,
			"user.impersonation.ended",
			"User",
			user.ID,
			"user",
			"user.impersonation.ended",
			map[string]any{},
		)
	})
}

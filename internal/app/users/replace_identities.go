package users

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ReplaceUserIdentityCommand struct {
	ID              string
	ProviderID      string
	IdentityType    string
	Subject         string
	ProviderAccount string
	ProfileSnapshot map[string]any
}

func (s *Service) ReplaceUserIdentities(ctx context.Context, organizationID, userID string, commands []ReplaceUserIdentityCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validateReplaceUserIdentitiesCommands(commands); err != nil {
		return err
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// {id} comes from the URL: resolve the aggregate through the trusted
		// tenant before replacing any child identity.
		user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}

		// A client-supplied identity id must already belong to this user.
		// Omitting id creates a new identity; naming another user's id (even in
		// the same tenant) rejects the whole replacement as NotFound.
		existing := make(map[string]struct{}, len(user.Identities))
		for _, identity := range user.Identities {
			existing[identity.ID] = struct{}{}
		}
		for _, command := range commands {
			if command.ID == "" {
				continue
			}
			if _, ok := existing[command.ID]; !ok {
				return apperrors.NewNotFound("identity not found")
			}
		}

		now := s.Clock.Now()
		identities := make([]userdomain.UserIdentity, 0, len(commands))
		for _, command := range commands {
			identityID := command.ID
			if identityID == "" {
				generatedID, err := s.UUID.NewString()
				if err != nil {
					return err
				}
				identityID = generatedID
			}

			identities = append(identities, userdomain.UserIdentity{
				ID:              identityID,
				UserID:          userID,
				ProviderID:      command.ProviderID,
				IdentityType:    command.IdentityType,
				Subject:         command.Subject,
				ProviderAccount: command.ProviderAccount,
				ProfileSnapshot: command.ProfileSnapshot,
				LinkedAt:        now,
			})
		}

		user.Identities = identities
		user.UpdatedAt = now
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.identity.replaced",
			"User",
			user.ID,
			"user",
			"user.identity.replaced",
			map[string]any{},
		); err != nil {
			return err
		}
		return tx.Users().ReplaceIdentities(ctx, userID, identities)
	})
}

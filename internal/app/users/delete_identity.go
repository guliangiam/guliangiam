package users

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DeleteUserIdentity removes a specific identity from a user by identity ID.
func (s *Service) DeleteUserIdentity(ctx context.Context, organizationID, userID, identityID string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if strings.TrimSpace(userID) == "" {
		return apperrors.NewInvalidArgument("user ID is required")
	}
	if strings.TrimSpace(identityID) == "" {
		return apperrors.NewInvalidArgument("identity ID is required")
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// {id} comes from the URL: prove the parent user belongs to the trusted
		// tenant before touching the identity table.
		user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}

		// The aggregate already contains the user's identities, so a foreign
		// identity id in the same tenant is indistinguishable from a missing one.
		found := false
		for _, identity := range user.Identities {
			if identity.ID == identityID {
				found = true
				break
			}
		}
		if !found {
			return apperrors.NewNotFound("identity not found")
		}

		if err := tx.Users().DeleteIdentity(ctx, userID, identityID); err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.identity.deleted",
			"user",
			userID,
			"user",
			"user.identity.deleted",
			map[string]any{"userId": userID, "identityId": identityID, "orgId": user.OrganizationID},
		)
	})
}

package users

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// RolePolicyManager keeps Casbin role-inheritance rows aligned with the
// persisted user → role bindings.
//
// Authorization evaluates `g(user, role, org)` from Casbin, not the
// `user_role_bindings` table, so every binding write must re-project the
// user's current role set into the policy store.
type RolePolicyManager interface {
	SyncUserRoles(ctx context.Context, orgID, orgSlug, username string, roleNames []string) error
}

// resyncUserRoles reloads a user's role names from the database and rewrites
// their Casbin grouping rows. It is called after binding mutations commit.
func (s *Service) resyncUserRoles(ctx context.Context, userID string) error {
	if s.RolePolicies == nil {
		return nil
	}

	var organizationID, organizationSlug, username string
	var roleNames []string
	err := runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByID(ctx, userID)
		if err != nil {
			return err
		}
		organizationID = user.OrganizationID
		username = user.Username

		organization, err := tx.Organizations().GetByID(ctx, organizationID)
		if err != nil {
			return err
		}
		if organization != nil {
			organizationSlug = organization.Slug
		}

		roleNames, err = tx.Users().ListRoleNames(ctx, userID)
		return err
	})
	if err != nil {
		return err
	}

	if organizationSlug == "" {
		return apperrors.NewInternal("organization slug missing for casbin role sync")
	}
	if syncErr := s.RolePolicies.SyncUserRoles(ctx, organizationID, organizationSlug, username, roleNames); syncErr != nil {
		return apperrors.WrapInternal(syncErr, "sync casbin role inheritance for user "+username)
	}
	return nil
}

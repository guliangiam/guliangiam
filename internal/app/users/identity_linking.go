package users

import (
	"context"
	"time"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// LinkIdentityCommand links a third-party identity to a user.
type LinkIdentityCommand struct {
	UserID       string
	Provider     string
	ProviderType string
	ExternalID   string
	DisplayName  string
}

// UnlinkIdentityCommand removes a third-party identity from a user.
type UnlinkIdentityCommand struct {
	OrganizationID string
	UserID         string
	IdentityID     string
}

// UserIdentity represents a linked third-party identity.
type UserIdentity struct {
	ID           string
	Provider     string
	ProviderType string
	ExternalID   string
	DisplayName  string
	CreatedTime  string
}

// LinkIdentity associates a third-party identity (OAuth, SAML, LDAP) with a user account.
func (s *Service) LinkIdentity(ctx context.Context, cmd LinkIdentityCommand) error {
	if cmd.UserID == "" {
		return apperrors.NewInvalidArgument("user ID is required")
	}
	if cmd.Provider == "" {
		return apperrors.NewInvalidArgument("provider is required")
	}
	if cmd.ExternalID == "" {
		return apperrors.NewInvalidArgument("external ID is required")
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx uow.UnitOfWork) error {
		// Verify user exists
		_, err := tx.Users().GetByID(ctx, cmd.UserID)
		if err != nil {
			return apperrors.WrapInternal(err, "user not found")
		}

		// Create identity record
		identityID, err := s.UUID.NewString()
		if err != nil {
			return apperrors.WrapInternal(err, "generate identity ID")
		}

		identity := &users.UserIdentity{
			ID:              identityID,
			UserID:          cmd.UserID,
			ProviderID:      cmd.Provider,
			IdentityType:    cmd.ProviderType,
			Subject:         cmd.ExternalID,
			ProviderAccount: cmd.DisplayName,
			LinkedAt:        time.Now(),
		}

		return tx.Users().CreateIdentity(ctx, identity)
	})
}

// UnlinkIdentity removes a third-party identity association from a user account.
func (s *Service) UnlinkIdentity(ctx context.Context, cmd UnlinkIdentityCommand) error {
	if err := requireOrganizationID(cmd.OrganizationID); err != nil {
		return err
	}
	if cmd.UserID == "" {
		return apperrors.NewInvalidArgument("user ID is required")
	}
	if cmd.IdentityID == "" {
		return apperrors.NewInvalidArgument("identity ID is required")
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx uow.UnitOfWork) error {
		// A foreign user must be indistinguishable from a missing one: resolve
		// ownership before deleting the identity row.
		if _, err := tx.Users().GetByIDForOrganization(ctx, cmd.UserID, cmd.OrganizationID); err != nil {
			return err
		}

		return tx.Users().DeleteIdentity(ctx, cmd.UserID, cmd.IdentityID)
	})
}

// GetUserIdentities retrieves all third-party identities linked to a user.
func (s *Service) GetUserIdentities(ctx context.Context, userID string) ([]UserIdentity, error) {
	// userID 来自 URL path；GetByID 无 organization_id 条件，跨 org 读取 identities 由调用方作用域与 RLS 决定。
	if userID == "" {
		return nil, apperrors.NewInvalidArgument("user ID is required")
	}

	user, err := s.UOW.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "user not found")
	}

	// Map domain identities to service DTOs
	result := make([]UserIdentity, 0, len(user.Identities))
	for i := range user.Identities {
		id := &user.Identities[i]
		result = append(result, UserIdentity{
			ID:           id.ID,
			Provider:     id.ProviderID,
			ProviderType: id.IdentityType,
			ExternalID:   id.Subject,
			DisplayName:  id.ProviderAccount,
			CreatedTime:  id.LinkedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return result, nil
}

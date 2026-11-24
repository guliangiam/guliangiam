package roles

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchRoleCommand struct {
	ID string
	// ActorID is the operator user id (empty for system actions).
	ActorID     string
	DisplayName *string
	Description *string
	GroupIDs    []string
	Users       []string
	RoleIDs     []string
	Domains     []string
	IsEnabled   *bool
	Revision    int64
}

func (s *Service) PatchRole(ctx context.Context, organizationID string, command PatchRoleCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validatePatchRoleCommand(command); err != nil {
		return err
	}

	var updatedRole *domain.Role

	err := (s.UOW.(appuow.TxRunner)).WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// The scoped lookup decides ownership before any mutation, so a
		// cross-tenant id leaves no write and no audit record behind.
		role, err := tx.Roles().GetByIDForOrganization(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && role.Revision != command.Revision {
			return apperrors.NewConflict("role revision mismatch")
		}

		if command.DisplayName != nil {
			role.DisplayName = *command.DisplayName
		}
		if command.Description != nil {
			role.Description = *command.Description
		}
		if command.GroupIDs != nil {
			role.GroupIDs = append([]string(nil), command.GroupIDs...)
		}
		if command.Users != nil {
			role.Users = append([]string(nil), command.Users...)
		}
		if command.RoleIDs != nil {
			role.RoleIDs = append([]string(nil), command.RoleIDs...)
		}
		if command.Domains != nil {
			role.Domains = append([]string(nil), command.Domains...)
		}
		if command.IsEnabled != nil {
			role.IsEnabled = *command.IsEnabled
		}

		role.UpdatedAt = s.Clock.Now()
		role.Revision++

		if err := tx.Roles().Update(ctx, role); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			role.OrganizationID, command.ActorID,
			"role.updated", "Role", role.ID,
			"Role", "role.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		updatedRole = role
		return nil
	})
	if err != nil {
		return err
	}

	// Re-sync Casbin policies after the role update.
	// First remove old policies, then add the updated ones.
	if updatedRole != nil && s.CasbinManager != nil {
		if syncErr := s.CasbinManager.RemoveRole(ctx, updatedRole.OrganizationID, updatedRole.ID); syncErr != nil {
			return apperrors.WrapInternal(syncErr, "patch role: casbin cleanup failed")
		}
		if syncErr := s.CasbinManager.SyncRole(ctx, updatedRole.OrganizationID, updatedRole); syncErr != nil {
			return apperrors.WrapInternal(syncErr, "patch role: casbin sync failed")
		}
	}

	return nil
}

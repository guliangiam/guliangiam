package groups

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type DeleteGroupCommand struct {
	ID string
	// ActorID is the operator user id (empty for system actions).
	ActorID string
}

func (s *Service) DeleteGroup(ctx context.Context, organizationID string, command DeleteGroupCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// The scoped lookup decides ownership before any delete, so a
		// cross-tenant id leaves no write and no audit record behind.
		group, err := tx.Groups().GetByIDForOrganization(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}

		// parent_id 存的是父组的 id（uuid），不是组名。
		childrenCount, err := tx.Groups().CountChildrenByParentID(ctx, group.OrganizationID, group.ID)
		if err != nil {
			return err
		}
		if childrenCount > 0 {
			return apperrors.NewConflict("group has children")
		}

		if err := tx.Groups().Delete(ctx, command.ID, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			group.OrganizationID, command.ActorID,
			"groups.deleted", "groups", command.ID,
			"groups", "groups.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

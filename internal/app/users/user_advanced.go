package users

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// RemoveFromGroupCommand is the command for removing a user from a group.
type RemoveFromGroupCommand struct {
	OrganizationID string
	UserID         string
	GroupID        string
	// ActorID is the operator user id (empty for system actions).
	ActorID string
}

// RemoveUserFromGroup removes a user from a specific group.
func (s *Service) RemoveUserFromGroup(ctx context.Context, command RemoveFromGroupCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		// Filter out the specified group membership.
		var remaining []userdomain.UserGroupMembership
		for _, m := range user.GroupMemberships {
			if m.GroupID != command.GroupID {
				remaining = append(remaining, m)
			}
		}

		if len(remaining) == len(user.GroupMemberships) {
			return apperrors.NewNotFound("group membership not found")
		}

		if err := tx.Users().ReplaceGroupMemberships(ctx, command.UserID, remaining); err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			command.ActorID,
			"user.group_membership.removed",
			"User",
			user.ID,
			"user",
			"user.group_membership.removed",
			map[string]any{},
		)
	})
}

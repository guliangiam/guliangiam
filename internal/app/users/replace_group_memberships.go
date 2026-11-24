package users

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

type ReplaceUserGroupMembershipCommand struct {
	GroupID string
}

func (s *Service) ReplaceUserGroupMemberships(ctx context.Context, organizationID, userID string, commands []ReplaceUserGroupMembershipCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validateReplaceUserGroupMembershipCommands(commands); err != nil {
		return err
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// {id} comes from the URL: resolve the aggregate through the trusted
		// tenant before replacing any child membership.
		user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}

		// Group ids are tenant resources: a foreign or missing group must reject
		// the whole replacement before any write or audit.
		for _, command := range commands {
			if _, err := tx.Groups().GetByIDForOrganization(ctx, command.GroupID, organizationID); err != nil {
				return err
			}
		}

		memberships := make([]userdomain.UserGroupMembership, 0, len(commands))
		for _, command := range commands {
			memberships = append(memberships, userdomain.UserGroupMembership{UserID: userID, GroupID: command.GroupID})
		}

		user.GroupMemberships = memberships
		user.UpdatedAt = s.Clock.Now()
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, organizationID); err != nil {
			return err
		}

		if err := tx.Users().ReplaceGroupMemberships(ctx, userID, memberships); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.group_membership.replaced",
			"User",
			user.ID,
			"user",
			"user.group_membership.replaced",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

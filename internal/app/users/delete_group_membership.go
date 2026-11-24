package users

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

// DeleteUserGroupMembership removes a group membership from a user.
func (s *Service) DeleteUserGroupMembership(ctx context.Context, organizationID, userID, groupID string) error {
	return s.mutateUserLinks(ctx, organizationID, userID, "user.group_membership.removed",
		func(user *userdomain.User) bool {
			memberships := make([]userdomain.UserGroupMembership, 0, len(user.GroupMemberships))
			found := false
			for _, m := range user.GroupMemberships {
				if m.GroupID == groupID {
					found = true
					continue
				}
				memberships = append(memberships, m)
			}
			user.GroupMemberships = memberships
			return found
		},
		func(ctx context.Context, tx appuow.UnitOfWork, user *userdomain.User) error {
			return tx.Users().ReplaceGroupMemberships(ctx, userID, user.GroupMemberships)
		})
}

package users

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DeleteUserRoleBinding removes a role binding from a user.
func (s *Service) DeleteUserRoleBinding(ctx context.Context, organizationID, userID, roleID string) error {
	if strings.TrimSpace(userID) == "" {
		return apperrors.NewInvalidArgument("userID is required")
	}
	if strings.TrimSpace(roleID) == "" {
		return apperrors.NewInvalidArgument("roleID is required")
	}

	err := s.mutateUserLinks(ctx, organizationID, userID, "user.role_binding.removed",
		func(user *userdomain.User) bool {
			bindings := make([]userdomain.UserRoleBinding, 0, len(user.RoleBindings))
			found := false
			for _, b := range user.RoleBindings {
				if b.RoleID == roleID {
					found = true
					continue
				}
				bindings = append(bindings, b)
			}
			user.RoleBindings = bindings
			return found
		},
		func(ctx context.Context, tx appuow.UnitOfWork, user *userdomain.User) error {
			return tx.Users().ReplaceRoleBindings(ctx, userID, user.RoleBindings)
		})
	if err != nil {
		return err
	}
	return s.resyncUserRoles(ctx, userID)
}

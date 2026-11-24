package users

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/app/jml"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

// diffRoleIDs 计算替换前后的角色差集（Mover 出站事件用）。
func diffRoleIDs(oldIDs, newIDs []string) (added, removed []string) {
	oldSet := make(map[string]struct{}, len(oldIDs))
	for _, id := range oldIDs {
		oldSet[id] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newIDs))
	for _, id := range newIDs {
		newSet[id] = struct{}{}
	}
	for _, id := range newIDs {
		if _, ok := oldSet[id]; !ok {
			added = append(added, id)
		}
	}
	for _, id := range oldIDs {
		if _, ok := newSet[id]; !ok {
			removed = append(removed, id)
		}
	}
	return added, removed
}

type ReplaceUserRoleBindingCommand struct {
	RoleID string
}

func (s *Service) ReplaceUserRoleBindings(ctx context.Context, organizationID, userID string, commands []ReplaceUserRoleBindingCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validateReplaceUserRoleBindingCommands(commands); err != nil {
		return err
	}

	if err := runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// {id} comes from the URL: resolve the aggregate through the trusted
		// tenant before replacing any child binding.
		user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}

		// Role ids are tenant resources: a foreign or missing role must reject
		// the whole replacement before any write or audit.
		for _, command := range commands {
			if _, err := tx.Roles().GetByIDForOrganization(ctx, command.RoleID, organizationID); err != nil {
				return err
			}
		}

		bindings := make([]userdomain.UserRoleBinding, 0, len(commands))
		for _, command := range commands {
			bindings = append(bindings, userdomain.UserRoleBinding{UserID: userID, RoleID: command.RoleID})
		}

		oldRoleIDs := make([]string, 0, len(user.RoleBindings))
		for _, binding := range user.RoleBindings {
			oldRoleIDs = append(oldRoleIDs, binding.RoleID)
		}
		newRoleIDs := make([]string, 0, len(commands))
		for _, command := range commands {
			newRoleIDs = append(newRoleIDs, command.RoleID)
		}

		user.RoleBindings = bindings
		user.UpdatedAt = s.Clock.Now()
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, organizationID); err != nil {
			return err
		}

		if err := tx.Users().ReplaceRoleBindings(ctx, userID, bindings); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.role_binding.replaced",
			"User",
			user.ID,
			"user",
			"user.role_binding.replaced",
			map[string]any{},
		); err != nil {
			return err
		}

		if s.JML != nil {
			// Mover（ADR-030）：绑定已落库，JML 做 SoD 双时机复检；
			// 冲突返回 SoDError → 调用方回滚整个替换事务。
			added, removed := diffRoleIDs(oldRoleIDs, newRoleIDs)
			if err := s.JML.OnRoleBindingChanged(ctx, tx, userID, user.OrganizationID, added, removed, jml.SafeUserPayload(user)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	return s.resyncUserRoles(ctx, userID)
}

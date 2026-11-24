package users

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// mutateUserLinks 是「从用户的某个链接集合中移除一项」的共享实现：
// 取 scoped 聚合 → mutate 改写集合 → 乐观锁更新 → 替换子表 → 同事务写审计。
// 角色绑定与群组成员两个用例除类型、集合字段与审计动作外完全相同；
// 共享一份实现可保证事务边界与审计载荷只有一处定义。
//
// mutate 返回 false 表示目标子资源不存在：此时不写、不审计，统一 NotFound。
func (s *Service) mutateUserLinks(
	ctx context.Context,
	organizationID, userID string,
	auditAction string,
	mutate func(*userdomain.User) bool,
	replace func(context.Context, appuow.UnitOfWork, *userdomain.User) error,
) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if strings.TrimSpace(userID) == "" {
		return apperrors.NewInvalidArgument("userID is required")
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// {id} comes from the URL: resolve the aggregate through the trusted
		// tenant before reading or mutating any child collection.
		user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}

		if !mutate(user) {
			return apperrors.NewNotFound("user link not found")
		}
		user.UpdatedAt = s.Clock.Now()
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, organizationID); err != nil {
			return err
		}

		if err := replace(ctx, tx, user); err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			auditAction,
			"User",
			user.ID,
			auditAction,
			auditAction,
			map[string]any{},
		)
	})
}

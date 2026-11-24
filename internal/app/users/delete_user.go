package users

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/app/jml"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type DeleteUserCommand struct {
	OrganizationID string
	ID             string
	// ActorID is the operator user id (empty for system actions).
	ActorID string
	// ActorIsPlatformAdmin 由 handler 从 session.PrincipalType 判定；
	// 非平台管理员不得删除任何平台管理员账号（reset_password.go 同口径）。
	ActorIsPlatformAdmin bool
}

func (s *Service) DeleteUser(ctx context.Context, command DeleteUserCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if command.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}

	var deletedUserID, deletedOrgID, deletedOrgSlug, deletedUsername string
	if err := runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// A foreign user id must look deleted-or-missing, not forbidden: no
		// existence probe, no write, no audit.
		user, err := tx.Users().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}
		// 自删守卫：管理员删除自己走账号注销/自助路径，避免会话内自锁与
		// 审计主体消失（reset_password.go 自重置守卫同款语义）。
		if command.ActorID != "" && command.ActorID == user.ID {
			return apperrors.NewUnprocessableEntity("administrators cannot delete their own account")
		}
		if err := assertCanManagePlatformAdmin(ctx, tx, command.ActorIsPlatformAdmin, user.ID); err != nil {
			return err
		}

		organization, err := tx.Organizations().GetByID(ctx, user.OrganizationID)
		if err != nil {
			return err
		}
		if organization != nil && organization.Slug == "built-in" && user.Username == "admin" {
			return apperrors.NewForbidden("cannot delete built-in admin user")
		}

		now := s.Clock.Now()
		user.DeletedAt = &now
		// PostgreSQL 由 trg_users_sync_is_deleted 触发器从 deleted_at 派生
		// is_deleted；MySQL 镜像无触发器，app 层显式同步保持双方言一致。
		user.IsDeleted = true
		user.UpdatedAt = now
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, command.OrganizationID); err != nil {
			return err
		}

		// Audit the organization loaded from the entity — never an empty value
		// that would fall back to the built-in tenant.
		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			command.ActorID,
			"user.deleted",
			"User",
			user.ID,
			"user",
			"user.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		// Leaver（ADR-030）：JML 注入时统一编排吊销 + 绑定清理 + 下游停用
		// 事件；未注入（MySQL 镜像等）保持原有吊销路径。
		if s.JML != nil {
			if err := s.JML.OnUserDeactivated(ctx, tx, user.ID, user.OrganizationID, jml.SafeUserPayload(user)); err != nil {
				return err
			}
		} else {
			if _, err := tx.Sessions().RevokeByUserExcept(ctx, user.ID, "", now); err != nil {
				return err
			}

			if _, err := tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{UserID: user.ID}, now); err != nil {
				return err
			}
		}

		deletedUserID = user.ID
		deletedOrgID = user.OrganizationID
		deletedOrgSlug = organization.Slug
		deletedUsername = user.Username
		return nil
	}); err != nil {
		return err
	}

	// A soft-deleted user must not keep role inheritance in Casbin.
	if s.RolePolicies != nil && deletedUserID != "" {
		if syncErr := s.RolePolicies.SyncUserRoles(ctx, deletedOrgID, deletedOrgSlug, deletedUsername, nil); syncErr != nil {
			return apperrors.WrapInternal(syncErr, "delete user: clear casbin role inheritance")
		}
	}
	return nil
}

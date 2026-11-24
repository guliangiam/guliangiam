package groups

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchGroupCommand struct {
	ID string
	// ActorID is the operator user id (empty for system actions).
	ActorID      string
	Name         *string
	DisplayName  *string
	Manager      *string
	ContactEmail *string
	Type         *string
	ParentID     *string
	IsTopGroup   *bool
	IsEnabled    *bool
	Homepage     *string
	Users        []string
	Revision     int64
}

func (s *Service) PatchGroup(ctx context.Context, organizationID string, command PatchGroupCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validatePatchGroupCommand(command); err != nil {
		return err
	}
	// 规范 4.10：可编辑资源写路径必须携带乐观锁前提（本端点约定为 body
	// revision）；缺失不再默认最后写入获胜，直接 428。
	if command.Revision <= 0 {
		return apperrors.NewPreconditionRequired("group patch requires a revision precondition")
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// The scoped lookup decides ownership before any mutation, so a
		// cross-tenant id leaves no write and no audit record behind.
		group, err := tx.Groups().GetByIDForOrganization(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}
		updated := cloneGroup(group)

		if updated.Revision != command.Revision {
			// 412 携带当前 revision（规范 4.10：响应体指明当前版本），客户端
			// 重取后可直接重试；409 仅保留给唯一约束等非版本冲突。
			return apperrors.NewWithMeta(apperrors.CodePreconditionFailed,
				"group revision mismatch", map[string]any{"revision": updated.Revision})
		}

		if command.Name != nil {
			updated.Name = normalizeOptionalString(*command.Name)
		}
		if command.DisplayName != nil {
			updated.DisplayName = *command.DisplayName
		}
		if command.Manager != nil {
			updated.Manager = normalizeOptionalString(*command.Manager)
		}
		if command.ContactEmail != nil {
			updated.ContactEmail = normalizeOptionalString(*command.ContactEmail)
		}
		if command.Type != nil {
			updated.Type = normalizeOptionalString(*command.Type)
		}
		if command.ParentID != nil {
			updated.ParentID = normalizeOptionalString(*command.ParentID)
		}
		if command.IsTopGroup != nil {
			updated.IsTopGroup = *command.IsTopGroup
		}
		if command.IsEnabled != nil {
			updated.IsEnabled = *command.IsEnabled
		}
		if command.Homepage != nil {
			updated.Homepage = *command.Homepage
		}
		if err := validateGroupParent(ctx, tx, updated.OrganizationID, updated.ID, updated.ParentID); err != nil {
			return err
		}
		if command.Users != nil {
			normalized, err := normalizeGroupUsers(ctx, tx, updated.OrganizationID, command.Users)
			if err != nil {
				return err
			}
			updated.Users = normalized
		}
		updated.UpdatedAt = s.Clock.Now()
		updated.Revision++

		if err := tx.Groups().Update(ctx, updated); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			updated.OrganizationID, command.ActorID,
			"groups.updated", "groups", updated.ID,
			"groups", "groups.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func cloneGroup(group *domain.Group) *domain.Group {
	if group == nil {
		return nil
	}
	cloned := *group
	return &cloned
}

func normalizeOptionalString(value string) string {
	return strings.TrimSpace(value)
}

package platform_admins

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	sessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformerrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// PrincipalCacheInvalidator drops the principal-type cache entry for a user.
type PrincipalCacheInvalidator interface {
	Invalidate(userID string)
}

// Command / Query 输入
type (
	CreatePlatformAdminCommand struct {
		UserID    string // 应用层 users 表的 id（不是 email/username）
		CreatedBy string
	}

	UpdatePlatformAdminCommand struct {
		ID        string
		CreatedBy string // 允许更换创建者（审计字段）
	}

	SetActiveCommand struct {
		ID     string
		Active bool
	}

	DeletePlatformAdminCommand struct {
		ID string
	}

	ListPlatformAdminsQuery struct {
		Page     int
		PageSize int
		Active   *bool  // nil=不过滤
		Keyword  string // 搜 user_id
	}

	// ── Scope ──

	CreateScopeCommand struct {
		PlatformAdminID string
		ScopeType       string   // 'all' | 'targeted'
		TargetTenants   []string // scope_type='targeted' 时必填
		ApprovedBy      *string  // 审批者的 users.id（FK approved_by → users.id）；nil 表示待审批
		ExpiresAt       time.Time
		CreatedBy       string // 创建 scope 的 user（审计用）
	}

	ApproveScopeCommand struct {
		ScopeID    string
		ApproverID string // 审批者的 users.id（FK approved_by → users.id）
	}

	DeleteScopeCommand struct {
		ScopeID string
	}

	ListScopesQuery struct {
		PlatformAdminID string
		OnlyActive      bool
	}
)

// Service 平台管理员 + 作用域业务层。
// 所有操作走 UoW.WithinTransaction —— RLS (TenantFilterSystem) 由 repository 自动注入。
type Service struct {
	uow   uow.PlatformAdminUoW
	clock clock.Clock
	uuid  platformuuid.Generator
	log   *slog.Logger
	// PrincipalCache 可选：停用平台管理员时立即失效其 principal 分类缓存。
	PrincipalCache PrincipalCacheInvalidator
}

// NewService 构造 PlatformAdminService。
// 参数顺序与 invitations 等保持一致：clock, uuid, uow, log（可选）。
func NewService(c clock.Clock, uid platformuuid.Generator, u uow.PlatformAdminUoW, log *slog.Logger) Service {
	return Service{uow: u, clock: c, uuid: uid, log: log}
}

// ─── helpers ───────────────────────────────────────────────────────────────

// isNotFound 判断 error 是否为 NotFound（repository 返回）。
func isNotFound(err error) bool {
	var pe *platformerrors.Error
	if errors.As(err, &pe) {
		return pe.Code == platformerrors.CodeNotFound
	}
	return false
}

// newID 生成新的 V7 UUID string。
func (s *Service) newID() (string, error) {
	return s.uuid.NewString()
}

// ─── PlatformAdmin ─────────────────────────────────────────────────────────

// CreatePlatformAdmin 注册新的平台管理员。
//
//	业务校验：user_id 不能重复（同一用户不能注册两次）。
//	审计字段：created_by 写入传入值（通常是另一个 platform_admin 的 id）。
func (s *Service) CreatePlatformAdmin(ctx context.Context, cmd CreatePlatformAdminCommand) (*platformadmins.PlatformAdmin, error) {
	if cmd.UserID == "" {
		return nil, platformerrors.NewInvalidArgument("user_id is required")
	}
	if cmd.CreatedBy == "" {
		return nil, platformerrors.NewInvalidArgument("created_by is required")
	}

	// 快速检查——事务外查是否已存在。
	existing, err := s.uow.PlatformAdmins().GetByUserID(ctx, cmd.UserID)
	if err != nil && !isNotFound(err) {
		return nil, platformerrors.WrapInternal(err, "lookup platform admin")
	}
	if existing != nil {
		return nil, platformerrors.NewConflict("user is already registered as platform admin")
	}

	var created *platformadmins.PlatformAdmin
	err = s.uow.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		// 二次检查（并发安全）：NotFound 表示可创建，其余错误必须上抛，
		// 不能吞错后当作"不存在"继续插入（BE-A-13）。
		again, err := tx.PlatformAdmins().GetByUserID(ctx, cmd.UserID)
		if err != nil && !isNotFound(err) {
			return err
		}
		if again != nil {
			return platformerrors.NewConflict("user is already registered as platform admin")
		}

		id, err := s.newID()
		if err != nil {
			return platformerrors.WrapInternal(err, "generate id")
		}
		now := s.clock.Now()
		pa := &platformadmins.PlatformAdmin{
			ID:        id,
			UserID:    cmd.UserID,
			CreatedBy: cmd.CreatedBy,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
			Revision:  1,
		}
		if err := tx.PlatformAdmins().Create(ctx, pa); err != nil {
			return platformerrors.WrapInternal(err, "insert platform admin")
		}
		created = pa
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// UpdatePlatformAdmin 更新审计字段（created_by）。
// 乐观锁通过 revision 实现。
func (s *Service) UpdatePlatformAdmin(ctx context.Context, cmd UpdatePlatformAdminCommand) (*platformadmins.PlatformAdmin, error) {
	if cmd.ID == "" {
		return nil, platformerrors.NewInvalidArgument("id is required")
	}

	var updated *platformadmins.PlatformAdmin
	err := s.uow.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		pa, err := tx.PlatformAdmins().GetByID(ctx, cmd.ID)
		if err != nil {
			return err
		}
		pa.CreatedBy = cmd.CreatedBy
		pa.UpdatedAt = s.clock.Now()
		if err := tx.PlatformAdmins().Update(ctx, pa); err != nil {
			return err
		}
		updated = pa
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// SetActive 启用/停用平台管理员。
//
// 停用必须立即生效：同一事务内先吊销该用户的全部既有会话，事务提交后再主动
// 失效 principalType 缓存。否则停用后最多 60s 内仍会以 platform_admin 主体
// 通过 Gate 5/7，且既有会话继续有效（OPEN-API-55）。
func (s *Service) SetActive(ctx context.Context, cmd SetActiveCommand) error {
	if cmd.ID == "" {
		return platformerrors.NewInvalidArgument("id is required")
	}

	var userID string
	var revokedSessions int64
	err := s.uow.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		admin, err := tx.PlatformAdmins().GetByID(ctx, cmd.ID)
		if err != nil {
			return err
		}
		userID = admin.UserID
		if err := tx.PlatformAdmins().SetActive(ctx, cmd.ID, cmd.Active); err != nil {
			return err
		}
		if cmd.Active {
			return nil
		}
		// Narrow capability: PlatformAdminUoW intentionally does not declare
		// Sessions(), so only the concrete UoW (and test doubles) opt in.
		sessionTx, ok := tx.(interface {
			Sessions() sessions.Repository
		})
		if !ok {
			// Fail-closed（BE-A-06）：停用必须吊销会话；若当前 UoW 不具备
			// Sessions 能力，宁可拒绝停用也不能让管理员停留在"已停用但会话仍有效"。
			return platformerrors.NewInternal("session revocation unavailable: cannot safely deactivate platform admin")
		}
		rows, err := sessionTx.Sessions().RevokeByUserExcept(ctx, userID, "", s.clock.Now())
		if err != nil {
			return err
		}
		revokedSessions = rows
		return nil
	})
	if err != nil {
		return err
	}
	if !cmd.Active && s.PrincipalCache != nil {
		s.PrincipalCache.Invalidate(userID)
	}
	if s.log != nil && !cmd.Active {
		s.log.InfoContext(ctx, "platform admin deactivated", "platform_admin_id", cmd.ID, "user_id", userID, "revoked_sessions", revokedSessions)
	}
	return nil
}

// DeletePlatformAdmin 硬删除平台管理员及其 scopes（ON DELETE CASCADE）。
//
//	注意：不可逆。生产环境建议用 SetActive(false) 代替。
//	删除与停用一样必须立即吊销该用户的全部会话并失效 principal 缓存（BE-A-06），
//	否则被删除的管理员既有会话仍会以 platform_admin 主体通过 Gate。
func (s *Service) DeletePlatformAdmin(ctx context.Context, cmd DeletePlatformAdminCommand) error {
	if cmd.ID == "" {
		return platformerrors.NewInvalidArgument("id is required")
	}
	var userID string
	err := s.uow.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		admin, err := tx.PlatformAdmins().GetByID(ctx, cmd.ID)
		if err != nil {
			return err
		}
		userID = admin.UserID
		if err := tx.PlatformAdmins().Delete(ctx, cmd.ID); err != nil {
			return err
		}
		sessionTx, ok := tx.(interface {
			Sessions() sessions.Repository
		})
		if !ok {
			return platformerrors.NewInternal("session revocation unavailable: cannot safely delete platform admin")
		}
		if _, err := sessionTx.Sessions().RevokeByUserExcept(ctx, userID, "", s.clock.Now()); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if s.PrincipalCache != nil {
		s.PrincipalCache.Invalidate(userID)
	}
	return nil
}

// ListPlatformAdmins 分页查询平台管理员。只读。
func (s *Service) ListPlatformAdmins(ctx context.Context, q ListPlatformAdminsQuery) ([]*platformadmins.PlatformAdmin, int64, error) {
	opts := platformadmins.ListOptions{
		Page:     q.Page,
		PageSize: q.PageSize,
		Active:   q.Active,
		Keyword:  q.Keyword,
	}
	return s.uow.PlatformAdmins().List(ctx, opts)
}

// GetPlatformAdmin 按 ID 查询。只读。
func (s *Service) GetPlatformAdmin(ctx context.Context, id string) (*platformadmins.PlatformAdmin, error) {
	return s.uow.PlatformAdmins().GetByID(ctx, id)
}

// GetPlatformAdminByUserID 按 users 表的 id 查询。只读。
func (s *Service) GetPlatformAdminByUserID(ctx context.Context, userID string) (*platformadmins.PlatformAdmin, error) {
	return s.uow.PlatformAdmins().GetByUserID(ctx, userID)
}

// ─── Scope ──────────────────────────────────────────────────────────────────

// CreateScope 为平台管理员创建作用域。
//
//	业务校验：
//	  - scope_type 必须是 'all' 或 'targeted'
//	  - scope_type='targeted' 时 target_tenants 不能为空
//	  - approved_by 必填：未审批 scope 只能由种子/运维以 is_bootstrap 写入，
//	    应用层不存在创建未审批 scope 的入口
//	  - expires_at 最长 1 年、不能过期（DB CHECK 双保险）
//	  - no self-approval — 传了 approved_by 必须是不同的 active admin
func (s *Service) CreateScope(ctx context.Context, cmd CreateScopeCommand) (*platformadmins.PlatformAdminScope, error) {
	if cmd.PlatformAdminID == "" {
		return nil, platformerrors.NewInvalidArgument("platform_admin_id is required")
	}
	if cmd.ApprovedBy == nil {
		return nil, platformerrors.NewInvalidArgument("approved_by is required: unapproved scopes can only be created by seed bootstrap")
	}
	if !platformadmins.ValidScopeTypes[cmd.ScopeType] {
		return nil, platformerrors.NewInvalidArgument(fmt.Sprintf("invalid scope_type: %s (must be 'all' or 'targeted')", cmd.ScopeType))
	}
	if cmd.ScopeType == platformadmins.ScopeTypeTargeted && len(cmd.TargetTenants) == 0 {
		return nil, platformerrors.NewInvalidArgument("targeted scope requires at least one target_tenant")
	}
	if cmd.ExpiresAt.IsZero() {
		return nil, platformerrors.NewInvalidArgument("expires_at is required")
	}
	if cmd.ExpiresAt.Before(s.clock.Now()) {
		return nil, platformerrors.NewInvalidArgument("expires_at cannot be in the past")
	}
	if cmd.ExpiresAt.After(s.clock.Now().Add(365 * 24 * time.Hour)) {
		return nil, platformerrors.NewInvalidArgument("expires_at must be within 1 year")
	}

	var created *platformadmins.PlatformAdminScope
	err := s.uow.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		// 校验 admin 存在且 active
		admin, err := tx.PlatformAdmins().GetByID(ctx, cmd.PlatformAdminID)
		if err != nil {
			return platformerrors.WrapInternal(err, "lookup platform admin")
		}
		if !admin.IsActive {
			return platformerrors.NewForbidden("cannot create scope for inactive platform admin")
		}

		// no self-approval（approved_by 语义为 users.id，与 Schema FK 一致）
		if cmd.ApprovedBy != nil {
			approver, err := tx.PlatformAdmins().GetByUserID(ctx, *cmd.ApprovedBy)
			if err != nil {
				return platformerrors.WrapInternal(err, "lookup approver")
			}
			if !approver.IsActive {
				return platformerrors.NewForbidden("approver must be an active platform admin")
			}
			if *cmd.ApprovedBy == admin.UserID {
				return platformerrors.NewForbidden("no self-approval: cannot approve your own scope")
			}
		}

		id, err := s.newID()
		if err != nil {
			return platformerrors.WrapInternal(err, "generate id")
		}
		now := s.clock.Now()
		var approvedAt *time.Time
		if cmd.ApprovedBy != nil {
			approvedAt = &now
		}

		scope := &platformadmins.PlatformAdminScope{
			ID:              id,
			PlatformAdminID: cmd.PlatformAdminID,
			ScopeType:       cmd.ScopeType,
			TargetTenants:   cmd.TargetTenants,
			ApprovedBy:      cmd.ApprovedBy,
			ApprovedAt:      approvedAt,
			ExpiresAt:       cmd.ExpiresAt,
			CreatedAt:       now,
		}
		if err := tx.PlatformAdminScopes().Create(ctx, scope); err != nil {
			return platformerrors.WrapInternal(err, "insert scope")
		}
		created = scope
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// ApproveScope 审批待审批的作用域。
//
//	业务校验：scope 必须存在且未审批；approver（users.id）必须是 active admin；
//	approver ≠ 目标 admin 的 user（no self-approval）。
func (s *Service) ApproveScope(ctx context.Context, cmd ApproveScopeCommand) error {
	if cmd.ScopeID == "" || cmd.ApproverID == "" {
		return platformerrors.NewInvalidArgument("scope_id and approver_id are required")
	}
	return s.uow.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		scope, err := tx.PlatformAdminScopes().GetByID(ctx, cmd.ScopeID)
		if err != nil {
			return platformerrors.WrapInternal(err, "lookup scope")
		}
		if scope.ApprovedBy != nil {
			return platformerrors.NewForbidden("scope is already approved")
		}
		admin, err := tx.PlatformAdmins().GetByID(ctx, scope.PlatformAdminID)
		if err != nil {
			return platformerrors.WrapInternal(err, "lookup scope owner")
		}
		if cmd.ApproverID == admin.UserID {
			return platformerrors.NewForbidden("no self-approval: cannot approve your own scope")
		}

		approver, err := tx.PlatformAdmins().GetByUserID(ctx, cmd.ApproverID)
		if err != nil {
			return platformerrors.WrapInternal(err, "lookup approver")
		}
		if !approver.IsActive {
			return platformerrors.NewForbidden("approver must be an active platform admin")
		}

		if err := tx.PlatformAdminScopes().Approve(ctx, cmd.ScopeID, cmd.ApproverID); err != nil {
			return err
		}

		if s.log != nil {
			s.log.InfoContext(ctx, "platform_admins.scope.approved",
				"scope_id", cmd.ScopeID,
				"admin_id", scope.PlatformAdminID,
				"approver_id", cmd.ApproverID,
			)
		}
		return nil
	})
}

// DeleteScope 删除作用域。
func (s *Service) DeleteScope(ctx context.Context, cmd DeleteScopeCommand) error {
	if cmd.ScopeID == "" {
		return platformerrors.NewInvalidArgument("scope_id is required")
	}
	return s.uow.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		return tx.PlatformAdminScopes().Delete(ctx, cmd.ScopeID)
	})
}

// ListScopes 列出指定平台管理员的作用域。只读。
func (s *Service) ListScopes(ctx context.Context, q ListScopesQuery) ([]*platformadmins.PlatformAdminScope, error) {
	if q.PlatformAdminID == "" {
		return nil, platformerrors.NewInvalidArgument("platform_admin_id is required")
	}
	return s.uow.PlatformAdminScopes().ListByPlatformAdmin(ctx, q.PlatformAdminID, platformadmins.ScopeListOptions{
		OnlyActive: q.OnlyActive,
	})
}

// GetScope 按 ID 查询作用域。只读。
func (s *Service) GetScope(ctx context.Context, scopeID string) (*platformadmins.PlatformAdminScope, error) {
	return s.uow.PlatformAdminScopes().GetByID(ctx, scopeID)
}

// ─── 纯函数辅助（可用于 API 层前置校验，不涉及 DB） ──────────────────────────

// ValidateScopeCreationFields 校验 scope 创建字段的合法性（纯函数，不查 DB）。
func ValidateScopeCreationFields(scopeType string, targetTenants []string, expiresAt time.Time) error {
	if !platformadmins.ValidScopeTypes[scopeType] {
		return platformerrors.NewInvalidArgument(fmt.Sprintf("invalid scope_type: %s", scopeType))
	}
	if scopeType == platformadmins.ScopeTypeTargeted && len(targetTenants) == 0 {
		return platformerrors.NewInvalidArgument("targeted scope requires target_tenants")
	}
	if expiresAt.IsZero() {
		return platformerrors.NewInvalidArgument("expires_at is required")
	}
	if expiresAt.Before(time.Now()) {
		return platformerrors.NewInvalidArgument("expires_at cannot be in the past")
	}
	if expiresAt.After(time.Now().Add(365 * 24 * time.Hour)) {
		return platformerrors.NewInvalidArgument("expires_at must be within 1 year")
	}
	return nil
}

// IsValidUUID 轻量校验 UUID 格式（API 层前置校验用）。
func IsValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

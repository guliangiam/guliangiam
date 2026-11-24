package scopeauthorizer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ErrNoRepository 当 ScopeAuthorizer 没有注入必需的 repository 时返回（fail-closed）。
var ErrNoRepository = errors.New("AllowIfScopedScopeAuthorizer: required repository not configured")

// AllowIfScopedScopeAuthorizer 是真正的 ScopeAuthorizer 实现。
//
// 所有 DB 访问通过 repository 接口完成 — 不持有 *pgxpool.Pool。
// repository 内部自动注入 TenantFilterSystem ctx（RLS: is_super_admin()）。
//
// Gate 顺序（安全版本，Phase D-Step 3 + Advisor 漏洞修复）：
//  1. system → bypass
//  2. 主体不是 platform_admin → deny（Gate 2 在 Gate 3 ticket check 之前，避免非 admin 浪费 ticket 查询）
//  3. RiskCritical → ticketRepo.HasApprovedTicket
//  4. adminRepo/scopeRepo/ticketRepo 任一 nil → ErrNoRepository
//  5. adminRepo.GetByUserID → membership + active
//  6. same-tenant → allow
//  7. scopeRepo.HasActiveScopeForTenant → scope 覆盖
//  8. RiskHigh → ticketRepo.HasApprovedTicket
//  9. allow
//
// actor_id 语义：req.ActorID 是 users 表的 id（V5 schema actor_id 存 user_id）。
//
//	Gate 2/8 ticket 查询直接用 req.ActorID.String()，无需先查 membership。
//	Gate 5 membership 需要 platform_admins.id — 通过 adminRepo.GetByUserID 转换。
type AllowIfScopedScopeAuthorizer struct {
	adminRepo   platformadmins.PlatformAdminRepository
	scopeRepo   platformadmins.PlatformAdminScopeRepository
	ticketRepo  crosstenantops.TicketRepository
	log         *slog.Logger
	auditWriter *AuditWriter
}

// NewAllowIfScopedScopeAuthorizer 构造真正实现。
//
//	Dependencies（全部为 domain 接口，无 infra/pgx 依赖）：
//	  - adminRepo:   PlatformAdminRepository (Gate 5 membership)
//	  - scopeRepo:   PlatformAdminScopeRepository (Gate 7 scope 覆盖)
//	  - ticketRepo:  CrossTenantOperationRepository (Gate 2/8 ticket 查询)
//	  - log:         可选，nil 时跳过结构化日志
//	  - auditWriter: 可选，nil 时跳过审计
func NewAllowIfScopedScopeAuthorizer(
	adminRepo platformadmins.PlatformAdminRepository,
	scopeRepo platformadmins.PlatformAdminScopeRepository,
	ticketRepo crosstenantops.TicketRepository,
	log *slog.Logger,
	auditWriter *AuditWriter,
) *AllowIfScopedScopeAuthorizer {
	return &AllowIfScopedScopeAuthorizer{
		adminRepo:   adminRepo,
		scopeRepo:   scopeRepo,
		ticketRepo:  ticketRepo,
		log:         log,
		auditWriter: auditWriter,
	}
}

// logAudit 写入审计记录（失败不阻塞决策）。
func (a *AllowIfScopedScopeAuthorizer) logAudit(ctx context.Context, req AuthorizationRequest, decision, reason string) {
	if a.auditWriter == nil {
		return
	}
	a.auditWriter.Append(ctx, AuditRecord{
		ActorID:      req.ActorID,
		ActorType:    req.ActorType,
		SourceTenant: req.SourceTenant,
		TargetTenant: req.TargetTenant,
		ResourceType: req.ResourceType,
		Action:       req.Action,
		RiskLevel:    req.RiskLevel,
		Decision:     decision,
		Reason:       reason,
	})
}

// Authorize 执行 scope 校验。
//
//	所有 repository=nil / DB error 都 fail-closed（return error → 调用方返回 500）。
//	业务拒绝（not member / inactive / no scope / no ticket）不返回 error，在 AuthorizationResult 里体现。
func (a *AllowIfScopedScopeAuthorizer) Authorize(ctx context.Context, req AuthorizationRequest) (AuthorizationResult, error) {
	// ── Gate 1: system 主体 → 跳过 ─────────────────────────────────────
	if req.ActorType == PrincipalSystem {
		return AuthorizationResult{Allowed: true, Reason: "system principal bypass scope check"}, nil
	}

	// ── Gate 2: 主体不是 platform_admin → deny ────────────────────────
	// 必须在 ticket check 之前 — 避免普通 user 发起的 RiskCritical 请求浪费一次 DB 查询。
	if req.ActorType != PrincipalPlatformAdmin {
		reason := fmt.Sprintf("principal type %s cannot cross-tenant", req.ActorType)
		a.logAudit(ctx, req, "deny", reason)
		return AuthorizationResult{Allowed: false, Reason: reason}, nil
	}

	// ── Gate 3: RiskCritical → 必须有 approved ticket ────────────────
	// 高风险操作（impersonate, batch-delete）**不管是否 same-tenant** 都需要事前审批。
	// 注意：Gate 3 在主体类型校验之后 — ticket 查询直接用 req.ActorID (users.id)。
	if req.RiskLevel == RiskCritical {
		if a.ticketRepo == nil {
			return AuthorizationResult{}, ErrNoRepository
		}
		targetStr := ""
		if req.TargetTenant != uuid.Nil {
			targetStr = req.TargetTenant.String()
		}
		hasTicket, err := a.ticketRepo.HasApprovedTicket(ctx, req.ActorID.String(), targetStr, req.ResourceType, req.Action)
		if err != nil {
			return AuthorizationResult{}, apperrors.WrapInternal(err, "critical-risk ticket check")
		}
		if !hasTicket {
			a.logAudit(ctx, req, "deny", fmt.Sprintf("critical-risk %s operation requires an approved ticket", req.Action))
			return AuthorizationResult{
				Allowed: false,
				Reason:  fmt.Sprintf("critical-risk %s operation requires an approved ticket", req.Action),
			}, nil
		}
	}

	// ── Gate 4: repository 未注入 → fail ───────────────────────────────
	if a.adminRepo == nil || a.scopeRepo == nil || a.ticketRepo == nil {
		return AuthorizationResult{}, ErrNoRepository
	}

	// ── Gate 5: membership + active（通过 repository） ────────────────
	// 必须在 same-tenant allow 之前检查！（Advisor 漏洞修复：防止硬传 ActorType 绕过）
	admin, err := cachedAdminByUserID(ctx, a.adminRepo, req.ActorID.String())
	if err != nil {
		// A lookup failure is indistinguishable from "not an admin" for the caller
		// (fail-closed), but it is logged so infrastructure errors stay visible.
		slog.WarnContext(ctx, "platform admin lookup failed; denying authorization", "actor_id", req.ActorID.String(), "error", err)
		a.logAudit(ctx, req, "deny", "actor is not a registered platform admin")
		return AuthorizationResult{
			Allowed: false,
			Reason:  "actor is not a registered platform admin",
		}, nil
	}
	if !admin.IsActive {
		a.logAudit(ctx, req, "deny", "platform admin account is inactive")
		return AuthorizationResult{
			Allowed: false,
			Reason:  "platform admin account is inactive",
		}, nil
	}

	// ── Gate 6: 本租户 → allow ────────────────────────────────────────
	// 现在在 membership 检查之后 — 只有注册的 platform_admin 才能 same-tenant allow。
	// RiskCritical 已在 Gate 2 过 ticket。
	if req.TargetTenant == uuid.Nil || (req.SourceTenant != uuid.Nil && req.TargetTenant == req.SourceTenant) {
		a.logAudit(ctx, req, "allow", "same-tenant operation")
		return AuthorizationResult{Allowed: true, Reason: "same-tenant operation"}, nil
	}

	// ── Gate 7: scope 覆盖（通过 repository） ────────────────────────
	// 用 admin.ID (platform_admins.id PK) — platform_admin_scopes 表的 FK
	covers, err := a.scopeRepo.HasActiveScopeForTenant(ctx, admin.ID, req.TargetTenant.String())
	if err != nil {
		return AuthorizationResult{}, apperrors.WrapInternal(err, "query scopes")
	}
	if !covers {
		reason := fmt.Sprintf("no active scope covering target tenant %s", req.TargetTenant)
		a.logAudit(ctx, req, "deny", reason)
		return AuthorizationResult{Allowed: false, Reason: reason}, nil
	}

	// ── Gate 8: RiskHigh → 需要 approved ticket ────────────────────────
	// ticket 继续用 req.ActorID (users.id) — cross_tenant_operations.actor_id 存 user_id
	if req.RiskLevel == RiskHigh {
		targetStr := ""
		if req.TargetTenant != uuid.Nil {
			targetStr = req.TargetTenant.String()
		}
		hasTicket, err := a.ticketRepo.HasApprovedTicket(ctx, req.ActorID.String(), targetStr, req.ResourceType, req.Action)
		if err != nil {
			return AuthorizationResult{}, apperrors.WrapInternal(err, "high-risk ticket check")
		}
		if !hasTicket {
			reason := fmt.Sprintf("cross-tenant %s operation requires an approved ticket", req.RiskLevel)
			a.logAudit(ctx, req, "deny", reason)
			return AuthorizationResult{Allowed: false, Reason: reason}, nil
		}
	}

	// ── Gate 9: allow ─────────────────────────────────────────────────
	a.logAudit(ctx, req, "allow", "scope validation passed")
	if a.log != nil {
		a.log.InfoContext(ctx, "scopeauthorizer: allow",
			"actor_type", req.ActorType,
			"actor_id", req.ActorID.String(),
			"target_tenant", req.TargetTenant.String(),
			"resource_type", req.ResourceType,
			"action", req.Action,
			"risk_level", req.RiskLevel,
		)
	}
	return AuthorizationResult{Allowed: true, Reason: "scope validation passed"}, nil
}

// ResolveAdminScope 解析平台管理员的有效作用域快照（all / targeted / none）。
//
// 该结果用于无显式 target 的请求：TenantFilterMiddleware 只有看到 all 才提升
// 为 RLS super_admin；targeted / none / unknown 一律保持 fail-closed。
// 因此解析失败返回 error，由调用方按 unknown（不提升）处理。
func (a *AllowIfScopedScopeAuthorizer) ResolveAdminScope(ctx context.Context, userID string) (AdminScope, error) {
	if a.adminRepo == nil || a.scopeRepo == nil {
		return AdminScope{}, ErrNoRepository
	}

	admin, err := cachedAdminByUserID(ctx, a.adminRepo, userID)
	if err != nil {
		return AdminScope{}, apperrors.WrapInternal(err, "resolve platform admin membership")
	}
	if admin == nil || !admin.IsActive {
		return AdminScope{Class: AdminScopeClassNone}, nil
	}

	scopes, err := a.scopeRepo.ListByPlatformAdmin(ctx, admin.ID, platformadmins.ScopeListOptions{OnlyActive: true})
	if err != nil {
		return AdminScope{}, apperrors.WrapInternal(err, "list platform admin scopes")
	}

	result := AdminScope{Class: AdminScopeClassNone}
	for _, scope := range scopes {
		// 有效性口径不在本层判定：列表查询已按 sql.go 的 scopeValidity
		// （未过期 && (is_bootstrap || approved_by 非空)）过滤，Gate 7 复用同一常量。
		// 此处只处理分类，禁止再引入 IsExpired/approved_by 判定，避免第二套口径。
		if scope == nil {
			continue
		}
		switch scope.ScopeType {
		case platformadmins.ScopeTypeAll:
			// all 覆盖一切，无需继续收集 targeted。
			return AdminScope{Class: AdminScopeClassAll}, nil
		case platformadmins.ScopeTypeTargeted:
			result.Class = AdminScopeClassTargeted
			for _, raw := range scope.TargetTenants {
				id, parseErr := uuid.Parse(raw)
				if parseErr != nil {
					continue
				}
				result.TargetTenants = append(result.TargetTenants, id)
			}
		}
	}
	return result, nil
}

// 确保 AllowIfScoped 实现 ScopeAuthorizer 接口。
var _ ScopeAuthorizer = (*AllowIfScopedScopeAuthorizer)(nil)

// 确保 AllowIfScoped 可被无 target 请求用于作用域分类。
var _ AdminScopeResolver = (*AllowIfScopedScopeAuthorizer)(nil)

// AuditTenantBypass 记录一次「无显式 target 的 all-scope 平台管理员」RLS 旁路。
// TargetTenant 取 source tenant 以满足审计表外键；旁路范围由 Reason 表达。
func (a *AllowIfScopedScopeAuthorizer) AuditTenantBypass(ctx context.Context, actorID uuid.UUID, actorType PrincipalType, sourceTenant uuid.UUID, reason string) {
	if a.auditWriter == nil {
		return
	}
	a.auditWriter.Append(ctx, AuditRecord{
		ActorID:      actorID,
		ActorType:    actorType,
		SourceTenant: sourceTenant,
		TargetTenant: sourceTenant,
		ResourceType: "tenant",
		Action:       "tenant.bypass",
		RiskLevel:    RiskHigh,
		Decision:     "allow",
		Reason:       reason,
	})
}

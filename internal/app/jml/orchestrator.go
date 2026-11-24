// Package jml 实现 ADR-030 JML（Joiner / Mover / Leaver）入站编排器。
//
// 设计原则：
//   - 零侵入：不修改 app/scim/service.go 或 domain 层任何现有代码。
//     Orchestrator 是独立包，通过依赖注入接口组合。
//   - 与出站同源：入站触发的 Outbox 事件用 connectors.EvtUserCreated
//     / EvtUserDeactivated / EvtRoleMemberAdded 等常量（已在
//     internal/domain/connectors/connector.go 定义）。
//   - 事务安全：依赖接口暴露 context 参数，调用方（SCIM sync 层）在同一
//     事务内完成 "用户创建 → JML 编排"。
//   - SoD 预检：Joiner 新增初始角色 + Mover 角色变更都做 SoD 检查
//     （ADR-025 双时机校验）。
package jml

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	domainconnectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	iga "github.com/guliangiam/guliangiam/internal/domain/iga"
	users "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	domainoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// connectorTarget 是 Outbox 事件 Headers 的 JSON 载荷结构。
// JML 只设置 organization_id，不指定 connector_id —— 让 Outbox Worker
// 遍历该租户下所有 enabled connector 投递（零侵入多连接器）。
type connectorTarget struct {
	ConnectorID    string `json:"connector_id"`
	OrganizationID string `json:"organization_id"`
}

// Orchestrator 实现 ADR-030 Decision 4 的 JML 入站编排。
//
// 三个入口：
//
//	Joiner: OnUserCreated      — 建号 + 初始角色绑定 + SoD 预检 +
//	                              IGA campaign 自动创建 + 出站 outbox
//	Mover:  OnRoleBindingChanged — 角色变更 + SoD 双时机复检（ADR-025）+
//	                              出站 outbox
//	Leaver: OnUserDeactivated   — 吊销会话/令牌 + 清理 role binding +
//	                              出站 deactivated
//
// 所有副作用（outbox Insert、role binding 创建/删除）交给注入接口，
// 测试时全部 Stub/mock。Orchestrator 本身只做编排顺序 + SoD 检查。
type Orchestrator struct {
	Users    users.Repository
	Roles    RoleBindingRepository
	IGA      IGARoleService
	Outbox   OutboxEnqueuer
	Sessions SessionRevoker
	Tokens   TokenRevoker
	TimeNow  func() time.Time // 可注入，默认 time.Now
}

// connectorHeaders 构造 Outbox 事件的 Headers 字节。
func connectorHeaders(orgID string) []byte {
	h := connectorTarget{OrganizationID: orgID}
	b, _ := json.Marshal(h)
	return b
}

// newOutboxItem 构造一条待 Insert 的 OutboxItem。
// status 固定 pending，max_attempts 默认 5。
func newOutboxItem(orgID, aggregateID, eventType string, payload []byte, now time.Time) *domainoutbox.OutboxItem {
	orgIDCopy := orgID
	return &domainoutbox.OutboxItem{
		ID:             uuid.NewString(),
		OrganizationID: &orgIDCopy,
		AggregateType:  domainconnectors.AggregateType,
		AggregateID:    aggregateID,
		EventType:      eventType,
		EventVersion:   1,
		Payload:        payload,
		Headers:        connectorHeaders(orgID),
		Status:         domainoutbox.StatusPending,
		MaxAttempts:    5,
		AvailableAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// ─── Joiner ──────────────────────────────────────────────────────────

// OnUserCreated 是 Joiner 入口。
//
// 流程：
//  1. 校验用户存在
//  2. 为每个 initialRole 创建 role binding
//  3. SoD 预检（candidate × 已创建的角色）—— 冲突时回滚所有刚创建的 binding
//  4. IGA campaign 自动创建（initialRoles 非空时）
//  5. 写 outbox：EvtUserCreated × 1 + EvtRoleMemberAdded × N
func (o *Orchestrator) OnUserCreated(ctx context.Context, userID, orgID string, userPayload []byte, initialRoles []string) error {
	if o.TimeNow == nil {
		o.TimeNow = time.Now
	}

	// 1. 校验用户存在
	user, err := o.Users.GetByID(ctx, userID)
	if err != nil {
		return apperrors.WrapInternalf(err, "jml joiner: fetch user %s", userID)
	}
	if user == nil {
		return apperrors.NewNotFoundf("jml joiner: user %s not found", userID)
	}

	now := o.TimeNow()

	// 2+3. 创建 binding 并 SoD 预检
	createdBindings := make([]*users.UserRoleBinding, 0, len(initialRoles))
	for _, roleID := range initialRoles {
		// SoD：candidate 新角色 vs 已创建的角色
		existing := roleIDs(createdBindings)
		rule, err := o.IGA.CheckSoD(ctx, orgID, roleID, existing)
		if err != nil {
			_ = o.rollbackBindings(ctx, createdBindings) // 尽力回滚
			return apperrors.WrapInternalf(err, "jml joiner: SoD check role %s", roleID)
		}
		if rule != nil {
			_ = o.rollbackBindings(ctx, createdBindings) // 回滚
			return newSoDConflict(roleID, rule)
		}

		binding := &users.UserRoleBinding{UserID: userID, RoleID: roleID}
		if err := o.Roles.Create(ctx, binding); err != nil {
			_ = o.rollbackBindings(ctx, createdBindings)
			return apperrors.WrapInternalf(err, "jml joiner: create binding role %s for user %s", roleID, userID)
		}
		createdBindings = append(createdBindings, binding)
	}

	// 4. IGA campaign 自动创建（initialRoles 非空）
	if len(initialRoles) > 0 {
		if err := o.IGA.CreateAutoCampaign(ctx, orgID, userID, initialRoles, "joiner"); err != nil {
			// campaign 创建失败不阻塞主流程——角色已绑定成功，campaign 可由运维补救。
			// BE-D-20：失败必须留痕，不得静默吞掉。
			slog.WarnContext(ctx, "jml joiner: auto campaign creation failed",
				"organization_id", orgID, "user_id", userID, "error", err)
		}
	}

	// 5. 写 outbox：user.created + N × role.member.added
	if err := o.Outbox.Insert(ctx, newOutboxItem(orgID, userID, domainconnectors.EvtUserCreated, userPayload, now)); err != nil {
		// outbox 写入失败不回滚 binding（ADR-008 最终一致性：binding 已落库，
		// outbox 有自身重试机制；绑定创建已经完成，回滚会造成不一致）
		return apperrors.WrapInternalf(err, "jml joiner: enqueue user.created for %s", userID)
	}
	for _, roleID := range initialRoles {
		rolePayload := mustJSON(struct {
			UserID string `json:"user_id"`
			RoleID string `json:"role_id"`
		}{UserID: userID, RoleID: roleID})
		if err := o.Outbox.Insert(ctx, newOutboxItem(orgID, roleID, domainconnectors.EvtRoleMemberAdded, rolePayload, now)); err != nil {
			return apperrors.WrapInternalf(err, "jml joiner: enqueue role.member.added user=%s role=%s", userID, roleID)
		}
	}

	return nil
}

// ─── Mover ───────────────────────────────────────────────────────────

// OnRoleBindingChanged 是 Mover 入口。
//
// 编排器语义：业务层已经把 binding 变更落库；编排器负责 SoD 双时机复检 + outbox。
// 调用方式（在同一事务内）：
//
//  1. app 层先落库 addedRoles / removedRoles 的 binding 变更
//  2. 同一事务内调用 orchestrator.OnRoleBindingChanged
//
// 如果 SoD 冲突发生在调用之前（由业务层拦截），本方法只做 outbox。
// 如果调用时检测到 SoD 冲突，返回 SoDError——调用方应立即回滚事务。
func (o *Orchestrator) OnRoleBindingChanged(ctx context.Context, userID, orgID string, addedRoles, removedRoles []string, userPayload []byte) error {
	if o.TimeNow == nil {
		o.TimeNow = time.Now
	}

	// 空操作直接返回
	if len(addedRoles) == 0 && len(removedRoles) == 0 {
		return nil
	}

	// 1. SoD 双时机复检（ADR-025）—— 新增角色 vs 用户现有角色
	currentBindings, err := o.Roles.ListByUser(ctx, userID)
	if err != nil {
		return apperrors.WrapInternalf(err, "jml mover: list bindings for user %s", userID)
	}

	// 现有角色 = currentBindings 里所有 roleID - removedRoles（去掉要删除的）
	removed := map[string]bool{}
	for _, r := range removedRoles {
		removed[r] = true
	}
	existingAfter := make([]string, 0, len(currentBindings))
	for _, b := range currentBindings {
		if !removed[b.RoleID] {
			existingAfter = append(existingAfter, b.RoleID)
		}
	}

	// 对每个 addedRole × (现有角色 + 已遍历的 addedRole) 做 SoD —— 覆盖：
	//   - 新增 vs 现有（ADR-025 双时机）
	//   - 新增 A vs 新增 B（同一批次内冲突）
	tracked := make([]string, 0, len(existingAfter)+len(addedRoles))
	tracked = append(tracked, existingAfter...)
	for _, candidate := range addedRoles {
		rule, err := o.IGA.CheckSoD(ctx, orgID, candidate, tracked)
		if err != nil {
			return apperrors.WrapInternalf(err, "jml mover: SoD check role %s for user %s", candidate, userID)
		}
		if rule != nil {
			return newSoDConflict(candidate, rule)
		}
		tracked = append(tracked, candidate)
	}

	now := o.TimeNow()

	// 2. outbox：role.added + role.removed + user.updated（有 payload 时）
	for _, roleID := range addedRoles {
		payload := mustJSON(struct {
			UserID string `json:"user_id"`
			RoleID string `json:"role_id"`
		}{UserID: userID, RoleID: roleID})
		if err := o.Outbox.Insert(ctx, newOutboxItem(orgID, roleID, domainconnectors.EvtRoleMemberAdded, payload, now)); err != nil {
			return apperrors.WrapInternalf(err, "jml mover: enqueue role.member.added user=%s role=%s", userID, roleID)
		}
	}
	for _, roleID := range removedRoles {
		payload := mustJSON(struct {
			UserID string `json:"user_id"`
			RoleID string `json:"role_id"`
		}{UserID: userID, RoleID: roleID})
		if err := o.Outbox.Insert(ctx, newOutboxItem(orgID, roleID, domainconnectors.EvtRoleMemberRemoved, payload, now)); err != nil {
			return apperrors.WrapInternalf(err, "jml mover: enqueue role.member.removed user=%s role=%s", userID, roleID)
		}
	}
	if len(userPayload) > 0 {
		if err := o.Outbox.Insert(ctx, newOutboxItem(orgID, userID, domainconnectors.EvtUserUpdated, userPayload, now)); err != nil {
			return apperrors.WrapInternalf(err, "jml mover: enqueue user.updated for %s", userID)
		}
	}

	return nil
}

// ─── Leaver ─────────────────────────────────────────────────────────

// OnUserDeactivated 是 Leaver 入口。
//
// 流程：
//  1. Sessions.RevokeAll —— 吊销所有 session（失败则停止，不继续清理）
//  2. Tokens.RevokeAll   —— 吊销所有 access_token（失败则停止，保留 binding）
//  3. 列出当前 role binding → 清理 + 写 outbox
//  4. 写 EvtUserDeactivated
//
// 半安全策略：每一步失败都停止，避免不一致。例如 token 吊销失败时保留原角色，
// 让调用方可以重试 —— 会话已吊销，用户无法登录；令牌还在但后续可以清理。
func (o *Orchestrator) OnUserDeactivated(ctx context.Context, userID, orgID string, userPayload []byte) error {
	if o.TimeNow == nil {
		o.TimeNow = time.Now
	}

	// 1. 吊销会话
	if err := o.Sessions.RevokeAll(ctx, userID); err != nil {
		return apperrors.WrapInternalf(err, "jml leaver: revoke sessions for user %s", userID)
	}

	// 2. 吊销令牌
	if err := o.Tokens.RevokeAll(ctx, userID); err != nil {
		return apperrors.WrapInternalf(err, "jml leaver: revoke tokens for user %s", userID)
	}

	// 3. 列出当前绑定，清理 + 记录 role.removed outbox
	bindings, err := o.Roles.ListByUser(ctx, userID)
	if err != nil {
		return apperrors.WrapInternalf(err, "jml leaver: list bindings for user %s", userID)
	}

	now := o.TimeNow()

	for _, b := range bindings {
		payload := mustJSON(struct {
			UserID string `json:"user_id"`
			RoleID string `json:"role_id"`
		}{UserID: userID, RoleID: b.RoleID})
		if err := o.Outbox.Insert(ctx, newOutboxItem(orgID, b.RoleID, domainconnectors.EvtRoleMemberRemoved, payload, now)); err != nil {
			return apperrors.WrapInternalf(err, "jml leaver: enqueue role.member.removed user=%s role=%s", userID, b.RoleID)
		}
	}

	// 写 EvtUserDeactivated（先写 role removed，再写 deactivated，保证出站顺序）
	if err := o.Outbox.Insert(ctx, newOutboxItem(orgID, userID, domainconnectors.EvtUserDeactivated, userPayload, now)); err != nil {
		return apperrors.WrapInternalf(err, "jml leaver: enqueue user.deactivated for %s", userID)
	}

	// 最后清理 role binding（outbox 已记录，出站处理者已收到角色变化信号）
	if err := o.Roles.DeleteByUser(ctx, userID); err != nil {
		return apperrors.WrapInternalf(err, "jml leaver: delete bindings for user %s", userID)
	}

	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────

// rollbackBindings 尽力删除本批刚创建的 binding（BE-D-20：逐条精确删除，
// 不用 DeleteByUser 整删——那会清空用户既有角色）。
// 某条 Delete 失败时继续尝试剩余的——回滚场景下 best-effort 即可。
func (o *Orchestrator) rollbackBindings(ctx context.Context, bindings []*users.UserRoleBinding) error {
	var firstErr error
	for _, b := range bindings {
		if err := o.Roles.Delete(ctx, b.UserID, b.RoleID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// roleIDs 从 binding 切片提取去重后的 role ID 列表。
func roleIDs(bindings []*users.UserRoleBinding) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(bindings))
	for _, b := range bindings {
		if !seen[b.RoleID] {
			seen[b.RoleID] = true
			out = append(out, b.RoleID)
		}
	}
	return out
}

// roleIDsFromStrs 字符串 → 去重字符串列表（原地去重）。
func roleIDsFromStrs(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// mustJSON 将 v 序列化为 JSON 字节，panic 不可恢复时返回 nil（JSON 序列化基本不会失败）。
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// newSoDConflict 构造 SoD 冲突错误（CodeConflict，409），带冲突规则 Meta。
func newSoDConflict(candidateRoleID string, rule *iga.SODRule) error {
	msg := fmt.Sprintf("SOD conflict: role %s conflicts with role %s (rule=%s)", candidateRoleID, rule.RoleBID, rule.ID)
	if rule.RoleBID == candidateRoleID {
		msg = fmt.Sprintf("SOD conflict: role %s conflicts with role %s (rule=%s)", candidateRoleID, rule.RoleAID, rule.ID)
	}
	return apperrors.NewWithMeta(apperrors.CodeConflict, msg, map[string]any{
		"rule_id":        rule.ID,
		"rule_role_a":    rule.RoleAID,
		"rule_role_b":    rule.RoleBID,
		"candidate_role": candidateRoleID,
	})
}

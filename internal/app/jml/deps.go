package jml

import (
	"context"

	iga "github.com/guliangiam/guliangiam/internal/domain/iga"
	users "github.com/guliangiam/guliangiam/internal/domain/users"
	domainoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// RoleBindingRepository 是 JML 编排器需要的用户-角色绑定仓储子集接口。
//
// 注意：internal/domain/users/repository.go 中的 users.Repository 只有
// ReplaceRoleBindings（全量替换），不满足 Joiner 逐个 Create / Leaver 按用户
// 清理 / Leaver 列出绑定的精细化需求，因此 JML 定义独立子集接口。
// 具体实现可以委托给同一个 DB 表，或由 app 层 Service 适配。
type RoleBindingRepository interface {
	// Create 新建一条用户-角色绑定。忽略重复（调用方保证幂等）。
	Create(ctx context.Context, binding *users.UserRoleBinding) error

	// DeleteByUser 删除指定用户的所有角色绑定。
	DeleteByUser(ctx context.Context, userID string) error

	// Delete 精确删除一条用户-角色绑定（BE-D-20：Joiner 回滚只删本批，
	// 不得波及用户既有绑定）。
	Delete(ctx context.Context, userID, roleID string) error

	// ListByUser 返回指定用户当前所有角色绑定。
	ListByUser(ctx context.Context, userID string) ([]*users.UserRoleBinding, error)
}

// IGARoleService 是 JML 需要的 IGA 能力子集。
// 具体实现是 app/iga.Service（完整的），测试用 Stub。
type IGARoleService interface {
	// CheckSoD 检查 candidateRoleID 是否与 existingRoleIDs 中的任一角色
	// 构成 SoD 冲突。返回冲突规则（包含 RuleAID/RuleBID）；nil 表示无冲突。
	CheckSoD(ctx context.Context, orgID, candidateRoleID string, existingRoleIDs []string) (*iga.SODRule, error)

	// CreateAutoCampaign 在 Joiner 时自动创建一个已通过 SoD 的 campaign。
	// 状态：auto-approved；到期：now + 365d；created_by="jml-system"。
	CreateAutoCampaign(ctx context.Context, orgID, userID string, initialRoleIDs []string, reason string) error
}

// OutboxEnqueuer 是 Outbox Repository 的最小子集接口。
// Outbox 全量接口在 platform/outbox；JML 只需要一个 Insert 方法。
type OutboxEnqueuer interface {
	Insert(ctx context.Context, item *domainoutbox.OutboxItem) error
}

// SessionRevoker 负责 Leaver 时吊销用户所有会话。
type SessionRevoker interface {
	RevokeAll(ctx context.Context, userID string) error
}

// TokenRevoker 负责 Leaver 时吊销用户所有令牌。
type TokenRevoker interface {
	RevokeAll(ctx context.Context, userID string) error
}

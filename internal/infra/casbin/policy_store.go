package casbin

import (
	"context"

	"github.com/casbin/casbin/v3/model"
)

// PolicyStore 是 Casbin 策略存储的方言端口（ADR-023）。
//
// Manager 只依赖本接口，不感知连接池类型：PostgreSQL 由 Adapter（pgx）实现，
// MySQL 由 internal/infra/mysql/casbin.Store 实现。前五个方法同时满足
// casbin persist.Adapter，可直接传给 casbin.NewEnforcer。
type PolicyStore interface {
	// Scoped 返回绑定到指定 enforcer 记录的副本。
	Scoped(enforcerID string) PolicyStore

	// LoadPolicy / SavePolicy / AddPolicy / RemovePolicy / RemoveFilteredPolicy
	// 实现 casbin persist.Adapter 的持久化契约。
	LoadPolicy(m model.Model) error
	SavePolicy(m model.Model) error
	AddPolicy(sec string, ptype string, rule []string) error
	RemovePolicy(sec string, ptype string, rule []string) error
	RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error

	// ReplaceUserGroupings 按 (username, orgSlug) 重写 g-规则（角色继承的唯一写入路径）。
	ReplaceUserGroupings(ctx context.Context, username, orgSlug string, roleNames []string) error

	// EnforceJIT 在请求期评估带时间窗的 JIT 策略。
	EnforceJIT(ctx context.Context, enforcerID, lifecycleID string, rvals ...interface{}) (bool, error)

	// ResolveEnforcerID 把组织 ID 解析为该组织启用的 enforcer 记录 ID。
	ResolveEnforcerID(ctx context.Context, orgID string) (string, error)

	// ResolveEnforcerUUID 是授权中间件使用的同义解析（不带租户上下文）。
	ResolveEnforcerUUID(ctx context.Context, orgID string) (string, error)
}

var _ PolicyStore = Adapter{}

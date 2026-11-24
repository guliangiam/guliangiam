package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// RLS GUC 名称——声明式 schema 里 RLS 策略统一用这两个。
// 不得在其他地方 hardcode 字符串（gen_schema.py 和本包是唯一来源）。
const (
	RLS_GUCRole          = "app.current_role"
	RLSGUCOrganizationID = "app.current_organization_id"

	// RLSRoleSuperAdmin 绕过所有租户隔离——用于后台 worker、migration、seed 等无用户上下文场景。
	RLSRoleSuperAdmin = "super_admin"
	// RLSRoleMember 普通业务角色——配合 organization_id 做租户隔离。
	RLSRoleMember = "member"
)

// ⚠️⚠️⚠️ PostgreSQL set_config 陷阱（极其容易踩坑）⚠️⚠️⚠️
// set_config(key, val, is_local) 第三个参数:
//   true  → LOCAL  设置——只在当前事务(block)内有效，事务外立即消失！
//   false → SESSION 设置——整个连接(session)生命周期有效
// 如果你在 pgxpool.AfterConnect hook（不在任何事务里）里用 set_config(key, val, true)，
// 你会看到函数返回成功，但紧接着 current_setting 读出来是空！
// 这就是 pgxpool 上设置 GUC 最常见的 bug——已踩过一次，不要再踩。

// SetRLSContext 在事务（pgx.Tx）内设置 RLS GUC——用 LOCAL(true) 覆盖，
// 这样事务内所有语句（包括 RLS 策略检查）都能读到正确值。
// 事务提交/回滚后自动恢复——不会泄漏到下一个请求。
//
//   - organization_id 为空 → current_organization_id() 返回 NULLIF(”, ”)::uuid
//   - role 为空 → 默认 "member"（fail-closed 最安全）
func SetRLSContext(ctx context.Context, tx pgx.Tx, organizationID, role string) error {
	if role == "" {
		role = RLSRoleMember
	}
	_, err := tx.Exec(ctx,
		"SELECT set_config($1, $2, true), set_config($3, $4, true)",
		RLS_GUCRole, role,
		RLSGUCOrganizationID, organizationID,
	)
	return err
}

// ClearRLSContext 把 RLS GUC 重置为空——事务内用 LOCAL(true)。
// 空值与 AfterConnect 的 fail-closed 默认一致（见 SetDefaultRLSForPoolAfterConnect）；
// pgxpool v5 归还连接时不 reset session GUC，事务局部 GUC 随事务结束自动失效。
// 当前无调用方，保留供需要在事务中途显式清空租户上下文的场景使用。
func ClearRLSContext(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx,
		"SELECT set_config($1, '', true), set_config($2, '', true)",
		RLS_GUCRole, RLSGUCOrganizationID,
	)
	return err
}

// SetDefaultRLSForPoolAfterConnect 是 pgxpool.AfterConnect hook。
// 每次 pool 新建立一个物理连接时调用——用 SESSION(false) 设成 fail-closed 默认值。
//
// 🔴 必须用 false(SESSION)，不能用 true(LOCAL)！AfterConnect 不在任何事务里，
//
//	LOCAL 设置会立即丢失——返回值看起来"成功"，实际没效果。
//
// ⚠️ Phase A-D1 变更：从 super_admin 默认改为 fail-closed。
//
//   - 旧行为：role=super_admin, org=” → is_super_admin()=true → RLS 放行所有
//
//   - 新行为：role=”, org=” → is_super_admin()=false, current_organization_id()=NULL
//     → RLS policy `is_super_admin() OR org_id = current_org()` = **UNKNOWN** → 拒绝所有
//
//   - HTTP 请求：WithinTransaction 里 set_tenant_context(member, orgID) 覆盖为 member + 正确 org
//
//   - Worker：TenantFilterSystem 注入 → setTenantContext 映射到 super_admin → RLS 全放行
//
//     前置条件：D0 Worker 迁移必须先完成（所有 Worker 入口已注入 TenantFilterSystem）
func SetDefaultRLSForPoolAfterConnect(ctx context.Context, conn *pgx.Conn) error {
	// fail-closed：空 role + 空 org → RLS 策略全部拒绝
	// HTTP 请求的 WithinTransaction 会用 set_tenant_context 覆盖为 member + 正确 org_id
	// Worker 的 TenantFilterSystem → setTenantContext 覆盖为 super_admin（全放行）
	_, err := conn.Exec(ctx,
		"SELECT set_config($1, '', false), set_config($2, '', false)",
		RLS_GUCRole,
		RLSGUCOrganizationID,
	)
	return err
}

package casbin

import (
	"context"
	"testing"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/stretchr/testify/require"
)

const threeArgModel = `[request_definition]
r = sub, org, obj, act

[policy_definition]
p = sub, org, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub, r.org)) && r.org == p.org && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`

// casbin v3.11 的 buildRoleLinks 拒绝元素数少于角色定义元数的 g 行。
// 历史 SyncRole 写入的无 domain 2 元 g 行必须被补齐后加载，否则整个
// enforcer 的 LoadPolicy 失败，该租户所有 Casbin 端点 503。
func TestLoadPolicy_LegacyTwoElementGroupingRow(t *testing.T) {
	db := &stubPolicyQuerier{rows: shared.NewMockRows([][]any{
		{"g", "admin", "32ba3157-f0ef-4936-b0e0-41488685f9aa", "", "", "", nil},
		{"g", "admin", "guliang_super_admin", "guliangtech", "", "", nil},
		{"p", "guliang_super_admin", "guliangtech", "users", "read", "", nil},
	})}

	m, err := casbinmodel.NewModelFromString(threeArgModel)
	require.NoError(t, err)
	require.NoError(t, loadPolicy(context.Background(), db, "enforcer-1", m))

	e, err := casbin.NewEnforcer(m)
	require.NoError(t, err)
	// v3.11 在此校验 g 行元数；补齐前这里会报
	// "grouping policy elements do not meet role definition"。
	require.NoError(t, e.BuildRoleLinks())

	allowed, err := e.Enforce("admin", "guliangtech", "users", "read")
	require.NoError(t, err)
	require.True(t, allowed)

	// 补的空 domain 不得让无 domain 的旧行跨租户生效。
	allowed, err = e.Enforce("admin", "other-org", "users", "read")
	require.NoError(t, err)
	require.False(t, allowed)
}

// 2 元 org 模型（GetEnforcer）加载同一张表：短行原样保留，3 元行由
// casbin 截断，BuildRoleLinks 必须仍然成功。
func TestLoadPolicy_TwoArgModelWithMixedGroupingRows(t *testing.T) {
	db := &stubPolicyQuerier{rows: shared.NewMockRows([][]any{
		{"g", "admin", "32ba3157-f0ef-4936-b0e0-41488685f9aa", "", "", "", nil},
		{"g", "admin", "guliang_super_admin", "guliangtech", "", "", nil},
	})}

	m, err := casbinmodel.NewModelFromString(`[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub)) && r.dom == p.dom && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`)
	require.NoError(t, err)
	require.NoError(t, loadPolicy(context.Background(), db, "enforcer-1", m))

	e, err := casbin.NewEnforcer(m)
	require.NoError(t, err)
	require.NoError(t, e.BuildRoleLinks())
}

// INC-BE-04 回归：g 行中间空列（v1 role 为空、v2 domain 有值）必须按列位
// 保留。nonEmpty 压缩会把 domain 顶到 role 位，生成 sub→domain 的错误角色
// 链接，使无角色的用户继承 domain 同名角色的权限。
func TestLoadPolicy_MiddleEmptyGroupingColumnPreservesPosition(t *testing.T) {
	db := &stubPolicyQuerier{rows: shared.NewMockRows([][]any{
		// alice：role 列为空、domain 列有值 —— 不得链接到 "guliangtech" 角色。
		{"g", "alice", "", "guliangtech", "", "", nil},
		// bob：正常 2 元行（domain 空）—— 仍应继承 guliangtech 角色。
		{"g", "bob", "guliangtech", "", "", "", nil},
		{"p", "guliangtech", "guliangtech", "users", "read", "", nil},
	})}

	m, err := casbinmodel.NewModelFromString(`[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub)) && r.dom == p.dom && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`)
	require.NoError(t, err)
	require.NoError(t, loadPolicy(context.Background(), db, "enforcer-1", m))

	e, err := casbin.NewEnforcer(m)
	require.NoError(t, err)
	require.NoError(t, e.BuildRoleLinks())

	allowed, err := e.Enforce("alice", "guliangtech", "users", "read")
	require.NoError(t, err)
	require.False(t, allowed, "中间空列被压缩后 alice 会错误继承 domain 同名角色")

	allowed, err = e.Enforce("bob", "guliangtech", "users", "read")
	require.NoError(t, err)
	require.True(t, allowed, "正常 2 元 g 行仍须生效")
}

// 3 元模型下中间空列同样按列位保留：空 role 链接不得跨租户授予权限，
// 且 BuildRoleLinks 不因短行/空列报错。
func TestLoadPolicy_ThreeArgMiddleEmptyGroupingColumn(t *testing.T) {
	db := &stubPolicyQuerier{rows: shared.NewMockRows([][]any{
		{"g", "alice", "", "guliangtech", "", "", nil},
		{"g", "bob", "guliang_super_admin", "guliangtech", "", "", nil},
		{"p", "guliang_super_admin", "guliangtech", "users", "read", "", nil},
	})}

	m, err := casbinmodel.NewModelFromString(threeArgModel)
	require.NoError(t, err)
	require.NoError(t, loadPolicy(context.Background(), db, "enforcer-1", m))

	e, err := casbin.NewEnforcer(m)
	require.NoError(t, err)
	require.NoError(t, e.BuildRoleLinks())

	allowed, err := e.Enforce("alice", "guliangtech", "users", "read")
	require.NoError(t, err)
	require.False(t, allowed)

	allowed, err = e.Enforce("bob", "guliangtech", "users", "read")
	require.NoError(t, err)
	require.True(t, allowed)
}

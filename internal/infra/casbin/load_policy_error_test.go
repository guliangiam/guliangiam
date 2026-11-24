package casbin

import (
	"context"
	"testing"

	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/stretchr/testify/require"
)

// 模型缺少 p 定义时 AddPolicy 会失败；LoadPolicy 必须上抛而不是静默继续，
// 否则授权会以“已加载”姿态运行在残缺模型上。
func TestLoadPolicy_AddPolicyFailurePropagates(t *testing.T) {
	db := &stubPolicyQuerier{rows: shared.NewMockRows([][]any{{
		"g2", "alice", "admin", "", "", "", nil,
	}})}

	m, err := casbinmodel.NewModelFromString(`[request_definition]
r = sub, dom, obj, act
[policy_definition]
p = sub, dom, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub`)

	require.NoError(t, err)

	err = loadPolicy(context.Background(), db, "enforcer-1", m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "add policy to casbin model")
}

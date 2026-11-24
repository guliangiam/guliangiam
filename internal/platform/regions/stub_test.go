package regions

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/regions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── DR-001 验收核心：跨 region 数据访问必须被拦截 ──

// TestDR001_CrossRegion_Blocked 两个 org 在不同 region → 拦截
func TestDR001_CrossRegion_Blocked(t *testing.T) {
	resolver := NewStubRegionResolverFromMap(map[string]string{
		"org-cn": "cn-north-1",
		"org-us": "us-east-1",
	})
	guard := regions.NewCrossRegionGuard(resolver)

	err := guard.Validate(context.Background(), "org-cn", "org-us", "role.binding.create")
	require.Error(t, err)

	ce, ok := regions.IsCrossRegion(err)
	require.True(t, ok, "should be CrossRegionError")
	assert.Equal(t, "org-cn", ce.OrgA)
	assert.Equal(t, "cn-north-1", ce.RegionCodeA)
	assert.Equal(t, "org-us", ce.OrgB)
	assert.Equal(t, "us-east-1", ce.RegionCodeB)
	assert.Equal(t, "role.binding.create", ce.Operation)
}

// TestDR001_SameRegion_Allowed 同 region org → 放行
func TestDR001_SameRegion_Allowed(t *testing.T) {
	resolver := NewStubRegionResolverFromMap(map[string]string{
		"org-cn-1": "cn-north-1",
		"org-cn-2": "cn-north-1",
	})
	guard := regions.NewCrossRegionGuard(resolver)

	err := guard.Validate(context.Background(), "org-cn-1", "org-cn-2", "user.list")
	assert.NoError(t, err)
}

// TestDR001_SameOrg_Allowed 同一 org 内部操作 → 不受限制
func TestDR001_SameOrg_Allowed(t *testing.T) {
	resolver := NewStubRegionResolverFromMap(map[string]string{
		"org-cn": "cn-north-1",
	})
	guard := regions.NewCrossRegionGuard(resolver)

	err := guard.Validate(context.Background(), "org-cn", "org-cn", "user.create")
	assert.NoError(t, err)
}

// TestDR001_UnboundOrg_NotFound 任一 org 无绑定 → NotFound
func TestDR001_UnboundOrg_NotFound(t *testing.T) {
	resolver := NewStubRegionResolverFromMap(map[string]string{
		"org-cn": "cn-north-1",
	})
	guard := regions.NewCrossRegionGuard(resolver)

	err := guard.Validate(context.Background(), "org-cn", "org-unknown", "user.list")
	require.Error(t, err)
	// 是 NotFound 不是 CrossRegion
	assert.NotContains(t, err.Error(), "region residency violation")
}

// TestDR001_EmptyOrgID_InvalidArgument 空 orgID → InvalidArgument
func TestDR001_EmptyOrgID_InvalidArgument(t *testing.T) {
	resolver := NewStubRegionResolver()
	guard := regions.NewCrossRegionGuard(resolver)

	err := guard.Validate(context.Background(), "", "org-cn", "user.list")
	require.Error(t, err)
}

// ── ValidateMany: 批量跨 region 检测 ──

func TestValidateMany_CrossRegion_Blocked(t *testing.T) {
	resolver := NewStubRegionResolverFromMap(map[string]string{
		"org-a": "cn-north-1",
		"org-b": "us-east-1",
		"org-c": "cn-north-1",
	})
	guard := regions.NewCrossRegionGuard(resolver)

	// [org-a, org-b, org-c] — org-a 和 org-b 跨 region
	err := guard.ValidateMany(context.Background(), []string{"org-a", "org-b", "org-c"}, "outbox.fanout")
	require.Error(t, err)
	ce, ok := regions.IsCrossRegion(err)
	require.True(t, ok)
	assert.Equal(t, "org-a", ce.OrgA)
	assert.Equal(t, "org-b", ce.OrgB)
}

func TestValidateMany_SameRegion_Allowed(t *testing.T) {
	resolver := NewStubRegionResolverFromMap(map[string]string{
		"org-a": "cn-north-1",
		"org-b": "cn-north-1",
		"org-c": "cn-north-1",
	})
	guard := regions.NewCrossRegionGuard(resolver)

	err := guard.ValidateMany(context.Background(), []string{"org-a", "org-b", "org-c"}, "outbox.fanout")
	assert.NoError(t, err)
}

func TestValidateMany_SingleOrg_Allowed(t *testing.T) {
	resolver := NewStubRegionResolverFromMap(map[string]string{
		"org-a": "cn-north-1",
	})
	guard := regions.NewCrossRegionGuard(resolver)

	// 单个 org 或空列表 — 直接通过
	assert.NoError(t, guard.ValidateMany(context.Background(), []string{"org-a"}, "op"))
	assert.NoError(t, guard.ValidateMany(context.Background(), []string{}, "op"))
}

// ── StubRegionResolver Resolve/ResolveBatch ──

func TestStubResolver_Resolve(t *testing.T) {
	s := NewStubRegionResolverFromMap(map[string]string{"org-1": "cn-north-1"})
	code, err := s.Resolve(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, "cn-north-1", code)

	_, err = s.Resolve(context.Background(), "org-x")
	require.Error(t, err)
}

func TestStubResolver_ResolveBatch(t *testing.T) {
	s := NewStubRegionResolverFromMap(map[string]string{
		"a": "r1", "b": "r2", "c": "r1",
	})
	result, err := s.ResolveBatch(context.Background(), []string{"a", "b", "x", "c"})
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "r1", result["a"])
	assert.Equal(t, "r2", result["b"])
	assert.Equal(t, "r1", result["c"])
	// "x" 不存在 → 不出现在 map
	_, ok := result["x"]
	assert.False(t, ok)
}

// ── DeploymentRegion 域模型验证 ──

func TestDeploymentRegion_Validate(t *testing.T) {
	valid := &regions.DeploymentRegion{
		RegionCode:  "cn-north-1",
		CountryCode: "CN",
	}
	require.NoError(t, valid.Validate())

	// 缺 region_code
	invalid := &regions.DeploymentRegion{CountryCode: "CN"}
	assert.Error(t, invalid.Validate())

	// country_code 不是 2 字符 ISO
	invalid2 := &regions.DeploymentRegion{RegionCode: "x", CountryCode: "CNN"}
	assert.Error(t, invalid2.Validate())
}

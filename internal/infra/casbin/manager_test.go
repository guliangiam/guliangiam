package casbin

import (
	"context"
	"testing"

	casbinv3 "github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(Adapter{})
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.enforcers)
	assert.NotNil(t, mgr.orgEnforcers)
}

func TestManager_SyncUserRoles_IgnoresIncompleteScope(t *testing.T) {
	mgr := NewManager(Adapter{})
	assert.NoError(t, mgr.SyncUserRoles(context.Background(), "", "built-in", "alice", nil))
	assert.NoError(t, mgr.SyncUserRoles(context.Background(), "org-1", "", "alice", nil))
	assert.NoError(t, mgr.SyncUserRoles(context.Background(), "org-1", "built-in", "", nil))

	var nilManager *Manager
	assert.NoError(t, nilManager.SyncUserRoles(context.Background(), "org-1", "built-in", "alice", nil))
}

func TestNewManager_EmptyEnforcers(t *testing.T) {
	mgr := NewManager(Adapter{})
	assert.Empty(t, mgr.enforcers)
	assert.Empty(t, mgr.orgEnforcers)
}

func TestManager_Invalidate(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.enforcers[enforcerCacheKey("enf-1", "")] = nil
	mgr.enforcers[enforcerCacheKey("enf-2", "")] = nil
	assert.Len(t, mgr.enforcers, 2)

	mgr.Invalidate("enf-1")
	assert.Len(t, mgr.enforcers, 1)
	_, ok := mgr.enforcers[enforcerCacheKey("enf-1", "")]
	assert.False(t, ok)
	_, ok = mgr.enforcers[enforcerCacheKey("enf-2", "")]
	assert.True(t, ok)
}

func TestManager_Invalidate_RemovesEveryModelVariant(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.enforcers[enforcerCacheKey("enf-1", "model-a")] = nil
	mgr.enforcers[enforcerCacheKey("enf-1", "model-b")] = nil
	mgr.enforcers[enforcerCacheKey("enf-2", "model-a")] = nil

	mgr.Invalidate("enf-1")
	assert.Len(t, mgr.enforcers, 1)
	_, ok := mgr.enforcers[enforcerCacheKey("enf-2", "model-a")]
	assert.True(t, ok)
}

func TestManager_EnforcerCacheKey_DistinguishesModels(t *testing.T) {
	assert.NotEqual(
		t,
		enforcerCacheKey("enf-1", "model-a"),
		enforcerCacheKey("enf-1", "model-b"),
		"one enforcer serves several principal models; a shared key would reuse the first caller's matcher",
	)
}

func TestManager_InvalidateOrg(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.orgEnforcers["org-1"] = nil
	mgr.orgEnforcers["org-2"] = nil
	assert.Len(t, mgr.orgEnforcers, 2)

	mgr.InvalidateOrg("org-1")
	assert.Len(t, mgr.orgEnforcers, 1)
	_, ok := mgr.orgEnforcers["org-1"]
	assert.False(t, ok)
	_, ok = mgr.orgEnforcers["org-2"]
	assert.True(t, ok)
}

func TestManager_InvalidateAll(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.enforcers[enforcerCacheKey("enf-1", "")] = nil
	mgr.enforcers[enforcerCacheKey("enf-2", "")] = nil
	mgr.enforcers[enforcerCacheKey("enf-3", "")] = nil
	assert.Len(t, mgr.enforcers, 3)

	mgr.InvalidateAll()
	assert.Empty(t, mgr.enforcers)
}

func TestManager_Invalidate_NonExistent(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.Invalidate("nonexistent")
	assert.Empty(t, mgr.enforcers)
}

func TestManager_InvalidateOrg_NonExistent(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.InvalidateOrg("nonexistent")
	assert.Empty(t, mgr.orgEnforcers)
}

const managerTestModel = `[request_definition]
r = sub, dom, obj, act
[policy_definition]
p = sub, dom, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && r.dom == p.dom && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)`

func newManagerWithTestEnforcer(t *testing.T) *Manager {
	t.Helper()
	m, err := casbinmodel.NewModelFromString(managerTestModel)
	require.NoError(t, err)
	e, err := casbinv3.NewEnforcer(m)
	require.NoError(t, err)
	mgr := NewManager(Adapter{})
	mgr.enforcers[enforcerCacheKey("enforcer-1", managerTestModel)] = e
	return mgr
}

func TestManager_EnforceNeverFallsBackToJIT(t *testing.T) {
	mgr := newManagerWithTestEnforcer(t)
	jitCalls := 0
	mgr.jitEnforce = func(context.Context, string, string, ...interface{}) (bool, error) {
		jitCalls++
		return true, nil
	}

	allowed, err := mgr.Enforce(context.Background(), "enforcer-1", managerTestModel, "alice", "acme", "orders", "get")
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Zero(t, jitCalls)

	results, err := mgr.BatchEnforce(context.Background(), "enforcer-1", managerTestModel, [][]interface{}{{"alice", "acme", "orders", "get"}})
	require.NoError(t, err)
	assert.Equal(t, []bool{false}, results)
	assert.Zero(t, jitCalls)
}

func TestManager_EnforcePrivilegedBindsJITToSessionLifecycle(t *testing.T) {
	mgr := newManagerWithTestEnforcer(t)
	var gotLifecycle string
	var gotRvals []interface{}
	mgr.jitEnforce = func(_ context.Context, _ string, lifecycleID string, rvals ...interface{}) (bool, error) {
		gotLifecycle = lifecycleID
		gotRvals = rvals
		return lifecycleID == "lifecycle-a", nil
	}

	allowed, err := mgr.EnforcePrivileged(context.Background(), "enforcer-1", managerTestModel, "lifecycle-a", "alice", "acme", "orders", "get")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, "lifecycle-a", gotLifecycle)
	assert.Equal(t, []interface{}{"alice", "acme", "orders", "get"}, gotRvals, "lifecycle ID must not be added to the Casbin request")

	allowed, err = mgr.EnforcePrivileged(context.Background(), "enforcer-1", managerTestModel, "lifecycle-b", "alice", "acme", "orders", "get")
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, "lifecycle-b", gotLifecycle)
}

func TestManager_EnforcePrivilegedRejectsBlankLifecycleAndKeepsPermanentPolicies(t *testing.T) {
	mgr := newManagerWithTestEnforcer(t)
	jitCalls := 0
	mgr.jitEnforce = func(context.Context, string, string, ...interface{}) (bool, error) {
		jitCalls++
		return true, nil
	}
	for _, lifecycleID := range []string{"", "  "} {
		allowed, err := mgr.EnforcePrivileged(context.Background(), "enforcer-1", managerTestModel, lifecycleID, "alice", "acme", "orders", "get")
		require.NoError(t, err)
		assert.False(t, allowed)
	}
	assert.Zero(t, jitCalls)

	_, err := mgr.enforcers[enforcerCacheKey("enforcer-1", managerTestModel)].AddPolicy("alice", "acme", "orders", "get")
	require.NoError(t, err)
	allowed, err := mgr.EnforcePrivileged(context.Background(), "enforcer-1", managerTestModel, "", "alice", "acme", "orders", "get")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Zero(t, jitCalls, "permanent allow must not require a JIT lookup")
}

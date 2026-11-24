package casbin

import (
	"context"
	"testing"

	domainroles "github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/stretchr/testify/assert"
)

func TestManager_SetCache(t *testing.T) {
	mgr := NewManager(Adapter{})
	cache := NewEnforceCache(100)
	mgr.SetCache(cache)
	assert.Equal(t, cache, mgr.EnforceCache())
}

func TestManager_EnforceCache(t *testing.T) {
	mgr := NewManager(Adapter{})
	cache := mgr.EnforceCache()
	assert.NotNil(t, cache)
}

func TestManager_SyncRole_NilManager(t *testing.T) {
	var mgr *Manager
	err := mgr.SyncRole(context.Background(), "org-1", nil)
	assert.NoError(t, err)
}

func TestManager_SyncRole_NilRole(t *testing.T) {
	mgr := NewManager(Adapter{})
	err := mgr.SyncRole(context.Background(), "org-1", nil)
	assert.NoError(t, err)
}

func TestManager_SyncRole_DisabledRole(t *testing.T) {
	mgr := NewManager(Adapter{})
	role := &domainroles.Role{IsEnabled: false}
	err := mgr.SyncRole(context.Background(), "org-1", role)
	assert.NoError(t, err)
}

func TestManager_RemoveRole_NilManager(t *testing.T) {
	var mgr *Manager
	err := mgr.RemoveRole(context.Background(), "org-1", "role-1")
	assert.NoError(t, err)
}

func TestManager_InvalidateOrg_Existing(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.orgEnforcers["org-1"] = nil
	mgr.orgEnforcers["org-2"] = nil

	mgr.InvalidateOrg("org-1")
	_, ok := mgr.orgEnforcers["org-1"]
	assert.False(t, ok)
	_, ok = mgr.orgEnforcers["org-2"]
	assert.True(t, ok)
}

func TestManager_Invalidate_NonexistentKey(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.Invalidate("does-not-exist")
	assert.Empty(t, mgr.enforcers)
}

func TestManager_InvalidateOrg_NonexistentKey(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.InvalidateOrg("does-not-exist")
	assert.Empty(t, mgr.orgEnforcers)
}

func TestManager_InvalidateAll_ClearsAll(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.enforcers[enforcerCacheKey("a", "")] = nil
	mgr.enforcers[enforcerCacheKey("b", "")] = nil
	mgr.enforcers[enforcerCacheKey("c", "")] = nil

	mgr.InvalidateAll()
	assert.Empty(t, mgr.enforcers)
}

func TestManager_GetOrCreateEnforcer_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	_, err := mgr.GetOrCreateEnforcer("enf-1", "invalid model text")
	assert.Error(t, err)
}

func TestManager_Enforce_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	_, err := mgr.Enforce(context.Background(), "enf-1", "invalid model", "sub", "obj", "act")
	assert.Error(t, err)
}

func TestManager_BatchEnforce_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	_, err := mgr.BatchEnforce(context.Background(), "enf-1", "invalid model", [][]interface{}{{"sub", "obj", "act"}})
	assert.Error(t, err)
}

func TestManager_AddPolicy_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	err := mgr.AddPolicy("enf-1", "invalid model", []string{"sub", "obj", "act"})
	assert.Error(t, err)
}

func TestManager_RemovePolicy_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	err := mgr.RemovePolicy("enf-1", "invalid model", []string{"sub", "obj", "act"})
	assert.Error(t, err)
}

func TestManager_GetPolicies_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	_, err := mgr.GetPolicies("enf-1", "invalid model")
	assert.Error(t, err)
}

func TestManager_GetFilteredPolicies_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	_, err := mgr.GetFilteredPolicies("enf-1", "invalid model", 0, "sub")
	assert.Error(t, err)
}

func TestManager_UpdatePolicy_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	err := mgr.UpdatePolicy("enf-1", "invalid model", []string{"old"}, []string{"new"})
	assert.Error(t, err)
}

func TestManager_ReloadPolicy_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	err := mgr.ReloadPolicy("enf-1", "invalid model")
	assert.Error(t, err)
}

func TestManager_GetAllSubjects_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	_, err := mgr.GetAllSubjects("enf-1", "invalid model")
	assert.Error(t, err)
}

func TestManager_GetAllObjects_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	_, err := mgr.GetAllObjects("enf-1", "invalid model")
	assert.Error(t, err)
}

func TestManager_GetAllActions_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	_, err := mgr.GetAllActions("enf-1", "invalid model")
	assert.Error(t, err)
}

func TestManager_GetAllRoles_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	_, err := mgr.GetAllRoles("enf-1", "invalid model")
	assert.Error(t, err)
}

func TestManager_AddRoleForUser_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	err := mgr.AddRoleForUser("enf-1", "invalid model", "user-1", "role-1")
	assert.Error(t, err)
}

func TestManager_RemoveFilteredPolicy_InvalidModel(t *testing.T) {
	mgr := NewManager(Adapter{})
	err := mgr.RemoveFilteredPolicy("enf-1", "invalid model", 0, "sub")
	assert.Error(t, err)
}

// Note: GetEnforcer and related org-scoped methods require a real database
// connection (pgxpool.Pool) and cannot be tested without one. They are
// covered by integration tests.

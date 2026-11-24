package casbin

import (
	"context"
	"testing"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	domainroles "github.com/guliangiam/guliangiam/internal/domain/roles"
)

type recordingPolicyStore struct {
	added [][]string
}

func (s *recordingPolicyStore) Scoped(string) PolicyStore { return s }
func (s *recordingPolicyStore) LoadPolicy(model.Model) error {
	return nil
}
func (s *recordingPolicyStore) SavePolicy(model.Model) error { return nil }
func (s *recordingPolicyStore) AddPolicy(_ string, _ string, rule []string) error {
	s.added = append(s.added, append([]string(nil), rule...))
	return nil
}
func (s *recordingPolicyStore) RemovePolicy(string, string, []string) error { return nil }
func (s *recordingPolicyStore) RemoveFilteredPolicy(string, string, int, ...string) error {
	return nil
}
func (s *recordingPolicyStore) ReplaceUserGroupings(context.Context, string, string, []string) error {
	return nil
}
func (s *recordingPolicyStore) EnforceJIT(context.Context, string, string, ...interface{}) (bool, error) {
	return false, nil
}
func (s *recordingPolicyStore) ResolveEnforcerID(context.Context, string) (string, error) {
	return "enf-1", nil
}
func (s *recordingPolicyStore) ResolveEnforcerUUID(context.Context, string) (string, error) {
	return "enf-1", nil
}

const syncRoleTestModel = `[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub)) && r.dom == p.dom && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`

func newSyncRoleTestManager(t *testing.T, store *recordingPolicyStore) (*Manager, *casbin.Enforcer) {
	t.Helper()
	m, err := model.NewModelFromString(syncRoleTestModel)
	if err != nil {
		t.Fatalf("parse model: %v", err)
	}
	e, err := casbin.NewEnforcer(m, store)
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	mgr := NewManager(store)
	mgr.orgEnforcers["org-1"] = e
	mgr.enforcers[enforcerCacheKey("enf-1", syncRoleTestModel)] = e
	mgr.cache.Set("sub-1", "obj-1", "read", true)
	return mgr, e
}

func TestSyncRole_SkipsEmptySubjectsAndInvalidatesCaches(t *testing.T) {
	store := &recordingPolicyStore{}
	mgr, e := newSyncRoleTestManager(t, store)

	role := &domainroles.Role{
		ID:        "role-1",
		IsEnabled: true,
		Users:     []string{"user-1", "", "   ", "user-2"},
		RoleIDs:   []string{"parent-1", "", "  "},
	}
	if err := mgr.SyncRole(context.Background(), "org-1", role); err != nil {
		t.Fatalf("SyncRole: %v", err)
	}

	want := [][]string{
		{"user-1", "role-1"},
		{"user-2", "role-1"},
		{"role-1", "parent-1"},
	}
	if len(store.added) != len(want) {
		t.Fatalf("persisted g rows = %v, want %v", store.added, want)
	}
	for i, row := range want {
		if store.added[i][0] != row[0] || store.added[i][1] != row[1] {
			t.Fatalf("persisted g row %d = %v, want %v", i, store.added[i], row)
		}
	}

	grouping, err := e.GetGroupingPolicy()
	if err != nil {
		t.Fatalf("GetGroupingPolicy: %v", err)
	}
	for _, rule := range grouping {
		for _, v := range rule {
			if v == "" {
				t.Fatalf("in-memory grouping contains empty value: %v", rule)
			}
		}
	}

	if _, ok := mgr.orgEnforcers["org-1"]; ok {
		t.Fatal("org enforcer cache not invalidated after SyncRole")
	}
	if _, ok := mgr.enforcers[enforcerCacheKey("enf-1", syncRoleTestModel)]; ok {
		t.Fatal("request-time enforcer cache not invalidated after SyncRole")
	}
	if _, found := mgr.cache.Get("sub-1", "obj-1", "read"); found {
		t.Fatal("enforce cache not invalidated after SyncRole")
	}
}

func TestSyncRole_DisabledRoleIsNoop(t *testing.T) {
	store := &recordingPolicyStore{}
	mgr, _ := newSyncRoleTestManager(t, store)

	role := &domainroles.Role{ID: "role-1", IsEnabled: false, Users: []string{"user-1"}}
	if err := mgr.SyncRole(context.Background(), "org-1", role); err != nil {
		t.Fatalf("SyncRole: %v", err)
	}
	if len(store.added) != 0 {
		t.Fatalf("disabled role persisted g rows: %v", store.added)
	}
	if _, ok := mgr.orgEnforcers["org-1"]; !ok {
		t.Fatal("noop SyncRole must not invalidate caches")
	}
}

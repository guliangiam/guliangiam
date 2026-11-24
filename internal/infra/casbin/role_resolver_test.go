package casbin

import (
	"context"
	"testing"

	domainroles "github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/guliangiam/guliangiam/internal/domain/roles/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewRoleResolver(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)
	assert.NotNil(t, resolver)
	assert.NotNil(t, resolver.cache)
}

func TestRoleResolver_ResolveRoles_Cached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	// Pre-populate cache
	resolver.cache["org-1"] = []string{"admin, editor"}

	groupings, err := resolver.ResolveRoles(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"admin, editor"}, groupings)

	// Repository should NOT be called since cache hit
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Times(0)
}

func TestRoleResolver_ResolveRoles_NoInheritance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainroles.Role{
		{ID: "role-1", Name: "admin", RoleIDs: nil},
		{ID: "role-2", Name: "editor", RoleIDs: nil},
	}, int64(2), nil)

	groupings, err := resolver.ResolveRoles(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Empty(t, groupings) // No inheritance, no groupings
}

func TestRoleResolver_ResolveRoles_WithInheritance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainroles.Role{
		{ID: "role-1", Name: "admin", RoleIDs: []string{"role-2"}},
		{ID: "role-2", Name: "editor", RoleIDs: nil},
	}, int64(2), nil)

	groupings, err := resolver.ResolveRoles(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, groupings, 1)
	assert.Equal(t, "admin, editor", groupings[0])
}

func TestRoleResolver_ResolveRoles_MultipleInheritance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainroles.Role{
		{ID: "role-1", Name: "admin", RoleIDs: []string{"role-2", "role-3"}},
		{ID: "role-2", Name: "editor", RoleIDs: nil},
		{ID: "role-3", Name: "viewer", RoleIDs: nil},
	}, int64(3), nil)

	groupings, err := resolver.ResolveRoles(context.Background(), "org-1")
	require.NoError(t, err)
	require.Len(t, groupings, 2)
	assert.Contains(t, groupings, "admin, editor")
	assert.Contains(t, groupings, "admin, viewer")
}

func TestRoleResolver_ResolveRoles_NestedInheritance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	// admin inherits editor, editor inherits viewer
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainroles.Role{
		{ID: "role-1", Name: "admin", RoleIDs: []string{"role-2"}},
		{ID: "role-2", Name: "editor", RoleIDs: []string{"role-3"}},
		{ID: "role-3", Name: "viewer", RoleIDs: nil},
	}, int64(3), nil)

	groupings, err := resolver.ResolveRoles(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Contains(t, groupings, "admin, editor")
	assert.Contains(t, groupings, "editor, viewer")
}

func TestRoleResolver_ResolveRoles_CycleDetection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	// admin inherits editor, editor inherits admin (cycle)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainroles.Role{
		{ID: "role-1", Name: "admin", RoleIDs: []string{"role-2"}},
		{ID: "role-2", Name: "editor", RoleIDs: []string{"role-1"}},
	}, int64(2), nil)

	groupings, err := resolver.ResolveRoles(context.Background(), "org-1")
	require.NoError(t, err)
	// Should not infinite loop; cycle is detected
	assert.NotEmpty(t, groupings)
}

func TestRoleResolver_ResolveRoles_UnknownParentRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	// admin references a non-existent parent role
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainroles.Role{
		{ID: "role-1", Name: "admin", RoleIDs: []string{"nonexistent-role"}},
	}, int64(1), nil)

	groupings, err := resolver.ResolveRoles(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Empty(t, groupings) // Unknown parent is skipped
}

func TestRoleResolver_ResolveRoles_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	_, err := resolver.ResolveRoles(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRoleResolver_ResolveRoles_CacheResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domainroles.Role{
		{ID: "role-1", Name: "admin", RoleIDs: []string{"role-2"}},
		{ID: "role-2", Name: "editor", RoleIDs: nil},
	}, int64(2), nil)

	groupings1, err := resolver.ResolveRoles(context.Background(), "org-1")
	require.NoError(t, err)

	// Second call should use cache, repo not called again
	groupings2, err := resolver.ResolveRoles(context.Background(), "org-1")
	require.NoError(t, err)

	assert.Equal(t, groupings1, groupings2)
}

func TestRoleResolver_Invalidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	resolver.cache["org-1"] = []string{"admin, editor"}
	resolver.cache["org-2"] = []string{"viewer, reader"}

	resolver.Invalidate("org-1")
	_, ok := resolver.cache["org-1"]
	assert.False(t, ok)
	_, ok = resolver.cache["org-2"]
	assert.True(t, ok)
}

func TestRoleResolver_InvalidateAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock.NewMockRepository(ctrl)
	resolver := NewRoleResolver(repo)

	resolver.cache["org-1"] = []string{"admin, editor"}
	resolver.cache["org-2"] = []string{"viewer, reader"}

	resolver.InvalidateAll()
	assert.Empty(t, resolver.cache)
}

func TestRoleResolver_resolveNestedRoles_MaxDepth(t *testing.T) {
	resolver := NewRoleResolver(nil)
	roleByID := map[string]*domainroles.Role{
		"r1": {ID: "r1", Name: "deep", RoleIDs: []string{"r2"}},
		"r2": {ID: "r2", Name: "deeper", RoleIDs: []string{"r3"}},
	}

	// depth > 10 should return nil
	result := resolver.resolveNestedRoles(roleByID, "deep", map[string]bool{}, 11)
	assert.Nil(t, result)
}

func TestRoleResolver_resolveNestedRoles_CycleDetection(t *testing.T) {
	resolver := NewRoleResolver(nil)
	roleByID := map[string]*domainroles.Role{}

	// If roleName is already visited, return nil
	visited := map[string]bool{"deep": true}
	result := resolver.resolveNestedRoles(roleByID, "deep", visited, 1)
	assert.Nil(t, result)
}

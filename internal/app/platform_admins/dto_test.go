package platform_admins

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	platformerrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestNewPlatformAdminDetail_MapsAllFields(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	pa := &platformadmins.PlatformAdmin{
		ID: "pa-1", UserID: "user-1", CreatedBy: "pa-0",
		IsActive: true, CreatedAt: now, UpdatedAt: now.Add(time.Hour), Revision: 3,
	}
	detail := NewPlatformAdminDetail(pa)
	require.NotNil(t, detail)
	assert.Equal(t, "pa-1", detail.ID)
	assert.Equal(t, "user-1", detail.UserID)
	assert.Equal(t, "pa-0", detail.CreatedBy)
	assert.True(t, detail.IsActive)
	assert.Equal(t, now, detail.CreatedAt)
	assert.Equal(t, now.Add(time.Hour), detail.UpdatedAt)
	assert.Equal(t, int64(3), detail.Revision)

	// json tag 必须是 camelCase 全字段。
	raw, err := json.Marshal(detail)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	for _, key := range []string{"id", "userId", "createdBy", "isActive", "createdAt", "updatedAt", "revision"} {
		assert.Contains(t, decoded, key)
	}
}

func TestNewPlatformAdminDetail_NilSafe(t *testing.T) {
	assert.Nil(t, NewPlatformAdminDetail(nil))
	assert.Empty(t, NewPlatformAdminDetails(nil))
	assert.NotNil(t, NewPlatformAdminDetails(nil)) // [] 而不是 null
}

func TestNewScopeDetail_MapsAllFieldsAndNormalizesTargets(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	approver := "pa-2"
	scope := &platformadmins.PlatformAdminScope{
		ID: "scope-1", PlatformAdminID: "pa-1", ScopeType: platformadmins.ScopeTypeTargeted,
		TargetTenants: []string{"tenant-1"}, ApprovedBy: &approver, ApprovedAt: &now,
		IsBootstrap: false, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now,
	}
	detail := NewScopeDetail(scope)
	require.NotNil(t, detail)
	assert.Equal(t, "scope-1", detail.ID)
	assert.Equal(t, "pa-1", detail.PlatformAdminID)
	assert.Equal(t, platformadmins.ScopeTypeTargeted, detail.ScopeType)
	assert.Equal(t, []string{"tenant-1"}, detail.TargetTenants)
	require.NotNil(t, detail.ApprovedBy)
	assert.Equal(t, "pa-2", *detail.ApprovedBy)
	require.NotNil(t, detail.ApprovedAt)
	assert.Equal(t, now, *detail.ApprovedAt)
	assert.False(t, detail.IsBootstrap)
	assert.Equal(t, now.Add(24*time.Hour), detail.ExpiresAt)
	assert.Equal(t, now, detail.CreatedAt)

	// 'all' scope 的 nil target_tenants 必须序列化为 []。
	all := NewScopeDetail(&platformadmins.PlatformAdminScope{ID: "scope-2", ScopeType: platformadmins.ScopeTypeAll})
	require.NotNil(t, all)
	assert.NotNil(t, all.TargetTenants)
	raw, err := json.Marshal(all)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"targetTenants":[]`)
	assert.Nil(t, NewScopeDetail(nil))
	assert.NotNil(t, NewScopeDetails(nil))
}

// ─── DTOAdapter：投影 + 错误透传 ────────────────────────────────────────────

type dtoAdminRepoStub struct {
	admin *platformadmins.PlatformAdmin
	items []*platformadmins.PlatformAdmin
	total int64
	err   error
}

func (r dtoAdminRepoStub) GetByID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return r.admin, r.err
}
func (r dtoAdminRepoStub) GetByUserID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return r.admin, r.err
}
func (r dtoAdminRepoStub) List(context.Context, platformadmins.ListOptions) ([]*platformadmins.PlatformAdmin, int64, error) {
	return r.items, r.total, r.err
}
func (r dtoAdminRepoStub) Create(context.Context, *platformadmins.PlatformAdmin) error { return r.err }
func (r dtoAdminRepoStub) Update(context.Context, *platformadmins.PlatformAdmin) error { return r.err }
func (r dtoAdminRepoStub) SetActive(context.Context, string, bool) error               { return r.err }
func (r dtoAdminRepoStub) Delete(context.Context, string) error                        { return r.err }

type dtoScopeRepoStub struct {
	scope *platformadmins.PlatformAdminScope
	items []*platformadmins.PlatformAdminScope
	err   error
}

func (r dtoScopeRepoStub) GetByID(context.Context, string) (*platformadmins.PlatformAdminScope, error) {
	return r.scope, r.err
}
func (r dtoScopeRepoStub) ListByPlatformAdmin(context.Context, string, platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	return r.items, r.err
}
func (r dtoScopeRepoStub) Create(context.Context, *platformadmins.PlatformAdminScope) error {
	return r.err
}
func (r dtoScopeRepoStub) Approve(context.Context, string, string) error { return r.err }
func (r dtoScopeRepoStub) Delete(context.Context, string) error          { return r.err }
func (r dtoScopeRepoStub) HasActiveScopeForTenant(context.Context, string, string) (bool, error) {
	return false, r.err
}

func TestDTOAdapter_ReadPathsProjectEntities(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUow := appuowmock.NewMockUnitOfWork(ctrl)
	now := time.Now().UTC()
	admin := &platformadmins.PlatformAdmin{ID: "pa-1", UserID: "user-1", IsActive: true, CreatedAt: now, UpdatedAt: now}
	scope := &platformadmins.PlatformAdminScope{ID: "scope-1", PlatformAdminID: "pa-1", ScopeType: platformadmins.ScopeTypeAll, ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	mockUow.EXPECT().PlatformAdmins().Return(dtoAdminRepoStub{admin: admin, items: []*platformadmins.PlatformAdmin{admin}, total: 1}).AnyTimes()
	mockUow.EXPECT().PlatformAdminScopes().Return(dtoScopeRepoStub{scope: scope, items: []*platformadmins.PlatformAdminScope{scope}}).AnyTimes()

	adapter := NewDTOAdapter(NewService(nil, nil, mockUow, nil))
	ctx := context.Background()

	got, err := adapter.GetPlatformAdmin(ctx, "pa-1")
	require.NoError(t, err)
	assert.Equal(t, "pa-1", got.ID)

	byUser, err := adapter.GetPlatformAdminByUserID(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, "user-1", byUser.UserID)

	items, total, err := adapter.ListPlatformAdmins(ctx, ListPlatformAdminsQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "pa-1", items[0].ID)

	gotScope, err := adapter.GetScope(ctx, "scope-1")
	require.NoError(t, err)
	assert.Equal(t, "scope-1", gotScope.ID)

	scopes, err := adapter.ListScopes(ctx, ListScopesQuery{PlatformAdminID: "pa-1"})
	require.NoError(t, err)
	require.Len(t, scopes, 1)
	assert.Equal(t, "scope-1", scopes[0].ID)
}

func TestDTOAdapter_PropagatesErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockUow := appuowmock.NewMockUnitOfWork(ctrl)
	notFound := platformerrors.NewNotFound("platform admin not found")
	mockUow.EXPECT().PlatformAdmins().Return(dtoAdminRepoStub{err: notFound}).AnyTimes()
	mockUow.EXPECT().PlatformAdminScopes().Return(dtoScopeRepoStub{err: notFound}).AnyTimes()

	adapter := NewDTOAdapter(NewService(nil, nil, mockUow, nil))
	ctx := context.Background()

	_, err := adapter.GetPlatformAdmin(ctx, "pa-1")
	assert.ErrorIs(t, err, notFound)
	_, _, err = adapter.ListPlatformAdmins(ctx, ListPlatformAdminsQuery{})
	assert.ErrorIs(t, err, notFound)
	_, err = adapter.GetScope(ctx, "scope-1")
	assert.ErrorIs(t, err, notFound)
	_, err = adapter.ListScopes(ctx, ListScopesQuery{PlatformAdminID: "pa-1"})
	assert.ErrorIs(t, err, notFound)
}

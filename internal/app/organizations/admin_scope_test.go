package organizations

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
)

func seedListedOrganizations(t *testing.T, repo *testOrganizationRepository) (targetID, otherID string) {
	ctx := context.Background()
	targetID, otherID = uuid.NewString(), uuid.NewString()
	require.NoError(t, repo.Create(ctx, &orgdomain.Organization{
		ID: targetID, Slug: "target-org", DisplayName: "Target Org", Status: "active",
	}))
	require.NoError(t, repo.Create(ctx, &orgdomain.Organization{
		ID: otherID, Slug: "other-org", DisplayName: "Other Org", Status: "active",
	}))
	return targetID, otherID
}

func TestListOrganizations_TargetedAdminWithoutTargetReturnsEmpty(t *testing.T) {
	svc, repo := newTestService()
	targetID, _ := seedListedOrganizations(t, repo)

	ctx := scopeauthorizer.WithAdminScope(context.Background(), scopeauthorizer.AdminScope{
		Class:         scopeauthorizer.AdminScopeClassTargeted,
		TargetTenants: []uuid.UUID{uuid.MustParse(targetID)},
	})

	items, total, err := svc.ListOrganizations(ctx, ListOrganizationsQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)

	names, err := svc.GetOrganizationNames(ctx)
	require.NoError(t, err)
	require.Empty(t, names)
}

func TestListOrganizations_TargetedAdminWithTargetReturnsOnlyThatTenant(t *testing.T) {
	svc, repo := newTestService()
	targetID, otherID := seedListedOrganizations(t, repo)
	target := uuid.MustParse(targetID)

	ctx := scopeauthorizer.WithAdminScope(context.Background(), scopeauthorizer.AdminScope{
		Class:         scopeauthorizer.AdminScopeClassTargeted,
		TargetTenants: []uuid.UUID{target},
	})
	ctx = scopeauthorizer.WithAuthorizedTargetTenant(ctx, target)

	items, total, err := svc.ListOrganizations(ctx, ListOrganizationsQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, targetID, items[0].ID)
	require.EqualValues(t, 1, total)
	for _, item := range items {
		require.NotEqual(t, otherID, item.ID)
	}

	names, err := svc.GetOrganizationNames(ctx)
	require.NoError(t, err)
	require.Len(t, names, 1)
	require.Equal(t, targetID, names[0].ID)
}

// seedShapeAdminRepo/seedShapeScopeRepo 最小实现：只支撑 ResolveAdminScope 的两条查询。
type seedShapeAdminRepo struct{ admin *platformadmins.PlatformAdmin }

func (r seedShapeAdminRepo) GetByID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return nil, nil
}

func (r seedShapeAdminRepo) GetByUserID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return r.admin, nil
}

func (r seedShapeAdminRepo) List(context.Context, platformadmins.ListOptions) ([]*platformadmins.PlatformAdmin, int64, error) {
	return nil, 0, nil
}

func (r seedShapeAdminRepo) Create(context.Context, *platformadmins.PlatformAdmin) error { return nil }
func (r seedShapeAdminRepo) Update(context.Context, *platformadmins.PlatformAdmin) error { return nil }
func (r seedShapeAdminRepo) SetActive(context.Context, string, bool) error               { return nil }
func (r seedShapeAdminRepo) Delete(context.Context, string) error                        { return nil }

type seedShapeScopeRepo struct {
	scopes []*platformadmins.PlatformAdminScope
}

func (r seedShapeScopeRepo) GetByID(context.Context, string) (*platformadmins.PlatformAdminScope, error) {
	return nil, nil
}

func (r seedShapeScopeRepo) ListByPlatformAdmin(context.Context, string, platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	return r.scopes, nil
}

func (r seedShapeScopeRepo) Create(context.Context, *platformadmins.PlatformAdminScope) error {
	return nil
}
func (r seedShapeScopeRepo) Approve(context.Context, string, string) error { return nil }
func (r seedShapeScopeRepo) Delete(context.Context, string) error          { return nil }
func (r seedShapeScopeRepo) HasActiveScopeForTenant(context.Context, string, string) (bool, error) {
	return false, nil
}

// 种子形态（all + approved_by NULL + 未过期）经 ResolveAdminScope 到列表必须非空，
// 覆盖 P1 回归根因：解析层与列表层之间此前没有任何端到端断言。
func TestListOrganizations_SeedAllScopeResolvesToFullList(t *testing.T) {
	svc, repo := newTestService()
	seedListedOrganizations(t, repo)

	authorizer := scopeauthorizer.NewAllowIfScopedScopeAuthorizer(
		seedShapeAdminRepo{admin: &platformadmins.PlatformAdmin{ID: "pa-1", IsActive: true}},
		seedShapeScopeRepo{scopes: []*platformadmins.PlatformAdminScope{{
			ScopeType: platformadmins.ScopeTypeAll,
			ExpiresAt: time.Now().Add(time.Hour),
			// ApprovedBy/ApprovedAt 留 nil：与 db/seeds/V1 内置 admin 行完全一致。
		}}},
		nil, nil, nil,
	)

	resolved, err := authorizer.ResolveAdminScope(context.Background(), "admin-user")
	require.NoError(t, err)
	require.Equal(t, scopeauthorizer.AdminScopeClassAll, resolved.Class)

	ctx := scopeauthorizer.WithAdminScope(context.Background(), resolved)
	items, total, err := svc.ListOrganizations(ctx, ListOrganizationsQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.EqualValues(t, 2, total)
}

// none / unknown 与 targeted 同样必须返回空集，不能被“可见性修复”带成全量。
func TestListOrganizations_NoneAndUnknownScopesReturnEmpty(t *testing.T) {
	for _, class := range []scopeauthorizer.AdminScopeClass{
		scopeauthorizer.AdminScopeClassNone,
		scopeauthorizer.AdminScopeClassUnknown,
	} {
		t.Run(string(class), func(t *testing.T) {
			svc, repo := newTestService()
			seedListedOrganizations(t, repo)

			ctx := scopeauthorizer.WithAdminScope(context.Background(), scopeauthorizer.AdminScope{Class: class})
			items, total, err := svc.ListOrganizations(ctx, ListOrganizationsQuery{Page: 1, PageSize: 10})
			require.NoError(t, err)
			require.Empty(t, items)
			require.Zero(t, total)

			names, err := svc.GetOrganizationNames(ctx)
			require.NoError(t, err)
			require.Empty(t, names)
		})
	}
}

func TestListOrganizations_AllScopeReturnsEverything(t *testing.T) {
	svc, repo := newTestService()
	seedListedOrganizations(t, repo)

	ctx := scopeauthorizer.WithAdminScope(context.Background(), scopeauthorizer.AdminScope{
		Class: scopeauthorizer.AdminScopeClassAll,
	})

	items, total, err := svc.ListOrganizations(ctx, ListOrganizationsQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.EqualValues(t, 2, total)
}

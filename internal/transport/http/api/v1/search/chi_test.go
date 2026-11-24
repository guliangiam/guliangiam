package search

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appsearch "github.com/guliangiam/guliangiam/internal/app/search"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domainapplications "github.com/guliangiam/guliangiam/internal/domain/applications"
	applicationsmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	certsmock "github.com/guliangiam/guliangiam/internal/domain/certs/mock"
	groupsmock "github.com/guliangiam/guliangiam/internal/domain/groups/mock"
	invitationsmock "github.com/guliangiam/guliangiam/internal/domain/invitations/mock"
	keysmock "github.com/guliangiam/guliangiam/internal/domain/keys/mock"
	domainorganizations "github.com/guliangiam/guliangiam/internal/domain/organizations"
	organizationsmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	permissionsmock "github.com/guliangiam/guliangiam/internal/domain/permissions/mock"
	domainroles "github.com/guliangiam/guliangiam/internal/domain/roles"
	rolesmock "github.com/guliangiam/guliangiam/internal/domain/roles/mock"
	sessionsmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	ticketsmock "github.com/guliangiam/guliangiam/internal/domain/tickets/mock"
	tokensmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// testFixture 接好全部实体仓储：users/roles/applications 的 List 与
// organizations 的 GetByID 走可替换回调（默认空结果），其余实体固定为空。
type testFixture struct {
	router            chi.Router
	usersList         func(domainusers.ListQuery) ([]*domainusers.User, int64, error)
	rolesList         func(domainroles.ListQuery) ([]*domainroles.Role, int64, error)
	applicationsList  func(domainapplications.ListQuery) ([]*domainapplications.Application, int64, error)
	organizationsByID func(string) (*domainorganizations.Organization, error)
}

func newTestFixture(t *testing.T) *testFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	f := &testFixture{}
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	organizations := organizationsmock.NewMockRepository(ctrl)
	users := usersmock.NewMockRepository(ctrl)
	applications := applicationsmock.NewMockRepository(ctrl)
	roles := rolesmock.NewMockRepository(ctrl)

	uow.EXPECT().Organizations().Return(organizations).AnyTimes()
	uow.EXPECT().Users().Return(users).AnyTimes()
	uow.EXPECT().Applications().Return(applications).AnyTimes()
	uow.EXPECT().Roles().Return(roles).AnyTimes()
	permissions := permissionsmock.NewMockRepository(ctrl)
	permissions.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	uow.EXPECT().Permissions().Return(permissions).AnyTimes()
	groups := groupsmock.NewMockRepository(ctrl)
	groups.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	uow.EXPECT().Groups().Return(groups).AnyTimes()
	certs := certsmock.NewMockRepository(ctrl)
	certs.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	uow.EXPECT().Certs().Return(certs).AnyTimes()
	keys := keysmock.NewMockRepository(ctrl)
	keys.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	uow.EXPECT().Keys().Return(keys).AnyTimes()
	tokens := tokensmock.NewMockRepository(ctrl)
	tokens.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	uow.EXPECT().Tokens().Return(tokens).AnyTimes()
	sessions := sessionsmock.NewMockRepository(ctrl)
	sessions.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	uow.EXPECT().Sessions().Return(sessions).AnyTimes()
	tickets := ticketsmock.NewMockRepository(ctrl)
	tickets.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	uow.EXPECT().Tickets().Return(tickets).AnyTimes()
	invitations := invitationsmock.NewMockRepository(ctrl)
	invitations.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	uow.EXPECT().Invitations().Return(invitations).AnyTimes()

	organizations.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil).AnyTimes()
	organizations.EXPECT().GetByID(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, id string) (*domainorganizations.Organization, error) {
			if f.organizationsByID == nil {
				return nil, errors.New("unexpected GetByID")
			}
			return f.organizationsByID(id)
		}).AnyTimes()
	users.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
			if f.usersList == nil {
				return nil, 0, nil
			}
			return f.usersList(query)
		}).AnyTimes()
	applications.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainapplications.ListQuery) ([]*domainapplications.Application, int64, error) {
			if f.applicationsList == nil {
				return nil, 0, nil
			}
			return f.applicationsList(query)
		}).AnyTimes()
	roles.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainroles.ListQuery) ([]*domainroles.Role, int64, error) {
			if f.rolesList == nil {
				return nil, 0, nil
			}
			return f.rolesList(query)
		}).AnyTimes()

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	RegisterChiRoutes(router, NewHandler(appsearch.NewService(uow)))
	f.router = router
	return f
}

// TestSearch_EmptyKeywordRejected 固定 q 缺失/空白 → 400 invalid_argument。
func TestSearch_EmptyKeywordRejected(t *testing.T) {
	for _, target := range []string{"/api/v1/search", "/api/v1/search?q=%20%20"} {
		f := newTestFixture(t)
		recorder := httptest.NewRecorder()
		f.router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		require.Equal(t, http.StatusBadRequest, recorder.Code, target)
		assert.Contains(t, recorder.Body.String(), `"code":"invalid_argument"`)
	}
}

// TestSearch_MemberCannotSearchForeignTenant 固定跨组织拒绝：组织 A 的普通成员
// 伪造 organizationId=B 时，仓储仍只收到本组织过滤条件（搜不到组织 B 的数据）。
func TestSearch_MemberCannotSearchForeignTenant(t *testing.T) {
	f := newTestFixture(t)
	var capturedOrg string
	f.usersList = func(query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
		capturedOrg = query.OrganizationID
		return nil, 0, nil
	}
	f.applicationsList = func(query domainapplications.ListQuery) ([]*domainapplications.Application, int64, error) {
		assert.Equal(t, "org-a", query.OrganizationID)
		return []*domainapplications.Application{{ID: "app-1", Slug: "portal", DisplayName: "Portal"}}, 1, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=portal&organizationId=org-b", nil)
	ctx := tenantctx.WithTenantScope(req.Context(), "org-a", "acme", tenantctx.TenantFilterStrict)
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, req.WithContext(ctx))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "org-a", capturedOrg, "伪造的 organizationId 不得进入查询条件")

	var result appsearch.Result
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&result))
	require.Len(t, result.Groups, 1)
	assert.Equal(t, "applications", result.Groups[0].Entity)
}

// TestSearch_PlatformAdminCrossTenant 固定平台管理员跨组织聚合：super_admin
// 升级且未指定 organizationId 时，仓储收到空过滤条件（可见范围由 RLS 决定）；
// 显式指定时收窄到该组织。
func TestSearch_PlatformAdminCrossTenant(t *testing.T) {
	t.Run("empty organizationId aggregates across tenants", func(t *testing.T) {
		f := newTestFixture(t)
		f.rolesList = func(query domainroles.ListQuery) ([]*domainroles.Role, int64, error) {
			assert.Empty(t, query.OrganizationID)
			return nil, 0, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=adm", nil)
		ctx := tenantctx.WithTenantScope(req.Context(), "org-builtin", "built-in", tenantctx.TenantFilterSuperAdmin)
		recorder := httptest.NewRecorder()
		f.router.ServeHTTP(recorder, req.WithContext(ctx))
		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("explicit organizationId narrows the tenant", func(t *testing.T) {
		f := newTestFixture(t)
		f.usersList = func(query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
			assert.Equal(t, "org-c", query.OrganizationID)
			return []*domainusers.User{{ID: "user-1", OrganizationID: "org-c", Username: "carol"}}, 1, nil
		}
		f.organizationsByID = func(id string) (*domainorganizations.Organization, error) {
			return nil, errors.New("not visible")
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=carol&organizationId=org-c", nil)
		ctx := tenantctx.WithTenantScope(req.Context(), "org-builtin", "built-in", tenantctx.TenantFilterSuperAdmin)
		recorder := httptest.NewRecorder()
		f.router.ServeHTTP(recorder, req.WithContext(ctx))
		require.Equal(t, http.StatusOK, recorder.Code)

		var result appsearch.Result
		require.NoError(t, json.NewDecoder(recorder.Body).Decode(&result))
		require.Len(t, result.Groups, 1)
		assert.Equal(t, "users", result.Groups[0].Entity)
		// slug 补查失败只降级为空，不影响整体返回。
		assert.Empty(t, result.Groups[0].Items[0].OrganizationSlug)
	})
}

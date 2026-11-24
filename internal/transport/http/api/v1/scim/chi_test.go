package scim

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appscim "github.com/guliangiam/guliangiam/internal/app/scim"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
)

func TestHandler_ListUsers_MissingOrgID(t *testing.T) {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	h := &Handler{ScimService: appscim.Service{}}
	r.Get("/scim/v2/Users", h.ListUsers)

	req := httptest.NewRequest("GET", "/scim/v2/Users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetUser_MissingID(t *testing.T) {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	h := &Handler{ScimService: appscim.Service{}}
	r.Get("/scim/v2/Users/{id}", h.GetUser)

	req := httptest.NewRequest("GET", "/scim/v2/Users/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEqual(t, 0, w.Code)
}

func TestHandler_ListGroups_MissingOrgID(t *testing.T) {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	h := &Handler{ScimService: appscim.Service{}}
	r.Get("/scim/v2/Groups", h.ListGroups)

	req := httptest.NewRequest("GET", "/scim/v2/Groups", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResolveOrgID(t *testing.T) {
	t.Run("from_trusted_tenant_scope", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/?organizationId=forged-query", nil)
		r.Header.Set("X-Organization-Id", "forged-header")
		r = r.WithContext(tenantctx.WithTenantScope(r.Context(), "org-trusted", "", tenantctx.TenantFilterStrict))
		assert.Equal(t, "org-trusted", resolveOrgID(r))
	})

	t.Run("forged_query_and_header_ignored", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/?organizationId=org-1", nil)
		r.Header.Set("X-Organization-Id", "org-2")
		assert.Equal(t, "", resolveOrgID(r))
	})

	t.Run("empty_scope", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		assert.Equal(t, "", resolveOrgID(r))
	})
}

func TestScimStatusCodeFromError(t *testing.T) {
	assert.Equal(t, http.StatusOK, scimStatusCodeFromError(nil))
	assert.Equal(t, http.StatusInternalServerError, scimStatusCodeFromError(http.ErrAbortHandler))
}

// INC-BE-06 回归：ChiRouter 以 Mount("/") 挂到主路由时会兜住所有未匹配路径。
// Bearer 认证限定在 /scim/v2 子树后，公开前缀下的未知路径必须返回 404，
// 而不是落入 SCIM 中间件返回 SCIM 格式 401；/scim/v2/* 仍须 Bearer 认证。
func TestChiRouter_UnknownPublicPathReturns404NotSCIM401(t *testing.T) {
	root := chi.NewRouter()
	root.Mount("/", ChiRouter(&Handler{}, appapplications.Service{}))

	for _, path := range []string{
		"/.well-known/openid-configuration",
		"/.well-known/unknown-doc",
		"/oauth2/unknown-endpoint",
		"/totally-unknown-path",
	} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		root.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, "path %s 应 404，不得被 SCIM 认证兜成 401", path)
	}

	// SCIM 数据/发现端点仍受 Bearer 认证保护：缺 Authorization → 401。
	req := httptest.NewRequest("GET", "/scim/v2/Users", nil)
	w := httptest.NewRecorder()
	root.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

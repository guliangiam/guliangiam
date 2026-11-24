package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// BE-E-10 回归：白名单扩容后非法 UUID 统一 400；业务键路由不受影响；
// {id}:verb 自定义方法段也能被识别。

func newGuardTestRouter() *chi.Mux {
	// 必须用 Group（inline mux）挂载，与生产 bootstrap/chi.go 的
	// router.Group(nil)+protected.Use 一致：Group 的中间件包在端点上、
	// 路由匹配之后执行，RoutePattern() 才有值。根 mux 的 Use 先于路由，
	// guard 会拿到空 pattern（不代表生产行为）。
	r := chi.NewRouter()
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	r.Group(func(g chi.Router) {
		g.Use(UUIDParamGuard)
		g.Get("/api/v1/tickets/{id}", ok)
		g.Post("/api/v1/pay-channel-configs/{id}:transition", ok)
		g.Get("/api/v1/organizations/{id}", ok) // slug，豁免
		g.Get("/api/v1/organizations/{orgId}/domains/{domainId}", ok)
		g.Get("/api/v1/privacy/consents/{purpose}", ok) // purpose 非 id 形参
		g.Get("/api/v1/rebac/namespaces/{namespaceId}", ok)
		g.Get("/api/v1/webhook-events/{id}", ok) // 自校验，豁免
		g.Get("/api/v1/applications/{id}", ok)
		g.Get("/api/v1/applications/by-client-id/{clientId}", ok) // client_id 字符串，豁免
	})
	return r
}

func doGuardRequest(t *testing.T, r *chi.Mux, method, path string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}

func TestUUIDParamGuard_ExpandedWhitelistRejectsInvalidUUID(t *testing.T) {
	r := newGuardTestRouter()
	valid := uuid.NewString()
	cases := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/v1/tickets/not-a-uuid", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/tickets/" + valid, http.StatusOK},
		{http.MethodPost, "/api/v1/pay-channel-configs/not-a-uuid:transition", http.StatusBadRequest},
		{http.MethodPost, "/api/v1/pay-channel-configs/" + valid + ":transition", http.StatusOK},
		{http.MethodGet, "/api/v1/rebac/namespaces/not-a-uuid", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/organizations/" + valid + "/domains/not-a-uuid", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/organizations/not-a-slug", http.StatusOK},
		{http.MethodGet, "/api/v1/privacy/consents/marketing", http.StatusOK},
		{http.MethodGet, "/api/v1/webhook-events/not-a-uuid", http.StatusOK},
		{http.MethodGet, "/api/v1/applications/not-a-uuid", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/applications/" + valid, http.StatusOK},
		// OIDC 登录页用 OAuth client_id 查询应用，不能按 UUID 校验
		{http.MethodGet, "/api/v1/applications/by-client-id/gl_cid_L3UgfbZ3lKqdaCzKOsVcYYFqybB2CgZw", http.StatusOK},
		{http.MethodGet, "/api/v1/applications/by-client-id/built-in-client", http.StatusOK},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, doGuardRequest(t, r, tc.method, tc.path), tc.path)
	}
}

func TestUUIDParamGuard_IDParamNamesStripsVerbSuffix(t *testing.T) {
	assert.Equal(t, []string{"id"}, idParamNames("/api/v1/pay-channel-configs/{id}:transition"))
	assert.Equal(t, []string{"orgId", "domainId"}, idParamNames("/api/v1/organizations/{orgId}/domains/{domainId}"))
	assert.Empty(t, idParamNames("/api/v1/privacy/consents/{purpose}"))
}

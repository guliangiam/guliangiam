package bootstrap

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// 显式登记（Permission=public）与根路由显式清单都必须放行，不能误伤。
func TestValidatePublicRouteWhitelist_ExplicitEntriesPass(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/healthz", func(http.ResponseWriter, *http.Request) {})
	router.Get("/api/v1/auth/captcha", func(http.ResponseWriter, *http.Request) {})
	router.Get("/cas/{organizationSlug}/{applicationSlug}/validate", func(http.ResponseWriter, *http.Request) {})

	definitions := []route.Definition{
		{Method: http.MethodGet, Pattern: "/api/v1/auth/captcha", Permission: "public", Surface: route.SurfaceAuthentication},
		{Method: http.MethodGet, Pattern: "/cas/{organizationSlug}/{applicationSlug}/validate", Permission: "cas.validate", Surface: route.SurfaceProtocol},
	}

	require.NoError(t, validatePublicRouteWhitelist(router, definitions))
}

// 只被前缀「顺带」覆盖、自身声明又不是公开/协议面的路由必须在启动期被拒绝。
func TestValidatePublicRouteWhitelist_PrefixOnlyCoverageFails(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/api/v1/organizations/by-slug/{slug}/audit", func(http.ResponseWriter, *http.Request) {})

	definitions := []route.Definition{
		{
			Method:     http.MethodGet,
			Pattern:    "/api/v1/organizations/by-slug/{slug}/audit",
			Permission: "organizations:read",
			Surface:    route.SurfaceManagement,
		},
	}

	err := validatePublicRouteWhitelist(router, definitions)
	require.Error(t, err)
	require.Contains(t, err.Error(), "GET /api/v1/organizations/by-slug/{slug}/audit")
	require.Contains(t, err.Error(), "/api/v1/organizations/by-slug/")
	require.True(t, strings.Contains(err.Error(), "顺带覆盖"))
}

// 回归（2026-11-17 裸启动 panic）：Handle 全方法兜底挂载会被 chi.Walk 枚举出
// 每个方法，只被 family 前缀顺带覆盖时必须在启动期被拒；静态代理必须像
// chi.go 的 uploads 挂载那样按方法注册 GET/HEAD（两者已在 rootPublicRoutes
// 逐条登记）。
func TestValidatePublicRouteWhitelist_HandleMountRejectedPerMethodPasses(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	catchAll := chi.NewRouter()
	catchAll.Handle("/uploads/*", handler)
	err := validatePublicRouteWhitelist(catchAll, nil)
	require.Error(t, err, "Handle 兜底挂载必须被逐方法拒绝")
	require.Contains(t, err.Error(), "POST /uploads/*")
	require.Contains(t, err.Error(), "DELETE /uploads/*")
	require.NotContains(t, err.Error(), "GET /uploads/*", "GET 已登记，不应出现在违规集")

	perMethod := chi.NewRouter()
	perMethod.Get("/uploads/*", handler)
	perMethod.Head("/uploads/*", handler)
	require.NoError(t, validatePublicRouteWhitelist(perMethod, nil))
}

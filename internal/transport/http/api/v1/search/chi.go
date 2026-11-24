package search

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	appsearch "github.com/guliangiam/guliangiam/internal/app/search"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes 挂载 GET /api/v1/search。
func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		request, err := parseSearchRequest(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.Search(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
}

// parseSearchRequest 校验查询参数并解析可信租户边界。
func parseSearchRequest(r *http.Request) (SearchRequest, error) {
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	if keyword == "" {
		return SearchRequest{}, apperrors.NewInvalidArgument("q is required")
	}
	organizationID, err := trustedSearchOrganizationID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		return SearchRequest{}, err
	}
	return SearchRequest{
		Keyword:        keyword,
		OrganizationID: organizationID,
		Limit:          route.QueryInt(r, "limit", appsearch.DefaultLimit),
	}, nil
}

// trustedSearchOrganizationID 解析搜索的租户边界：请求里的 organizationId 不是
// 可信输入。普通成员一律限定在 TenantFilterMiddleware 注入的本组织 scope（空值
// 直接拒绝，仓储显式 organization_id 过滤）；平台管理员经 super_admin/system
// 升级后，可显式指定 organizationId 收窄到单组织，或留空做跨组织聚合（可见
// 范围由 RLS 升级决定，与 /api/v1/global/* 同一口径）。
func trustedSearchOrganizationID(r *http.Request, requested string) (string, error) {
	scope := tenantctx.TenantScopeFromContext(r.Context())
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		return strings.TrimSpace(requested), nil
	}
	organizationID := authtransport.TrustedOrganizationID(r, requested)
	if organizationID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return organizationID, nil
}

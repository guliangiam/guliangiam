package b2b

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes 把 B2B 域名治理路由注册到 chi.Router。
//
// 调用方负责：
//   - 注入 handler（含 app/b2b.Service 依赖）
//   - 外层 chi.Router 已挂载认证 / 权限中间件
//   - Permission 检查走 routes.go 定义的 surface（Management）
func RegisterChiRoutes(router chi.Router, handler Handler) {
	// POST /api/v1/organizations/{orgId}/domains
	router.Post("/api/v1/organizations/{orgId}/domains", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedPathOrgID(r, chi.URLParam(r, "orgId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req ClaimDomainRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		req.OrganizationID = organizationID

		resp, err := handler.ClaimDomain(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, resp)
	})

	// GET /api/v1/organizations/{orgId}/domains
	router.Get("/api/v1/organizations/{orgId}/domains", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, chi.URLParam(r, "orgId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		req := ListDomainsRequest{
			OrganizationID: organizationID,
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			Status:         r.URL.Query().Get("status"),
		}

		resp, err := handler.ListDomains(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	// DELETE /api/v1/organizations/{orgId}/domains/{domainId}
	router.Delete("/api/v1/organizations/{orgId}/domains/{domainId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, chi.URLParam(r, "orgId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		req := DeleteDomainRequest{
			OrganizationID: organizationID,
			DomainID:       chi.URLParam(r, "domainId"),
		}
		if err := handler.DeleteDomain(r.Context(), req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// POST /api/v1/organizations/{orgId}/domains/{domainId}/verify
	router.Post("/api/v1/organizations/{orgId}/domains/{domainId}/verify", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedPathOrgID(r, chi.URLParam(r, "orgId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		req := VerifyDomainRequest{
			OrganizationID: organizationID,
			DomainID:       chi.URLParam(r, "domainId"),
		}
		resp, err := handler.VerifyDomain(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if resp == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
}

// trustedOrgID resolves the tenant a request may act on. A caller-supplied
// path orgId is honoured only for platform administrators; otherwise the
// authenticated tenant scope wins. An empty result is a client error: on
// dialects without RLS an empty organization_id degrades to "all tenants".
func trustedOrgID(r *http.Request, requested string) (string, error) {
	orgID := authtransport.TrustedOrganizationID(r, requested)
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return orgID, nil
}

// trustedPathOrgID resolves the tenant for routes that carry the organization
// in the path. Unlike trustedOrgID, the path value is not silently replaced:
// a regular caller's path orgId must equal the trusted tenant, and the
// platform-admin override is only the explicit target. A mismatch maps to
// NotFound so a caller cannot probe whether the organization exists.
func trustedPathOrgID(r *http.Request, requested string) (string, error) {
	orgID := authtransport.TrustedOrganizationID(r, requested)
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	if !sameOrganizationID(orgID, requested) {
		return "", apperrors.NewNotFound("organization not found")
	}
	return orgID, nil
}

// sameOrganizationID treats the same UUID in different letter case as one
// organization (PostgreSQL's uuid cast is case-insensitive), while any other
// difference stays a mismatch.
func sameOrganizationID(a, b string) bool {
	if a == b {
		return true
	}
	parsedA, errA := uuid.Parse(a)
	parsedB, errB := uuid.Parse(b)
	return errA == nil && errB == nil && parsedA == parsedB
}

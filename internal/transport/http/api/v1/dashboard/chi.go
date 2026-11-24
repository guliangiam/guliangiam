package dashboard

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// allTenantsScope is the explicit tenant argument that asks dashboard
// repositories for the cross-tenant overview. Repository SQL interprets an
// empty organization_id as "no tenant filter" (empty-or-match), so "" is the
// existing product contract for platform-wide totals.
// Only trustedOrgID's platform-admin branch can produce this value.
const allTenantsScope = ""

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/dashboard/stats", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetDashboardStats(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/dashboard/users", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetUserStats(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/dashboard/auth", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetAuthStats(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/dashboard/resources", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetResourceStats(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/dashboard/provider-distribution", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.GetProviderDistribution(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/dashboard/mfa-coverage", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.GetMfaCoverage(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/dashboard/login-heatmap", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.GetLoginHeatmap(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/dashboard/trends", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.GetTrends(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/dashboard/activity-heatmap", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.GetActivityHeatmap(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/dashboard/org-user-distribution", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.GetOrgUserDistribution(r.Context())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})
}

// trustedOrgID resolves the tenant scope for a dashboard read.
//
// A caller-supplied organizationId is only a narrowing filter, and it is
// honoured only when the request carries an explicit platform-level scope
// (super_admin/system, injected by TenantFilterMiddleware after the
// platform-admin check). Regular callers always read their authenticated
// tenant, so a forged query parameter cannot widen or move the query.
//
// An empty tenant on a non-platform request is a client error: dashboard SQL
// treats an empty organization_id as all tenants, and on dialects without RLS
// an empty value would silently degrade to a cross-tenant read.
func trustedOrgID(r *http.Request, requested string) (string, error) {
	scope := tenantctx.TenantScopeFromContext(r.Context())
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		// KEEP existing product semantics: a platform administrator that does
		// not name a tenant gets the cross-tenant overview (allTenantsScope),
		// not their home tenant. The explicit mode check is what makes "" safe.
		return requested, nil
	}

	organizationID := authtransport.TrustedOrganizationID(r, requested)
	if organizationID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return organizationID, nil
}

package organizations

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// trustedOrganizationID resolves the organization a by-ID route may address.
//
// The organizations row key is the tenant key, so `WHERE id = $1` already is
// the explicit organization predicate on both dialects; the fix is to make sure
// the id handed to the app layer is trusted before it reaches the repository.
// A regular tenant is pinned to the organization resolved by
// TenantFilterMiddleware; only a platform administrator (super_admin/system
// scope) may address a different one.
//
// A cross-tenant id and a non-existent id both map to NotFound: the response
// must not disclose whether the row exists.
func trustedOrganizationID(r *http.Request) (string, error) {
	requested := chi.URLParam(r, "id")
	// {id} addresses a uuid column. For super_admin/system scopes
	// TrustedOrganizationID returns the requested value unchanged, so an opaque
	// slug would reach the repository and surface the cast failure as a 500
	// instead of NotFound. A regular tenant is pinned to the organization
	// resolved by TenantFilterMiddleware, so non-UUID values there can only
	// match (or fail to match) the session organization and remain 404s.
	scopeMode := tenantctx.TenantScopeFromContext(r.Context()).Mode
	if scopeMode == tenantctx.TenantFilterSuperAdmin || scopeMode == tenantctx.TenantFilterSystem {
		if _, err := uuid.Parse(requested); err != nil {
			return "", apperrors.NewNotFound("organization not found")
		}
	}
	organizationID := authtransport.TrustedOrganizationID(r, requested)
	if organizationID == "" || !sameOrganizationID(organizationID, requested) {
		return "", apperrors.NewNotFound("organization not found")
	}

	return organizationID, nil
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

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/organizations", func(w http.ResponseWriter, r *http.Request) {
		request := ListOrganizationsRequest{
			Page:     route.QueryInt(r, "page", 1),
			PageSize: route.QueryInt(r, "pageSize", route.DefaultPageSize),
			Keyword:  r.URL.Query().Get("keyword"),
			Status:   r.URL.Query().Get("status"),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/organizations", func(w http.ResponseWriter, r *http.Request) {
		var request CreateOrganizationRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.Create(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteCreated(w, "/api/v1/organizations/"+response.ID, response)
	})

	router.Get("/api/v1/organizations/{id}/policy-inheritance/preview", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		preview, err := handler.PreviewPolicyInheritance(r.Context(), organizationID, r.URL.Query().Get("policyKey"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, preview)
	})

	router.Get("/api/v1/organizations/deployment-regions", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.ListDeploymentRegions(r.Context())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"items": response})
	})

	router.Get("/api/v1/organizations/{id}", func(w http.ResponseWriter, r *http.Request) {
		// Detail reads carry masked credential fields and must not be replayed
		// from a browser cache after a save; declaring no-store here also makes
		// the ETag middleware skip its `private, no-cache` branch.
		w.Header().Set("Cache-Control", "no-store")
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.Get(r.Context(), GetOrganizationRequest{ID: organizationID})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/organizations/by-slug/{slug}", func(w http.ResponseWriter, r *http.Request) {
		// Slug discovery is a detail-level read (branding/default app), so it
		// follows the same no-store policy as the by-ID detail route.
		w.Header().Set("Cache-Control", "no-store")
		response, err := handler.GetBySlugPublic(r.Context(), GetOrganizationBySlugRequest{Slug: chi.URLParam(r, "slug")})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Patch("/api/v1/organizations/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request PatchOrganizationRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.Patch(r.Context(), organizationID, request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})

	router.Delete("/api/v1/organizations/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.Delete(r.Context(), organizationID); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	router.Put("/api/v1/organizations/{id}/configuration", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request UpdateOrganizationConfigurationRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.UpdateConfiguration(r.Context(), organizationID, request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})

	router.Put("/api/v1/organizations/{id}/settings", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request UpdateOrganizationSettingsRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.UpdateSettings(r.Context(), organizationID, request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})

	router.Put("/api/v1/organizations/{id}/theme", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request UpdateOrganizationThemeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.UpdateTheme(r.Context(), organizationID, request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})

	router.Put("/api/v1/organizations/{id}/security-policy", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request UpdateOrganizationSecurityPolicyRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.UpdateSecurityPolicy(r.Context(), organizationID, request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})

	router.Get("/api/v1/organizations/{id}/settings", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetSettings(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/organizations/{id}/theme", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetTheme(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/organizations/{id}/security-policy", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetSecurityPolicy(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/organizations/{id}/applications", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		page := route.QueryInt(r, "page", 1)
		pageSize := route.QueryInt(r, "pageSize", route.DefaultPageSize)

		response, err := handler.ListOrganizationApplications(r.Context(), organizationID, page, pageSize)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// Get default application for an organization.
	router.Get("/api/v1/organizations/{id}/default-application", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrganizationID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.GetDefaultApplication(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})

	// Resolve organization slug → 完整详情（public, pre-login discovery）。
	// 登录 SPA 需要在 session 之前获取 built-in org 的品牌配置、默认应用等。
	router.Get("/api/v1/organizations/resolve", func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("slug")
		if slug == "" {
			route.WriteError(w, r, apperrors.NewInvalidArgument("slug query required"))
			return
		}

		response, err := handler.GetBySlugPublic(r.Context(), GetOrganizationBySlugRequest{Slug: slug})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// Get organization names (lightweight listing).
	router.Get("/api/v1/organizations/names", func(w http.ResponseWriter, r *http.Request) {
		result, err := handler.GetOrganizationNames(r.Context())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
}

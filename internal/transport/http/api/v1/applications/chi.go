package applications

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/applications", func(w http.ResponseWriter, r *http.Request) {
		request := ListApplicationsRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId")),
			Keyword:        r.URL.Query().Get("keyword"),
			Category:       r.URL.Query().Get("category"),
			Status:         r.URL.Query().Get("status"),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/applications", func(w http.ResponseWriter, r *http.Request) {
		var request CreateApplicationRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// Tenant authority is the authenticated scope; the body may only
		// request a target tenant for a platform administrator.
		request.OrganizationID = authtransport.TrustedOrganizationID(r, request.OrganizationID)

		response, err := handler.Create(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteCreated(w, "/api/v1/applications/"+response.ID, response)
	})

	router.Get("/api/v1/applications/{id}", func(w http.ResponseWriter, r *http.Request) {
		// Detail reads carry masked credential fields and must not be replayed
		// from a browser cache after a save; declaring no-store here also makes
		// the ETag middleware skip its `private, no-cache` branch.
		w.Header().Set("Cache-Control", "no-store")
		// 详情 GET 是详情页的入口读：超管/系统作用域可省略 organizationId，
		// 应用层改走系统域 GetByID；普通租户仍强制可信租户。
		organizationID, err := trustedOrgIDForDetailRead(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.Get(r.Context(), GetApplicationRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/applications/{id}/delete-summary", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetDeleteSummary(r.Context(), GetApplicationRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Patch("/api/v1/applications/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request PatchApplicationRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.Patch(r.Context(), organizationID, chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/applications/{id}/client-secret/rotate", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.RotateClientSecret(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Delete("/api/v1/applications/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.Delete(r.Context(), GetApplicationRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		}); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	router.Get("/api/v1/applications/by-client-id/{clientId}", func(w http.ResponseWriter, r *http.Request) {
		clientID := chi.URLParam(r, "clientId")
		// 匿名预登录发现路由：System 只读定位在 app 层
		// GetApplicationPublicByClientID 内完成（INC-BE-05），transport 不提升。
		response, err := handler.GetByClientID(r.Context(), clientID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/applications/resolve", func(w http.ResponseWriter, r *http.Request) {
		ownerSlug := r.URL.Query().Get("ownerSlug")
		appSlug := r.URL.Query().Get("appSlug")
		if ownerSlug == "" || appSlug == "" {
			route.WriteError(w, r, apperrors.NewInvalidArgument("ownerSlug and appSlug are required"))
			return
		}

		response, err := handler.Resolve(r.Context(), ownerSlug, appSlug)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/applications/{id}/provider-bindings", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetProviderBindings(r.Context(), GetApplicationRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Put("/api/v1/applications/{id}/provider-bindings", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request ReplaceApplicationProviderBindingsRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.ReplaceProviderBindings(r.Context(), organizationID, chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// 客户端证书绑定：GET 只返回指纹与元数据，PUT 为替换式全量写入。
	router.Get("/api/v1/applications/{id}/client-certificates", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.GetClientCertificates(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Put("/api/v1/applications/{id}/client-certificates", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request ReplaceApplicationClientCertificatesRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.ReplaceClientCertificates(r.Context(), organizationID, chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})

	// Export application configuration as JSON
	router.Get("/api/v1/applications/{id}/export", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		payload, err := handler.Export(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		filename := payload.Export.Slug
		if filename == "" {
			filename = chi.URLParam(r, "id")
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-config.json", filename))
		route.WriteJSON(w, http.StatusOK, payload)
	})

	// Import application configuration from JSON
	router.Post("/api/v1/applications/import", func(w http.ResponseWriter, r *http.Request) {
		var request ImportApplicationRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = authtransport.TrustedOrganizationID(r, request.OrganizationID)

		response, err := handler.Import(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteCreated(w, "/api/v1/applications/"+response.ID, response)
	})

	// Get the default application for the current user's organization (returns full ApplicationDetail)
	router.Get("/api/v1/applications/user-application", func(w http.ResponseWriter, r *http.Request) {
		orgID := extractOrganizationIDFromToken(r, handler.JWTIssuer)
		if orgID == "" {
			route.WriteError(w, r, apperrors.NewUnauthorized("unauthorized"))
			return
		}

		result, err := handler.GetUserApplication(r.Context(), orgID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		typed := ApplicationResponse(*result)
		route.WriteJSON(w, http.StatusOK, typed)
	})

	// Get application names (lightweight listing).
	router.Get("/api/v1/applications/names", func(w http.ResponseWriter, r *http.Request) {
		result, err := handler.GetApplicationNames(r.Context())
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
}

// trustedOrgID resolves the tenant a request may act on. A caller-supplied
// organizationId is honoured only for platform administrators; otherwise the
// authenticated tenant scope wins. An empty result is a client error: on
// dialects without RLS an empty organization_id degrades to "all tenants".
func trustedOrgID(r *http.Request, requested string) (string, error) {
	orgID := authtransport.TrustedOrganizationID(r, requested)
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return orgID, nil
}

// trustedOrgIDForDetailRead resolves the trusted tenant for a single-resource
// read. A super-admin/system scope is explicitly allowed to omit
// organizationId: the read then falls through to the system-scoped lookup
// (GetByID) instead of being narrowed to the session organization — a platform
// admin's session organization frequently differs from the target resource's
// owner (e.g. an app created in another org), and narrowing would surface a
// spurious 404. Any non-super-admin scope keeps the strict trusted-tenant rule.
func trustedOrgIDForDetailRead(r *http.Request, requested string) (string, error) {
	scope := tenantctx.TenantScopeFromContext(r.Context())
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		return strings.TrimSpace(requested), nil
	}
	return trustedOrgID(r, requested)
}

// extractOrganizationIDFromToken extracts the organization_id claim from the JWT in the request.
func extractOrganizationIDFromToken(r *http.Request, jwtIssuer interface {
	ValidateToken(token string) (map[string]interface{}, error)
}) string {
	token := accessTokenFromRequest(r)
	if token == "" {
		return ""
	}

	claims, err := jwtIssuer.ValidateToken(token)
	if err != nil {
		return ""
	}

	orgID, _ := claims["organization_id"].(string)
	return orgID
}

func accessTokenFromRequest(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("X-Access-Token")); token != "" {
		return token
	}

	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorization == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return strings.TrimSpace(authorization[7:])
	}

	return authorization
}

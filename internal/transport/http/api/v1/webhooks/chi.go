package webhooks

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/webhooks", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request := ListWebhooksRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: organizationID,
			Keyword:        r.URL.Query().Get("keyword"),
			IsEnabled:      route.QueryOptionalBool(r, "isEnabled"),
			Method:         r.URL.Query().Get("method"),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/webhooks", func(w http.ResponseWriter, r *http.Request) {
		var request CreateWebhookRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		organizationID, err := trustedOrgID(r, request.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID

		secretPlain, err := handler.Create(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		// secretPlain 只在此响应出现一次；读接口永不回传（DEF-45 A）。
		route.WriteJSON(w, http.StatusCreated, presenter.NewActionResponseWithData("created", "resource created", map[string]any{"secretPlain": secretPlain}))
	})

	router.Get("/api/v1/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedEventOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Get(r.Context(), organizationID, GetWebhookRequest{ID: chi.URLParam(r, "id")})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Patch("/api/v1/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedEventOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request PatchWebhookRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		if err := handler.Patch(r.Context(), organizationID, chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})

	router.Delete("/api/v1/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedEventOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Delete(r.Context(), organizationID, chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	// Webhook event endpoints
	router.Get("/api/v1/webhooks/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedEventOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var status *string
		if v := r.URL.Query().Get("status"); v != "" {
			status = &v
		}
		resp, err := handler.ListEvents(r.Context(), chi.URLParam(r, "id"), organizationID,
			route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize),
			r.URL.Query().Get("eventType"), status)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	router.Get("/api/v1/webhook-events/{eventId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedEventOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.GetEvent(r.Context(), chi.URLParam(r, "eventId"), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	router.Post("/api/v1/webhook-events/{eventId}/replay", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedEventOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.ReplayEvent(r.Context(), chi.URLParam(r, "eventId"), organizationID); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("replayed", "operation replayed"))
	})

	router.Delete("/api/v1/webhook-events/{eventId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedEventOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.DeleteEvent(r.Context(), chi.URLParam(r, "eventId"), organizationID); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
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

// trustedEventOrgID resolves the tenant scope for webhook delivery records.
//
// webhook_events has no organization_id column, so its rows inherit the tenant
// of their parent webhook. The app layer enforces that parent ownership. An
// empty return is the platform scope (super_admin/system) and means "all
// tenants"; only such a scope may narrow it with an explicit organizationId.
// Regular callers always get their authenticated tenant, and an empty trusted
// tenant is a client error so MySQL (no RLS) cannot degrade to a cross-tenant
// read.
func trustedEventOrgID(r *http.Request) (string, error) {
	scope := tenantctx.TenantScopeFromContext(r.Context())
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		return r.URL.Query().Get("organizationId"), nil
	}

	orgID := authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return orgID, nil
}

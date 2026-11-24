package webhookevents

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

// RegisterChiRoutes wires the webhookevents endpoints. Complements the
// scoped event list at GET /api/v1/webhooks/{id}/events by providing a
// cross-webhook audit view.
func RegisterChiRoutes(router chi.Router, handler Handler) {
	// GET /api/v1/webhook-events — cross-webhook audit list
	router.Get("/api/v1/webhook-events", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, total, err := handler.List(r.Context(), ListQuery{
			WebhookID:      r.URL.Query().Get("webhookId"),
			EventType:      r.URL.Query().Get("eventType"),
			Status:         queryStringPtr(r, "status"),
			OrganizationID: organizationID,
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(resp, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})
}

// trustedOrgID resolves the tenant scope for the cross-webhook audit list.
//
// webhook_events has no organization_id column, but it does have RLS: its
// policy (guliangiam.is_super_admin() OR webhook_id IN (SELECT … FROM
// guliangiam.webhooks WHERE organization_id = current_organization_id()))
// inherits the tenant of the parent webhook. The app/repository layers add the
// same EXISTS predicate on guliangiam.webhooks for defense in depth. The
// organizationId query parameter is therefore a narrowing filter, never a
// tenant source.
//
// An explicit platform-level scope (super_admin/system, injected by
// TenantFilterMiddleware after the platform-admin check) may name a tenant, and
// an empty value keeps the all-tenant audit view for those callers only.
// Regular callers always get their authenticated tenant; an empty trusted
// tenant is a client error so a forged or missing parameter cannot degrade to a
// cross-tenant read (MySQL has no RLS to compensate).
func trustedOrgID(r *http.Request, requested string) (string, error) {
	scope := tenantctx.TenantScopeFromContext(r.Context())
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		if requested == "" {
			return "", nil
		}
		// webhook_events resolves the tenant with a uuid cast; reject malformed
		// input here so it cannot surface as a 500.
		organizationID, err := uuid.Parse(requested)
		if err != nil {
			return "", apperrors.NewInvalidArgument("organizationId must be a UUID")
		}
		return organizationID.String(), nil
	}

	organizationID := authtransport.TrustedOrganizationID(r, requested)
	if organizationID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return organizationID, nil
}

func queryStringPtr(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

package sessions

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/sessions/current", func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authtransport.OpaqueSessionIDFromRequest(r)
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("current session is unavailable"))
			return
		}

		response, err := handler.GetCurrent(r.Context(), GetSessionRequest{ID: sessionID})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/sessions/current/revoke-others", func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authtransport.OpaqueSessionIDFromRequest(r)
		if !ok {
			route.WriteError(w, r, apperrors.NewUnauthorized("current session is unavailable"))
			return
		}

		response, err := handler.RevokeOthers(r.Context(), GetSessionRequest{ID: sessionID})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request := ListSessionsRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: organizationID,
			UserID:         r.URL.Query().Get("userId"),
			ApplicationID:  r.URL.Query().Get("applicationId"),
			State:          r.URL.Query().Get("state"),
			Keyword:        r.URL.Query().Get("keyword"),
			ActiveOnly:     route.QueryOptionalBool(r, "activeOnly"),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Get(r.Context(), GetSessionRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Delete("/api/v1/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Delete(r.Context(), GetSessionRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		}); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	router.Post("/api/v1/sessions/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Revoke(r.Context(), GetSessionRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		}); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("revoked", "resource revoked"))
	})

	router.Patch("/api/v1/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request PatchSessionRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Update(r.Context(), organizationID, chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "operation succeeded"))
	})

	// Check if a user has duplicate sessions.
	router.Post("/api/v1/sessions/is-duplicated", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID           string `json:"userId"`
			CurrentSessionID string `json:"currentSessionId"`
		}
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}

		duplicated, err := handler.IsDuplicated(r.Context(), req.UserID, req.CurrentSessionID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, map[string]any{"duplicated": duplicated})
	})

	// Force-logout a user: revoke all of their active sessions (AC-RISK-005).
	router.Post("/api/v1/sessions/force-logout", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID string `json:"userId"`
			// OrganizationID is optional: platform administrators name the target
			// tenant explicitly, everyone else is pinned to the authenticated scope.
			OrganizationID string `json:"organizationId"`
		}
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if req.UserID == "" {
			route.WriteError(w, r, apperrors.NewInvalidArgument("userId is required"))
			return
		}

		// The authenticated scope is the authority for the tenant, so a forged
		// body value cannot revoke another tenant's sessions. The target user is
		// resolved through the scoped lookup before anything is revoked or audited.
		organizationID, err := trustedOrgID(r, req.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.ForceLogoutUserForOrganization(r.Context(), organizationID, req.UserID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// Force-logout an entire organization: revoke every active session (AC-RISK-005).
	router.Post("/api/v1/sessions/force-logout-organization", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OrganizationID string `json:"organizationId"`
		}
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if req.OrganizationID == "" {
			route.WriteError(w, r, apperrors.NewInvalidArgument("organizationId is required"))
			return
		}

		// The body names the tenant to lock down; the authenticated scope is the
		// authority, so the payload cannot target another organization.
		organizationID, err := trustedOrgID(r, req.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.ForceLogoutOrganization(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
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

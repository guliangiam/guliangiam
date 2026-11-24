package users

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerSessionsRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/users/{id}/sessions", func(w http.ResponseWriter, r *http.Request) {
		page := route.QueryInt(r, "page", 1)
		pageSize := route.QueryInt(r, "pageSize", route.DefaultPageSize)
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		items, total, err := handler.ListUserSessions(r.Context(), organizationID, chi.URLParam(r, "id"), page, pageSize)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, page, pageSize, total))
	})

	router.Delete("/api/v1/users/{id}/sessions/{sessionId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.RevokeUserSession(r.Context(), organizationID, chi.URLParam(r, "id"), chi.URLParam(r, "sessionId")); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("revoked", "resource revoked"))
	})

	router.Delete("/api/v1/users/{id}/sessions", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.ForceLogoutUserSessions(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, map[string]any{
			"status":       "revoked",
			"revokedCount": result.SessionCount,
			"refreshCount": result.RefreshCount,
			"accessCount":  result.AccessCount,
		})
	})
	router.Get("/api/v1/users/{id}/transactions", func(w http.ResponseWriter, r *http.Request) {
		page := route.QueryInt(r, "page", 1)
		pageSize := route.QueryInt(r, "pageSize", route.DefaultPageSize)
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		items, total, err := handler.ListUserTransactions(r.Context(), organizationID, chi.URLParam(r, "id"), page, pageSize)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, page, pageSize, total))
	})

	// User consents sub-resource: list consents for a user.
	router.Get("/api/v1/users/{id}/consents", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.ListUserConsents(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// User consents sub-resource: revoke a consent for a user.
	router.Delete("/api/v1/users/{id}/consents/{consentId}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.RevokeConsent(r.Context(), organizationID, chi.URLParam(r, "id"), chi.URLParam(r, "consentId")); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("revoked", "resource revoked"))
	})

	// User count endpoint lives in chi_import.go; registering it here as well
	// silently replaced that trusted handler at runtime.

	// Remove user from group.
	router.Post("/api/v1/users/{id}/remove-from-group", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req struct {
			GroupID string `json:"groupId"`
		}
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.RemoveUserFromGroup(r.Context(), organizationID, chi.URLParam(r, "id"), req.GroupID); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "operation succeeded"))
	})

	// Get masked contact info for password reset flow. This is PII, so the
	// route resolves the trusted tenant before the lookup even though sessions
	// and consents in this file stay on the next batch.
	router.Get("/api/v1/users/{id}/masked-contact", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.GetMaskedContact(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})

	// User orders sub-resource: list orders for a user.
	router.Get("/api/v1/users/{id}/orders", func(w http.ResponseWriter, r *http.Request) {
		page := route.QueryInt(r, "page", 1)
		pageSize := route.QueryInt(r, "pageSize", route.DefaultPageSize)
		orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.ListUserOrders(r.Context(), orgID, chi.URLParam(r, "id"), page, pageSize)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	// User payments sub-resource: list payments for a user.
	router.Get("/api/v1/users/{id}/payments", func(w http.ResponseWriter, r *http.Request) {
		page := route.QueryInt(r, "page", 1)
		pageSize := route.QueryInt(r, "pageSize", route.DefaultPageSize)
		orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.ListUserPayments(r.Context(), orgID, chi.URLParam(r, "id"), page, pageSize)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})
}

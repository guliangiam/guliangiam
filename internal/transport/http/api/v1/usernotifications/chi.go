package usernotifications

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/me/notifications", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var isRead *bool
		if raw := strings.TrimSpace(r.URL.Query().Get("isRead")); raw != "" {
			value := raw == "true" || raw == "1"
			isRead = &value
		}
		response, err := handler.List(r.Context(), sc, ListRequest{
			Category: r.URL.Query().Get("category"),
			IsRead:   isRead,
			Keyword:  r.URL.Query().Get("keyword"),
			Page:     route.QueryInt(r, "page", 1),
			PageSize: route.QueryInt(r, "pageSize", route.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/me/notifications/unread-count", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		count, err := handler.UnreadCount(r.Context(), sc)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, UnreadCountResponse{Count: count})
	})

	router.Get("/api/v1/me/notifications/{id}", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Get(r.Context(), sc, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/me/notifications/{id}/read", func(w http.ResponseWriter, r *http.Request) {
		markRead(w, r, handler, true)
	})

	router.Post("/api/v1/me/notifications/{id}/unread", func(w http.ResponseWriter, r *http.Request) {
		markRead(w, r, handler, false)
	})

	router.Post("/api/v1/me/notifications/read-all", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		count, err := handler.MarkAllRead(r.Context(), sc, r.URL.Query().Get("category"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponseWithData("ok", "marked read", map[string]any{"count": count}))
	})

	router.Delete("/api/v1/me/notifications/{id}", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Delete(r.Context(), sc, chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	router.Post("/api/v1/me/notifications/batch-delete", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request BatchDeleteRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		count, err := handler.BatchDelete(r.Context(), sc, request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponseWithData("ok", "deleted", map[string]any{"count": count}))
	})

	router.Get("/api/v1/me/notifications/{id}/replies", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.ListReplies(r.Context(), sc, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"items": response})
	})

	router.Post("/api/v1/me/notifications/{id}/replies", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request ReplyRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.CreateReply(r.Context(), sc, chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, response)
	})

	router.Get("/api/v1/me/notification-preferences", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.ListPreferences(r.Context(), sc)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"items": response})
	})

	router.Put("/api/v1/me/notification-preferences", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request PreferencesRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.UpdatePreferences(r.Context(), sc, request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"items": response})
	})

	router.Get("/api/v1/me/notification-channel-bindings", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.ListChannelBindings(r.Context(), sc)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"items": response})
	})

	router.Put("/api/v1/me/notification-channel-bindings", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request ChannelBindingRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.UpsertChannelBinding(r.Context(), sc, request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Delete("/api/v1/me/notification-channel-bindings/{id}", func(w http.ResponseWriter, r *http.Request) {
		sc, err := requestScope(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.DeleteChannelBinding(r.Context(), sc, chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func markRead(w http.ResponseWriter, r *http.Request, handler Handler, read bool) {
	sc, err := requestScope(r)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	if err := handler.MarkRead(r.Context(), sc, chi.URLParam(r, "id"), read); err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("ok", "updated"))
}

func requestScope(r *http.Request) (scope, error) {
	session, ok := authtransport.SessionContextFromContext(r.Context())
	if !ok || strings.TrimSpace(session.UserID) == "" {
		return scope{}, apperrors.NewUnauthorized("authentication required")
	}
	organizationID := authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	if strings.TrimSpace(organizationID) == "" {
		return scope{}, apperrors.NewInvalidArgument("organizationId is required")
	}
	return scope{OrganizationID: organizationID, UserID: session.UserID}, nil
}

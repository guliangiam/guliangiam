package actions

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes wires the six actions endpoints. Thin adapters only —
// no business logic lives here.
func RegisterChiRoutes(router chi.Router, handler Handler) {
	// Query organizationId is only a narrowing hint for platform administrators;
	// a regular caller's trusted tenant always comes from the middleware scope.
	orgID := func(r *http.Request) string {
		return auth.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	}

	// GET /api/v1/actions
	router.Get("/api/v1/actions", func(w http.ResponseWriter, r *http.Request) {
		resp, total, err := handler.ListActions(r.Context(), ListActionsQuery{
			OrganizationID: orgID(r),
			TriggerType:    queryStringPtr(r, "triggerType"),
			Status:         queryStringPtr(r, "status"),
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(resp, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})

	// POST /api/v1/actions
	router.Post("/api/v1/actions", func(w http.ResponseWriter, r *http.Request) {
		var req CreateActionRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.CreateAction(r.Context(), orgID(r), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteCreated(w, "/api/v1/actions/"+resp.ID, resp)
	})

	// GET /api/v1/actions/{id}
	router.Get("/api/v1/actions/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler.GetAction(r.Context(), orgID(r), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	// PATCH /api/v1/actions/{id}:status  (status transition)
	router.Patch("/api/v1/actions/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var req UpdateStatusRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.UpdateActionStatus(r.Context(), orgID(r), chi.URLParam(r, "id"), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	// GET /api/v1/actions/{actionId}/versions
	router.Get("/api/v1/actions/{actionId}/versions", func(w http.ResponseWriter, r *http.Request) {
		resp, total, err := handler.ListVersions(r.Context(), ListVersionsQuery{
			OrganizationID: orgID(r),
			ActionID:       chi.URLParam(r, "actionId"),
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(resp, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})

	// POST /api/v1/actions/{actionId}:versions  (publish new version)
	router.Post("/api/v1/actions/{actionId}/versions", func(w http.ResponseWriter, r *http.Request) {
		var req PublishVersionRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.PublishVersion(r.Context(), orgID(r), chi.URLParam(r, "actionId"), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, resp)
	})

	// GET /api/v1/actions/{actionId}/execution-logs
	router.Get("/api/v1/actions/{actionId}/execution-logs", func(w http.ResponseWriter, r *http.Request) {
		resp, total, err := handler.ListExecutionLogs(r.Context(), ListExecLogsQuery{
			OrganizationID: orgID(r),
			ActionID:       chi.URLParam(r, "actionId"),
			Status:         queryStringPtr(r, "status"),
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(resp, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})

	// POST /api/v1/actions/{actionId}:run
	router.Post("/api/v1/actions/{actionId}/run", func(w http.ResponseWriter, r *http.Request) {
		var req RunActionRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.RunAction(r.Context(), orgID(r), chi.URLParam(r, "actionId"), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusAccepted, resp)
	})
}

func queryStringPtr(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

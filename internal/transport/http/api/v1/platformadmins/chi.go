package platformadmins

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	appplatformadmins "github.com/guliangiam/guliangiam/internal/app/platform_admins"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes 注册平台管理员管理面路由（管理面分组，PEP + ScopeAuthorizer
// 已在上游 middleware 链生效；未授权角色默认拒绝）。
func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/platform-admins", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.List(r.Context(), listQuery(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/platform-admins", func(w http.ResponseWriter, r *http.Request) {
		var request CreatePlatformAdminRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Create(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, response)
	})

	router.Get("/api/v1/platform-admins/{id}", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.Get(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Patch("/api/v1/platform-admins/{id}", func(w http.ResponseWriter, r *http.Request) {
		var request PatchPlatformAdminRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Patch(r.Context(), chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Delete("/api/v1/platform-admins/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := handler.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	router.Post("/api/v1/platform-admins/{id}:set-active", func(w http.ResponseWriter, r *http.Request) {
		var request SetActiveRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.SetActive(r.Context(), chi.URLParam(r, "id"), request.Active); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})

	router.Get("/api/v1/platform-admins/{id}/scopes", func(w http.ResponseWriter, r *http.Request) {
		onlyActive := false
		if flag := route.QueryOptionalBool(r, "onlyActive"); flag != nil {
			onlyActive = *flag
		}
		response, err := handler.ListScopes(r.Context(), chi.URLParam(r, "id"), onlyActive)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/platform-admins/{id}/scopes", func(w http.ResponseWriter, r *http.Request) {
		var request CreateScopeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.CreateScope(r.Context(), chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, response)
	})

	router.Get("/api/v1/platform-admin-scopes/{scopeId}", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.GetScope(r.Context(), chi.URLParam(r, "scopeId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Delete("/api/v1/platform-admin-scopes/{scopeId}", func(w http.ResponseWriter, r *http.Request) {
		if err := handler.DeleteScope(r.Context(), chi.URLParam(r, "scopeId")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	router.Post("/api/v1/platform-admin-scopes/{scopeId}:approve", func(w http.ResponseWriter, r *http.Request) {
		var request ApproveScopeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.ApproveScope(r.Context(), chi.URLParam(r, "scopeId"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("approved", "scope approved"))
	})
}

func listQuery(r *http.Request) appplatformadmins.ListPlatformAdminsQuery {
	return appplatformadmins.ListPlatformAdminsQuery{
		Page:     route.QueryInt(r, "page", 1),
		PageSize: route.QueryInt(r, "pageSize", route.DefaultPageSize),
		Active:   route.QueryOptionalBool(r, "active"),
		Keyword:  r.URL.Query().Get("keyword"),
	}
}

package users

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func registerCRUDRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request := ListUsersRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: organizationID,
			Keyword:        r.URL.Query().Get("keyword"),
			Status:         r.URL.Query().Get("status"),
			UserType:       r.URL.Query().Get("userType"),
			SortField:      r.URL.Query().Get("sortField"),
			SortOrder:      r.URL.Query().Get("sortOrder"),
			SearchField:    r.URL.Query().Get("searchField"),
			SearchValue:    r.URL.Query().Get("searchValue"),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		var request CreateUserRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		// The body's organizationId is request input, not the tenant authority:
		// a regular caller is pinned to the authenticated scope, while a platform
		// administrator keeps the explicit cross-tenant target.
		organizationID, err := trustedOrgID(r, request.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID

		response, err := handler.Create(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteCreated(w, "/api/v1/users/"+response.ID, response)
	})

	router.Get("/api/v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		// Detail reads carry masked credential fields and must not be replayed
		// from a browser cache after a save; declaring no-store here also makes
		// the ETag middleware skip its `private, no-cache` branch.
		w.Header().Set("Cache-Control", "no-store")
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Get(r.Context(), GetUserRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Patch("/api/v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request PatchUserRequest
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

	router.Delete("/api/v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Delete(r.Context(), GetUserRequest{
			ID:             chi.URLParam(r, "id"),
			OrganizationID: organizationID,
		}); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})
}

package models

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/models", func(w http.ResponseWriter, r *http.Request) {
		req := ListModelsRequest{
			Page: route.QueryInt(r, "page", 1), PageSize: route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId")), Keyword: r.URL.Query().Get("keyword"),
			IsEnabled: route.QueryOptionalBool(r, "isEnabled"),
		}
		resp, err := handler.List(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Post("/api/v1/models", func(w http.ResponseWriter, r *http.Request) {
		var req CreateModelRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// Tenant comes from the authenticated scope, never from the payload.
		req.OrganizationID = authtransport.TrustedOrganizationID(r, req.OrganizationID)
		if err := handler.Create(r.Context(), req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, presenter.NewActionResponse("created", "resource created"))
	})
	router.Get("/api/v1/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.Get(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Patch("/api/v1/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req PatchModelRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Patch(r.Context(), organizationID, chi.URLParam(r, "id"), req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})
	router.Delete("/api/v1/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
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

package rebac

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes wires all ReBAC endpoints (CRUD + ADR-024 Evaluator).
func RegisterChiRoutes(router chi.Router, h Handler) {
	// ─── Namespace ───
	router.Get("/api/v1/rebac/namespaces", func(w http.ResponseWriter, r *http.Request) {
		resp, err := h.ListNamespaces(r.Context(), ListNamespacesRequest{
			OrganizationID: auth.TrustedOrganizationID(r, ""),
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	router.Post("/api/v1/rebac/namespaces", func(w http.ResponseWriter, r *http.Request) {
		var req CreateNamespaceRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// Tenant comes from the authenticated scope; a platform admin may name an
		// explicit target tenant, a regular user's payload value is ignored.
		req.OrganizationID = auth.TrustedOrganizationID(r, req.OrganizationID)
		resp, err := h.CreateNamespace(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		// The app use case only returns after a successful id generation; a
		// created namespace always has a canonical path, so Location is
		// unconditional here (a regression must surface as a test failure,
		// not silently disappear from the response headers).
		location := "/api/v1/rebac/namespaces/" + resp.Namespace.ID
		route.WriteCreated(w, location, resp)
	})

	router.Get("/api/v1/rebac/namespaces/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := h.GetNamespace(r.Context(), chi.URLParam(r, "id"), auth.TrustedOrganizationID(r, ""))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	router.Delete("/api/v1/rebac/namespaces/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := h.DeleteNamespace(r.Context(), chi.URLParam(r, "id"), auth.TrustedOrganizationID(r, "")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// ─── Tuple ───
	router.Get("/api/v1/rebac/namespaces/{namespaceId}/tuples", func(w http.ResponseWriter, r *http.Request) {
		orgID := auth.TrustedOrganizationID(r, "")
		nsID := chi.URLParam(r, "namespaceId")
		page, ps := route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize)
		items, total, err := h.ListTuples(r.Context(), orgID, nsID, page, ps)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, page, ps, int64(total)))
	})

	router.Post("/api/v1/rebac/namespaces/{namespaceId}/tuples", func(w http.ResponseWriter, r *http.Request) {
		var req CreateTupleRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := h.CreateTuple(r.Context(), auth.TrustedOrganizationID(r, ""), chi.URLParam(r, "namespaceId"), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, resp)
	})

	router.Delete("/api/v1/rebac/tuples/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := h.DeleteTuple(r.Context(), chi.URLParam(r, "id"), auth.TrustedOrganizationID(r, "")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// ─── ADR-024 Evaluator ───
	router.Post("/api/v1/rebac/namespaces/{namespaceId}/check", func(w http.ResponseWriter, r *http.Request) {
		var req CheckRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		req.OrganizationID = auth.TrustedOrganizationID(r, "")
		req.NamespaceID = chi.URLParam(r, "namespaceId")
		resp, err := h.Check(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	router.Post("/api/v1/rebac/namespaces/{namespaceId}/expand", func(w http.ResponseWriter, r *http.Request) {
		var req ExpandRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		req.OrganizationID = auth.TrustedOrganizationID(r, "")
		req.NamespaceID = chi.URLParam(r, "namespaceId")
		resp, err := h.Expand(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	router.Post("/api/v1/rebac/namespaces/{namespaceId}/batch-check", func(w http.ResponseWriter, r *http.Request) {
		var req BatchCheckRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		req.OrganizationID = auth.TrustedOrganizationID(r, "")
		req.NamespaceID = chi.URLParam(r, "namespaceId")
		resp, err := h.BatchCheck(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
}

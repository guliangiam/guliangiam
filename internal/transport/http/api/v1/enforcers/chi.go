package enforcers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// multipartMemoryBytes is ParseMultipartForm's in-memory threshold, **not** a
// request-body cap. The body cap is the bootstrap http.MaxBytesReader wall,
// which lets this import path use STORAGE_UPLOAD_MAX_BYTES (32 MiB default,
// 手册 4.17.2); larger bodies stream to temporary files.
const multipartMemoryBytes = 32 << 20

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/enforcers", func(w http.ResponseWriter, r *http.Request) {
		req := ListEnforcersRequest{
			Page: route.QueryInt(r, "page", 1), PageSize: route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: auth.TrustedOrganizationID(r, r.URL.Query().Get("organizationId")), Keyword: r.URL.Query().Get("keyword"),
		}
		if v := r.URL.Query().Get("isEnabled"); v != "" {
			if parsed, err := strconv.ParseBool(v); err == nil {
				req.IsEnabled = &parsed
			}
		}
		resp, err := handler.List(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Post("/api/v1/enforcers", func(w http.ResponseWriter, r *http.Request) {
		var req CreateEnforcerRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// Tenant comes from the authenticated scope, never from the payload.
		req.OrganizationID = auth.TrustedOrganizationID(r, req.OrganizationID)
		if err := handler.Create(r.Context(), req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, presenter.NewActionResponse("created", "resource created"))
	})
	router.Get("/api/v1/enforcers/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.Get(r.Context(), organizationID, GetEnforcerRequest{ID: chi.URLParam(r, "id")})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Patch("/api/v1/enforcers/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req PatchEnforcerRequest
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
	router.Delete("/api/v1/enforcers/{id}", func(w http.ResponseWriter, r *http.Request) {
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
	router.Post("/api/v1/enforcers/{id}/enforce", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req EnforceRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.Enforce(r.Context(), organizationID, chi.URLParam(r, "id"), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/api/v1/enforcers/{id}/batch-enforce", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req BatchEnforceRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.BatchEnforce(r.Context(), organizationID, chi.URLParam(r, "id"), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/api/v1/enforcers/{id}/policies", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req PolicyRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.AddPolicy(r.Context(), organizationID, chi.URLParam(r, "id"), req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, presenter.NewActionResponse("created", "resource created"))
	})
	router.Delete("/api/v1/enforcers/{id}/policies", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req PolicyRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.RemovePolicy(r.Context(), organizationID, chi.URLParam(r, "id"), req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})
	router.Get("/api/v1/enforcers/{id}/subjects", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.GetAllSubjects(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Get("/api/v1/enforcers/{id}/objects", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.GetAllObjects(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Get("/api/v1/enforcers/{id}/actions", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.GetAllActions(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Get("/api/v1/enforcers/{id}/roles", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.GetAllRoles(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Post("/api/v1/enforcers/{id}/reload", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.ReloadPolicy(r.Context(), organizationID, chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("reloaded", "configuration reloaded"))
	})
	router.Get("/api/v1/enforcers/{id}/policies", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		id := chi.URLParam(r, "id")
		fieldIndex := route.QueryInt(r, "fieldIndex", 0)
		fieldValues := r.URL.Query()["fieldValue"]
		var result [][]string
		if len(fieldValues) > 0 {
			result, err = handler.GetFilteredPolicies(r.Context(), organizationID, id, fieldIndex, fieldValues)
		} else {
			result, err = handler.GetPolicies(r.Context(), organizationID, id)
		}
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})
	router.Put("/api/v1/enforcers/{id}/policies", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req UpdatePolicyRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.UpdatePolicy(r.Context(), organizationID, chi.URLParam(r, "id"), req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("updated", "resource updated"))
	})
	router.Post("/api/v1/enforcers/-/refresh-engines", func(w http.ResponseWriter, r *http.Request) {
		if err := handler.RefreshEngines(r.Context()); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("refreshed", "resource refreshed"))
	})

	// Policy import: replace all policies with the provided list.
	router.Post("/api/v1/enforcers/{id}/import-policies", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req struct {
			Policies [][]string `json:"policies"`
		}
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.ImportPolicies(r.Context(), organizationID, chi.URLParam(r, "id"), req.Policies); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"status": "imported", "msg": "policies imported", "count": len(req.Policies)})
	})

	// Policy export: return all policies for the enforcer.
	router.Get("/api/v1/enforcers/{id}/export-policies", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.ExportPolicies(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, result)
	})

	// Policy Excel upload: replace all policies with those parsed from an Excel file.
	router.Post("/api/v1/enforcers/{id}/upload-policies", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := r.ParseMultipartForm(multipartMemoryBytes); err != nil {
			route.WriteError(w, r, err)
			return
		}
		defer r.MultipartForm.RemoveAll()

		file, _, err := r.FormFile("file")
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		defer file.Close()

		result, err := handler.UploadPoliciesExcel(r.Context(), organizationID, chi.URLParam(r, "id"), file)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
}

// trustedOrgID resolves the tenant a request may act on. A caller-supplied
// organizationId is honoured only for platform administrators; otherwise the
// authenticated tenant scope wins. An empty result is a client error: on
// dialects without RLS an empty organization_id degrades to "all tenants",
// and casbin_rules itself has neither an organization column nor RLS.
func trustedOrgID(r *http.Request, requested string) (string, error) {
	orgID := auth.TrustedOrganizationID(r, requested)
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return orgID, nil
}

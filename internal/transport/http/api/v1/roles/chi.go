package roles

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// multipartMemoryBytes is ParseMultipartForm's in-memory threshold, **not** a
// request-body cap. The body cap is the bootstrap http.MaxBytesReader wall,
// which lets this import path use STORAGE_UPLOAD_MAX_BYTES (32 MiB default,
// 手册 4.17.2); larger bodies stream to temporary files.
const multipartMemoryBytes = 32 << 20

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/roles", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		request := ListRolesRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: organizationID,
			Keyword:        r.URL.Query().Get("keyword"),
			IsEnabled:      route.QueryOptionalBool(r, "isEnabled"),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/roles", func(w http.ResponseWriter, r *http.Request) {
		var request CreateRoleRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// The body's organizationId only requests a target tenant; the
		// authenticated scope decides which tenant is actually written.
		organizationID, err := trustedOrgID(r, request.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID

		if err := handler.Create(r.Context(), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusCreated, presenter.NewActionResponse("created", "resource created"))
	})

	router.Get("/api/v1/roles/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		response, err := handler.Get(r.Context(), organizationID, GetRoleRequest{ID: chi.URLParam(r, "id")})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Patch("/api/v1/roles/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request PatchRoleRequest
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

	router.Delete("/api/v1/roles/{id}", func(w http.ResponseWriter, r *http.Request) {
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

	// Bulk import roles from JSON payload.
	router.Post("/api/v1/roles/bulk-import", func(w http.ResponseWriter, r *http.Request) {
		var request BulkImportRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		organizationID, err := trustedOrgID(r, request.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID
		// Per-row organizationId is request input: the whole batch stays inside
		// the trusted tenant resolved above.
		for i := range request.Roles {
			request.Roles[i].OrganizationID = organizationID
		}

		result, err := handler.BulkImport(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})

	// Upload roles from Excel file (multipart/form-data).
	router.Post("/api/v1/roles/upload", func(w http.ResponseWriter, r *http.Request) {
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

		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		result, err := handler.UploadExcel(r.Context(), file, organizationID)
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
// dialects without RLS an empty organization_id degrades to "all tenants".
func trustedOrgID(r *http.Request, requested string) (string, error) {
	orgID := authtransport.TrustedOrganizationID(r, requested)
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	return orgID, nil
}

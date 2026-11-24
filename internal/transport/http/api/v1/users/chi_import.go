package users

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// multipartMemoryBytes is ParseMultipartForm's in-memory threshold, **not** a
// request-body cap. The body cap is the bootstrap http.MaxBytesReader wall,
// which lets these three import paths use STORAGE_UPLOAD_MAX_BYTES (32 MiB
// default, 手册 4.17.2); larger bodies stream to temporary files. The earlier
// bare 32<<20 here looked like a 32 MiB body limit but the default 1 MiB wall
// rejected anything above 1 MiB before parsing ever saw it (E1 E-03).
const multipartMemoryBytes = 32 << 20

func registerImportRoutes(router chi.Router, handler Handler) {
	router.Post("/api/v1/users/import", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		var request ImportUsersRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		// JSON import used to let every row pick its own tenant. The request's
		// trusted scope is the authority, so rows cannot escape it.
		for i := range request.Users {
			request.Users[i].OrganizationID = organizationID
		}

		response, err := handler.BulkImport(r.Context(), organizationID, request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/users/export", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		file, err := handler.ExportExcel(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename=users.xlsx")
		if err := file.Write(w); err != nil {
			route.WriteError(w, r, err)
		}
	})

	// Upload users from Excel file (multipart/form-data).
	router.Post("/api/v1/users/upload", func(w http.ResponseWriter, r *http.Request) {
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

	// Preview users from Excel file without importing (multipart/form-data).
	router.Post("/api/v1/users/upload/preview", func(w http.ResponseWriter, r *http.Request) {
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

		result, err := handler.PreviewExcel(r.Context(), file, organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})

	// Upload users from CSV file with field mapping (multipart/form-data).
	router.Post("/api/v1/users/upload-csv", func(w http.ResponseWriter, r *http.Request) {
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

		// Parse optional field mapping from form field.
		fieldMapping := make(map[string]string)
		if mappingJSON := r.FormValue("fieldMapping"); mappingJSON != "" {
			if err := json.Unmarshal([]byte(mappingJSON), &fieldMapping); err != nil {
				route.WriteError(w, r, apperrors.WrapInternalf(err, "invalid fieldMapping JSON"))
				return
			}
		}

		result, err := handler.UploadCSV(r.Context(), file, organizationID, fieldMapping)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, result)
	})
	// User count endpoint.
	router.Get("/api/v1/users/count", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		count, err := handler.GetUserCount(r.Context(), organizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, map[string]any{"count": count})
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

package users

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// maxIdentityDocumentBytes is the product limit for one uploaded document
// image. ParseMultipartForm's in-memory threshold is intentionally larger so the
// explicit check below produces a clear 400 instead of a generic parse error.
const maxIdentityDocumentBytes = 5 << 20

func registerIdentityDocumentRoutes(router chi.Router, handler Handler) {
	router.Post("/api/v1/users/{id}/identity-documents/recognize", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := r.ParseMultipartForm(maxIdentityDocumentBytes + (1 << 20)); err != nil {
			route.WriteError(w, r, apperrors.NewInvalidArgument("request must be multipart/form-data with a file field"))
			return
		}
		defer r.MultipartForm.RemoveAll()

		file, header, err := r.FormFile("file")
		if err != nil {
			route.WriteError(w, r, apperrors.NewInvalidArgument("file is required"))
			return
		}
		defer file.Close()

		data, err := io.ReadAll(io.LimitReader(file, maxIdentityDocumentBytes+1))
		if err != nil {
			route.WriteError(w, r, apperrors.NewInvalidArgument("uploaded file could not be read"))
			return
		}
		if len(data) == 0 {
			route.WriteError(w, r, apperrors.NewInvalidArgument("uploaded file is empty"))
			return
		}
		if len(data) > maxIdentityDocumentBytes {
			route.WriteError(w, r, apperrors.NewInvalidArgument("image exceeds the 5MiB limit"))
			return
		}

		mediaType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
		if mediaType == "" || mediaType == "application/octet-stream" {
			mediaType = http.DetectContentType(data)
		}

		fields, err := handler.RecognizeIdentityDocument(
			r.Context(),
			organizationID,
			chi.URLParam(r, "id"),
			r.FormValue("documentType"),
			r.FormValue("side"),
			mediaType,
			data,
		)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, fields)
	})
}

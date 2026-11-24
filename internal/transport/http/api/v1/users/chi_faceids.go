package users

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// maxFaceImageBytes is the product limit for one captured face image, mirroring
// the identity-document upload cap.
const maxFaceImageBytes = 5 << 20

// FaceIDItem is the transport projection of one enrolled face template.
type FaceIDItem struct {
	ID        string `json:"id"`
	ImageUrl  string `json:"imageUrl"`
	CreatedAt string `json:"createdAt"`
}

// parseFaceImageUpload reads and bounds-checks the multipart "file" field.
// Shared by the admin (/users/{id}/faceids) and self-service (/auth/faceids)
// enrollment routes so both faces enforce the identical upload contract.
func parseFaceImageUpload(r *http.Request) (mediaType string, data []byte, err error) {
	if err := r.ParseMultipartForm(maxFaceImageBytes + (1 << 20)); err != nil {
		return "", nil, apperrors.NewInvalidArgument("request must be multipart/form-data with a file field")
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		return "", nil, apperrors.NewInvalidArgument("file is required")
	}
	defer file.Close()

	data, err = io.ReadAll(io.LimitReader(file, maxFaceImageBytes+1))
	if err != nil {
		return "", nil, apperrors.NewInvalidArgument("uploaded file could not be read")
	}
	if len(data) == 0 {
		return "", nil, apperrors.NewInvalidArgument("uploaded file is empty")
	}
	if len(data) > maxFaceImageBytes {
		return "", nil, apperrors.NewInvalidArgument("image exceeds the 5MiB limit")
	}

	mediaType = strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = http.DetectContentType(data)
	}
	return mediaType, data, nil
}

func registerFaceIDRoutes(router chi.Router, handler Handler) {
	// Enroll a face template: the image is liveness-checked by the configured
	// FaceID provider before it is stored and appended to the user's face ids.
	router.Post("/api/v1/users/{id}/faceids", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		mediaType, data, err := parseFaceImageUpload(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		faceID, err := handler.EnrollFaceID(
			r.Context(),
			organizationID,
			chi.URLParam(r, "id"),
			r.FormValue("providerId"),
			r.FormValue("deviceInfo"),
			mediaType,
			data,
		)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, faceID)
	})

	// Revoke the active face template: blocks face-id login and clears the legacy
	// face_ids marker in one audited transaction. Idempotent at the route layer;
	// the app returns 404 when no active template exists.
	router.Delete("/api/v1/users/{id}/faceids", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.RevokeFaceID(r.Context(), organizationID, chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

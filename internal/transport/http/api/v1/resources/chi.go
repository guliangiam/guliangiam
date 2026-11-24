package resources

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// uploadSniffBytes is how many leading bytes http.DetectContentType inspects
// (手册 4.17.2 ① 魔数嗅探).
const uploadSniffBytes = 512

// uploadParseError maps a multipart parse failure to the upload contract's
// status: a body over STORAGE_UPLOAD_MAX_BYTES is 413 Content Too Large
// (手册 4.17.2), anything else is a malformed request (400).
func uploadParseError(err error, maxBytes int64) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return apperrors.NewRequestEntityTooLargef(
			"upload exceeds the %d byte limit (STORAGE_UPLOAD_MAX_BYTES)", maxBytes)
	}
	return apperrors.WrapInvalidArgument(err, "parse multipart upload")
}

// verifyUploadMime enforces the 手册 4.17.2 MIME triple check: the client-declared
// type, the magic-number sniff and the extension-bound type must each pass
// UPLOAD_ALLOWED_MIME_TYPES and agree (see config.UploadMimeConsistent for the
// one bitmap relaxation the bundled crop widget requires).
func verifyUploadMime(declared, sniffed, filename string) error {
	ext := filepath.Ext(filename)
	extMime := platformconfig.UploadMimeTypeForExtension(ext)

	if !platformconfig.IsAllowedUploadMimeType(declared) {
		return apperrors.NewUnprocessableEntityf(
			"unsupported upload media type %q; allowed types are configured via UPLOAD_ALLOWED_MIME_TYPES", declared)
	}
	if !platformconfig.IsAllowedUploadMimeType(sniffed) {
		return apperrors.NewUnprocessableEntityf(
			"upload content does not sniff as an allowed media type (detected %q)", sniffed)
	}
	if !platformconfig.IsAllowedUploadExtension(ext) {
		return apperrors.NewUnprocessableEntityf(
			"file extension %q is not an allowed upload extension", ext)
	}
	if !platformconfig.UploadMimeConsistent(declared, sniffed, extMime) {
		return apperrors.NewUnprocessableEntityf(
			"upload media types disagree: declared %q, extension %q, sniffed %q",
			declared, extMime, sniffed)
	}
	return nil
}

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/resources", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request := ListResourcesRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: organizationID,
			ApplicationID:  r.URL.Query().Get("applicationId"),
			Keyword:        r.URL.Query().Get("keyword"),
			Type:           r.URL.Query().Get("type"),
			Tag:            r.URL.Query().Get("tag"),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/resources", func(w http.ResponseWriter, r *http.Request) {
		var request CreateResourceRequest
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

		if err := handler.Create(r.Context(), request); err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusCreated, presenter.NewActionResponse("created", "resource created"))
	})

	router.Get("/api/v1/resources/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Get(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Patch("/api/v1/resources/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request PatchResourceRequest
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

	router.Delete("/api/v1/resources/{id}", func(w http.ResponseWriter, r *http.Request) {
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

	// Upload a file resource (multipart form, 手册 4.17.2 / 5.5.3). The versioned
	// path is the contract form; the frontend ImageCropUpload component posts here.
	router.Post("/api/v1/upload-resource", func(w http.ResponseWriter, r *http.Request) {
		owner, err := trustedOrgID(r, r.URL.Query().Get("owner"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		maxBytes := platformconfig.UploadMaxBytes()

		// Fast path: a declared body over the cap is 413 before it is read.
		if r.ContentLength > maxBytes {
			route.WriteError(w, r, apperrors.NewRequestEntityTooLargef(
				"upload exceeds the %d byte limit (STORAGE_UPLOAD_MAX_BYTES)", maxBytes))
			return
		}

		// Parse multipart form; the in-memory ceiling tracks the single-file cap
		// from STORAGE_UPLOAD_MAX_BYTES (手册 4.17.2). The bootstrap hard wall wraps
		// the same cap around r.Body, so an over-cap stream surfaces here as
		// *http.MaxBytesError and is mapped to 413.
		if err := r.ParseMultipartForm(maxBytes); err != nil {
			route.WriteError(w, r, uploadParseError(err, maxBytes))
			return
		}
		defer r.MultipartForm.RemoveAll()

		file, header, err := r.FormFile("file")
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		defer file.Close()

		// ① 魔数嗅探: the first 512 bytes decide the real type, independent of what
		// the multipart part claims. The bytes are replayed to the store below.
		head := make([]byte, uploadSniffBytes)
		n, err := io.ReadFull(file, head)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			route.WriteError(w, r, apperrors.WrapInvalidArgument(err, "read upload header"))
			return
		}
		head = head[:n]
		sniffed := http.DetectContentType(head)
		declared := header.Header.Get("Content-Type")

		// 活跃内容默认拒绝 (手册 4.17.2): neither the declared nor the sniffed type
		// may be script-capable, whatever UPLOAD_ALLOWED_MIME_TYPES says.
		if platformconfig.IsActiveContentMimeType(declared) || platformconfig.IsActiveContentMimeType(sniffed) {
			route.WriteError(w, r, apperrors.NewUnprocessableEntityf(
				"active content is refused (declared %q, detected %q); SVG is accepted only after sanitisation and attachment-only delivery",
				declared, sniffed))
			return
		}

		// ②③ 扩展名白名单校验 + 魔数/扩展名/客户端声明三方比对.
		if err := verifyUploadMime(declared, sniffed, header.Filename); err != nil {
			route.WriteError(w, r, err)
			return
		}

		user := r.URL.Query().Get("user")
		tag := r.URL.Query().Get("tag")

		// Ignore legacy fullFilePath parameters; the application generates object keys.
		url, err := handler.Upload(r.Context(), UploadResourceCommand{
			Owner:    owner,
			User:     user,
			Tag:      tag,
			FileName: header.Filename,
			FileSize: header.Size,
			MimeType: declared,
			Reader:   io.MultiReader(bytes.NewReader(head), file),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, url)
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

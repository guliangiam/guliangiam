package system

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/app/system"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

type CLIHandler struct {
	service system.CLIService
}

func NewCLIHandler(service system.CLIService) CLIHandler {
	return CLIHandler{service: service}
}

func (h CLIHandler) DownloadCLI(w http.ResponseWriter, r *http.Request) {
	language := chi.URLParam(r, "language")
	if language == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("language path parameter is required"))
		return
	}

	platform := system.DetectPlatformFromUserAgent(r.UserAgent())

	artifact, err := h.service.DownloadCLI(r.Context(), language, platform)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	w.Header().Set("X-CLI-SHA256", artifact.SHA256)
	w.Header().Set("X-CLI-Filename", artifact.Filename)
	http.Redirect(w, r, artifact.DownloadURL, http.StatusFound)
}

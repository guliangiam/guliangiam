package reconciliation

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes mounts the reconciliation report read endpoint.
func RegisterChiRoutes(router chi.Router, handler *Handler) {
	router.Get("/api/v1/reconciliation/scim-reports", func(w http.ResponseWriter, r *http.Request) {
		request := ListRequest{
			Page:     route.QueryInt(r, "page", 1),
			PageSize: route.QueryInt(r, "pageSize", platformpagination.DefaultPageSize),
		}
		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})
}

func organizationIDFromContext(ctx context.Context) string {
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		return session.OrganizationID
	}
	return ""
}

func unavailableService() error {
	return apperrors.NewUnavailable("reconciliation reports are not available")
}

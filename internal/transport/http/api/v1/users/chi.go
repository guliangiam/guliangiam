package users

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes registers all /api/v1/users/* management routes.
func RegisterChiRoutes(router chi.Router, handler Handler) {
	registerCRUDRoutes(router, handler)
	registerCustomAttributeRoutes(router, handler)
	registerProfileRoutes(router, handler)
	registerMFARoutes(router, handler)
	registerRelationsRoutes(router, handler)
	registerAdminMFARoutes(router, handler)
	registerImportRoutes(router, handler)
	registerSessionsRoutes(router, handler)
	registerCollectionsRoutes(router, handler)
	registerContactVerificationRoutes(router, handler)
	registerIdentityDocumentRoutes(router, handler)
	registerFaceIDRoutes(router, handler)
	registerSelfServiceRoutes(router, handler)
}

// RegisterGlobalChiRoutes registers cross-organization user listing routes.
// Every route is gated on the verified platform administrator principal.
func RegisterGlobalChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/global/users", authtransport.RequirePlatformAdmin(func(w http.ResponseWriter, r *http.Request) {
		req := ListUsersRequest{
			Page:     route.QueryInt(r, "page", 1),
			PageSize: route.QueryInt(r, "pageSize", route.DefaultPageSize),
			Keyword:  r.URL.Query().Get("keyword"),
		}
		resp, err := handler.List(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	}))
}

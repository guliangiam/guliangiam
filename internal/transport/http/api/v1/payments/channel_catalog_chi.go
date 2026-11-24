package payments

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChannelCatalogChiRoutes registers the read-only payment channel
// capability catalog.
func RegisterChannelCatalogChiRoutes(router chi.Router, handler ChannelCatalogHandler) {
	router.Get("/api/v1/pay-channels", func(w http.ResponseWriter, r *http.Request) {
		route.WriteJSON(w, http.StatusOK, handler.List(r.Context(),
			route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", platformpagination.DefaultPageSize)))
	})
}

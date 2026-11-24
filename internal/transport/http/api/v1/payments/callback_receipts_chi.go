// callback_receipts_chi.go — callback receipts admin 查询路由。
package payments

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterCallbackReceiptsChiRoutes 注册 callback receipts 管理面路由。
// 与 RegisterReconChiRoutes / RegisterChannelConfigChiRoutes 同模式 ——
// 分离的 handler + chi 注册函数。
func RegisterCallbackReceiptsChiRoutes(router chi.Router, h CallbackReceiptsHandler) {
	router.Get("/api/v1/pay-callback-receipts", func(w http.ResponseWriter, r *http.Request) {
		page := route.QueryInt(r, "page", 1)
		pageSize := route.QueryInt(r, "pageSize", route.DefaultPageSize)
		orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := h.List(r.Context(), orgID, page, pageSize)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
}

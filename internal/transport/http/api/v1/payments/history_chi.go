// history_chi.go — Attempts + Refunds sub-resource HTTP 路由注册。
//
// 设计文档: G5 payments 生命周期查询
//
// 端点:
//   GET  /api/v1/payments/{paymentId}/attempts   指定 payment 的 attempts 时间线
//   GET  /api/v1/pay-attempts/{id}               单个 attempt 详情
//   GET  /api/v1/payments/{paymentId}/refunds    指定 payment 的退款列表
//   GET  /api/v1/pay-refunds/{id}                单个退款详情
//
// 与主 RegisterChiRoutes / RegisterReconChiRoutes / RegisterChannelConfigChiRoutes
// 分离注册 —— payments 模块 handler per-concern 模式。

package payments

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterAttemptsChiRoutes 注册 attempts 子资源路由。
func RegisterAttemptsChiRoutes(router chi.Router, handler AttemptsHandler) {
	// 指定 payment 的 attempts 时间线（用于 PaymentDetailPage 下方展示）
	router.Get("/api/v1/payments/{paymentId}/attempts", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.ListByPayment(r.Context(), organizationID, chi.URLParam(r, "paymentId"),
			route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", platformpagination.DefaultPageSize))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	// 单个 attempt 详情（原始响应 JSONB 调试）
	router.Get("/api/v1/pay-attempts/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.GetByID(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
}

// RegisterRefundsChiRoutes 注册 refunds 子资源路由。
func RegisterRefundsChiRoutes(router chi.Router, handler RefundsHandler) {
	// 指定 payment 的退款列表
	router.Get("/api/v1/payments/{paymentId}/refunds", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.ListByPayment(r.Context(), organizationID, chi.URLParam(r, "paymentId"),
			route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", platformpagination.DefaultPageSize))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	// 单个退款详情
	router.Get("/api/v1/pay-refunds/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.GetByID(r.Context(), organizationID, chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
}

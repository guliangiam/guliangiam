package payments

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterReconChiRoutes 注册对账模块 HTTP 路由 (design doc §8.2)。
// 与主 payments RegisterChiRoutes 分离——ReconHandler 依赖 domain.ReconRepo
// + ReconcileWorker，不走 apppayments.Service UOW 路径。
func RegisterReconChiRoutes(router chi.Router, handler ReconHandler) {
	// 对账批次列表
	router.Get("/api/v1/pay-recon-batches", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		req := ListReconBatchesRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: organizationID,
		}
		resp, err := handler.ListBatches(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	// 对账明细差异
	router.Get("/api/v1/pay-recon-batches/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		items, err := handler.ListItems(r.Context(), organizationID, ListReconItemsRequest{BatchID: chi.URLParam(r, "id")})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, items)
	})

	// 差异人工处置 (审批流入口，design doc §8.2)
	router.Post("/api/v1/pay-recon-items/{id}:resolve", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req ResolveReconItemRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		req.ItemID = chi.URLParam(r, "id")
		if err := handler.ResolveItem(r.Context(), organizationID, req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("resolved", "issue resolved"))
	})

	router.Post("/api/v1/pay-recon-items/{id}:approve", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req ApproveReconItemRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		req.ItemID = chi.URLParam(r, "id")
		if err := handler.ApproveItem(r.Context(), organizationID, req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("approved", "issue approved"))
	})

	// 手动触发对账 (两种模式: 指定 batchID 或新建 pending batch)
	router.Post("/api/v1/pay-recon:run", func(w http.ResponseWriter, r *http.Request) {
		var req RunReconRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// Both modes are tenant-scoped: mode 2 names the tenant in the body,
		// mode 1 names only a batch id. The authenticated scope is the authority
		// in either case, and app.RunManual re-checks batch ownership.
		organizationID, err := trustedOrgID(r, req.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		req.OrganizationID = organizationID
		batchID, err := handler.RunRecon(r.Context(), req)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusAccepted, map[string]any{
			"status":  "reconciled",
			"batchId": batchID,
		})
	})
}

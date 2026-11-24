// admin_chi.go — AI Gateway 管理面 chi 路由注册。
//
// 所有路由都带 /api/v1/ai 前缀（与设计文档 §管理面路由定义一致）。
// Casbin resourceFromPath 会把 /api/v1/ai/{entity} 推导为
// "aigateway:{entity}"（colon-separated），action 由 HTTP method 决定。
//
// 管理面路由必须挂在 protected 组下（已带 SessionContextMiddleware +
// APIAuthorizationMiddleware），绝不可以像数据平面那样挂到公开根路由。
package aigateway

import (
	"github.com/go-chi/chi/v5"
)

// RegisterAdminChiRoutes 把 AI Gateway 管理面 CRUD 路由注册到给定的
// chi.Router（调用方应当传入 protected 组下的 management 子组）。
func RegisterAdminChiRoutes(router chi.Router, h AdminHandler) {
	router.Route("/api/v1/ai", func(r chi.Router) {
		// ── Providers ───────────────────────────────────────────────────
		r.Get("/providers", h.handleListProviders)
		r.Post("/providers", h.handleCreateProvider)
		r.Get("/providers/{id}", h.handleGetProvider)
		r.Patch("/providers/{id}", h.handleUpdateProvider)
		r.Delete("/providers/{id}", h.handleDeleteProvider)
		r.Post("/providers/{id}/test", h.handleTestProvider)

		// ── Models ─────────────────────────────────────────────────────
		r.Get("/models", h.handleListModels)
		r.Post("/models", h.handleCreateModel)
		r.Get("/models/{id}", h.handleGetModel)
		r.Patch("/models/{id}", h.handleUpdateModel)
		r.Delete("/models/{id}", h.handleDeleteModel)

		// ── Aliases ────────────────────────────────────────────────────
		r.Get("/aliases", h.handleListAliases)
		r.Post("/aliases", h.handleCreateAlias)
		r.Get("/aliases/{id}", h.handleGetAlias)
		r.Patch("/aliases/{id}", h.handleUpdateAlias)
		r.Delete("/aliases/{id}", h.handleDeleteAlias)

		// ── Routes (NEW — binding aliases → models → providers) ────────
		r.Get("/routes", h.handleListRoutes)
		r.Post("/routes", h.handleCreateRoute)
		r.Get("/routes/{id}", h.handleGetRoute)
		r.Patch("/routes/{id}", h.handleUpdateRoute)
		r.Delete("/routes/{id}", h.handleDeleteRoute)

		// ── API Keys ───────────────────────────────────────────────────
		r.Get("/api-keys", h.handleListAPIKeys)
		r.Post("/api-keys", h.handleCreateAPIKey)
		r.Get("/api-keys/{id}", h.handleGetAPIKey)
		r.Patch("/api-keys/{id}", h.handleUpdateAPIKey)
		r.Delete("/api-keys/{id}", h.handleDeleteAPIKey)
		r.Post("/api-keys/{id}/revoke", h.handleRevokeAPIKey)

		// ── Quota Policies (NEW — RPM/TPM/cost limits) ─────────────────
		r.Get("/quotas", h.handleListQuotas)
		r.Post("/quotas", h.handleCreateQuota)
		r.Get("/quotas/{id}", h.handleGetQuota)
		r.Patch("/quotas/{id}", h.handleUpdateQuota)
		r.Delete("/quotas/{id}", h.handleDeleteQuota)

		// ── Usage (只读) ────────────────────────────────────────────────
		r.Get("/usage", h.handleListUsageLogs)
		r.Get("/usage/stats", h.handleGetUsageStats)

		// ── Price Versions ─────────────────────────────────────────────
		r.Get("/prices", h.handleListPrices)
		r.Post("/prices", h.handleCreatePrice)
		r.Get("/prices/{id}", h.handleGetPrice)
		r.Patch("/prices/{id}", h.handleUpdatePrice)
		r.Delete("/prices/{id}", h.handleDeletePrice)

		// ── Provider Health (只读 — 状态由 GatewayService 自动写) ──────
		r.Get("/health", h.handleListHealth)
		r.Get("/health/{id}", h.handleGetHealth)
	})
}

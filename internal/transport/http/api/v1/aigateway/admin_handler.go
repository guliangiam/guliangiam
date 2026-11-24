// admin_handler.go — AI Gateway 管理面 HTTP handlers。
//
// 严格遵守架构 Rule 2：Transport 层不 import domain 包。所有 DTO
// 与 Service 类型都来自 internal/app/aigateway（别名 appgateway）。
// OrgID 一律经 trustedOrgID 从可信租户作用域裁决，query 只作为平台管理员的
// 显式跨租户 target，普通租户的伪造值被忽略。
package aigateway

import (
	"net/http"
	"strconv"

	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"

	"github.com/go-chi/chi/v5"
)

// AdminHandler 把管理面的 CRUD 方法挂到 HTTP 路由上。
type AdminHandler struct {
	Service *appgateway.AdminService
}

// NewAdminHandler 构造 AdminHandler。
func NewAdminHandler(service *appgateway.AdminService) AdminHandler {
	return AdminHandler{Service: service}
}

// writeList answers a management list in the PaginatedResponse shape declared
// by openapi (items + page meta). Services return the whole set, so paging is
// applied here with the platform bounds: an omitted pageSize is served as
// DefaultPageSize, never as "one page with everything". The console has to send
// page/pageSize to walk the list; the previous whole-set default made every
// management response unbounded and disagreed with the rest of the API
// (手册 4.7.1).
func writeList[T any](w http.ResponseWriter, r *http.Request, items []T) {
	total := len(items)
	page, pageSize, err := listQueryParams(r, platformpagination.DefaultPageSize)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	page = platformpagination.ClampPage(page)
	pageSize = platformpagination.ClampPageSize(pageSize)
	start := platformpagination.Offset(page, pageSize)
	if start > total {
		start = total
	}
	end := start + min(pageSize, total-start)
	pageItems := items[start:end]
	if pageItems == nil {
		pageItems = make([]T, 0)
	}
	route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(pageItems, page, pageSize, int64(total)))
}

// listQueryParams resolves the page/pageSize pair for handlers that page an
// in-memory list. An absent value keeps the documented default; a present but
// unusable value is a 422 (手册 4.7.1: present means it must be usable, no
// silent fallback and no silent truncation). The middleware rejects these
// first, so this is the handler-side net for direct callers.
func listQueryParams(r *http.Request, defaultPageSize int) (page, pageSize int, err error) {
	page = 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || !platformpagination.ValidateQueryPage(parsed) {
			return 0, 0, platformpagination.NewInvalidQueryPageError(raw)
		}
		page = parsed
	}
	pageSize = defaultPageSize
	if raw := r.URL.Query().Get("pageSize"); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || !platformpagination.ValidateQueryPageSize(parsed) {
			return 0, 0, platformpagination.NewInvalidQueryPageSizeError(raw)
		}
		pageSize = parsed
	}
	return page, pageSize, nil
}

// ---------------------------------------------------------------------------
// Providers
// ---------------------------------------------------------------------------

func (h AdminHandler) handleListProviders(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	list, err := h.Service.ListProviders(r.Context(), orgID)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	writeList(w, r, list)
}

func (h AdminHandler) handleCreateProvider(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	var req appgateway.ProviderCreate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	req.OrganizationID = orgID
	resp, err := h.Service.CreateProvider(r.Context(), req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusCreated, resp)
}

func (h AdminHandler) handleGetProvider(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	resp, err := h.Service.GetProvider(r.Context(), orgID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	var req appgateway.ProviderUpdate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	resp, err := h.Service.UpdateProvider(r.Context(), orgID, id, req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Service.DeleteProvider(r.Context(), orgID, id); err != nil {
		route.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Models
// ---------------------------------------------------------------------------

func (h AdminHandler) handleListModels(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	list, err := h.Service.ListModels(r.Context(), orgID)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	writeList(w, r, list)
}

func (h AdminHandler) handleCreateModel(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	var req appgateway.ModelCreate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	req.OrganizationID = orgID
	resp, err := h.Service.CreateModel(r.Context(), req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusCreated, resp)
}

func (h AdminHandler) handleGetModel(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	resp, err := h.Service.GetModel(r.Context(), orgID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleUpdateModel(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	var req appgateway.ModelUpdate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	resp, err := h.Service.UpdateModel(r.Context(), orgID, id, req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Service.DeleteModel(r.Context(), orgID, id); err != nil {
		route.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Aliases
// ---------------------------------------------------------------------------

func (h AdminHandler) handleListAliases(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	list, err := h.Service.ListAliases(r.Context(), orgID)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	writeList(w, r, list)
}

func (h AdminHandler) handleCreateAlias(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	var req appgateway.AliasCreate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	req.OrganizationID = orgID
	resp, err := h.Service.CreateAlias(r.Context(), req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusCreated, resp)
}

func (h AdminHandler) handleGetAlias(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	resp, err := h.Service.GetAlias(r.Context(), orgID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleUpdateAlias(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	var req appgateway.AliasUpdate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	resp, err := h.Service.UpdateAlias(r.Context(), orgID, id, req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleDeleteAlias(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Service.DeleteAlias(r.Context(), orgID, id); err != nil {
		route.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// API Keys
// ---------------------------------------------------------------------------

func (h AdminHandler) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	list, err := h.Service.ListAPIKeys(r.Context(), orgID)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	writeList(w, r, list)
}

func (h AdminHandler) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	var req appgateway.APIKeyCreate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	req.OrganizationID = orgID
	resp, err := h.Service.CreateAPIKey(r.Context(), req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusCreated, resp)
}

func (h AdminHandler) handleGetAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	resp, err := h.Service.GetAPIKey(r.Context(), orgID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleUpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	var req appgateway.APIKeyUpdate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	resp, err := h.Service.UpdateAPIKey(r.Context(), orgID, id, req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Service.DeleteAPIKey(r.Context(), orgID, id); err != nil {
		route.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h AdminHandler) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Service.RevokeAPIKey(r.Context(), orgID, id); err != nil {
		route.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Providers — Test Connectivity
// ---------------------------------------------------------------------------

func (h AdminHandler) handleTestProvider(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	result, err := h.Service.TestProvider(r.Context(), orgID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	status := http.StatusOK
	if !result.Reachable {
		status = http.StatusOK // 连接测试本身不是业务流程失败，200 表示测试完成
	}
	route.WriteJSON(w, status, result)
}

// ---------------------------------------------------------------------------
// Routes
// ---------------------------------------------------------------------------

func (h AdminHandler) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	list, err := h.Service.ListRoutes(r.Context(), orgID)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	writeList(w, r, list)
}

func (h AdminHandler) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	var req appgateway.RouteCreate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	req.OrganizationID = orgID
	resp, err := h.Service.CreateRoute(r.Context(), req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusCreated, resp)
}

func (h AdminHandler) handleGetRoute(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	resp, err := h.Service.GetRoute(r.Context(), orgID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	var req appgateway.RouteUpdate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	resp, err := h.Service.UpdateRoute(r.Context(), orgID, id, req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Service.DeleteRoute(r.Context(), orgID, id); err != nil {
		route.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Quotas
// ---------------------------------------------------------------------------

func (h AdminHandler) handleListQuotas(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	list, err := h.Service.ListQuotas(r.Context(), orgID)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	writeList(w, r, list)
}

func (h AdminHandler) handleCreateQuota(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	var req appgateway.QuotaCreate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	req.OrganizationID = orgID
	resp, err := h.Service.CreateQuota(r.Context(), req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusCreated, resp)
}

func (h AdminHandler) handleGetQuota(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	resp, err := h.Service.GetQuota(r.Context(), orgID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleUpdateQuota(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	var req appgateway.QuotaUpdate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	resp, err := h.Service.UpdateQuota(r.Context(), orgID, id, req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleDeleteQuota(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Service.DeleteQuota(r.Context(), orgID, id); err != nil {
		route.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Usage Logs (只读)
// ---------------------------------------------------------------------------

func (h AdminHandler) handleListUsageLogs(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	page, pageSize, err := listQueryParams(r, platformpagination.DefaultPageSize)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	list, total, err := h.Service.ListUsageLogs(r.Context(), orgID, pageSize, platformpagination.Offset(page, pageSize))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	if list == nil {
		list = make([]appgateway.UsageLogResponse, 0)
	}

	route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(list, page, pageSize, total))
}

// handleGetUsageStats is GET /api/v1/ai/usage/stats — admin dashboard aggregate stats.
// Query param: ?days=7 (0 or missing = all-time).
func (h AdminHandler) handleGetUsageStats(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	days := route.QueryInt(r, "days", 0)
	if days < 0 || days > 365 {
		days = 0 // clamp invalid values
	}
	stats, err := h.Service.GetUsageStats(r.Context(), orgID, days)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, stats)
}

// ---------------------------------------------------------------------------
// Price Versions（管理面 CRUD）
// ---------------------------------------------------------------------------

func (h AdminHandler) handleListPrices(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	list, err := h.Service.ListPrices(r.Context(), orgID)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	writeList(w, r, list)
}

func (h AdminHandler) handleCreatePrice(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	var req appgateway.PriceVersionCreate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	req.OrganizationID = orgID
	resp, err := h.Service.CreatePrice(r.Context(), req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusCreated, resp)
}

func (h AdminHandler) handleGetPrice(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	resp, err := h.Service.GetPrice(r.Context(), orgID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleUpdatePrice(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	var req appgateway.PriceVersionUpdate
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}
	resp, err := h.Service.UpdatePrice(r.Context(), orgID, id, req)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

func (h AdminHandler) handleDeletePrice(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Service.DeletePrice(r.Context(), orgID, id); err != nil {
		route.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Provider Health（只读 — 状态由 GatewayService/HealthProbeWorker 自动写）
// ---------------------------------------------------------------------------

func (h AdminHandler) handleListHealth(w http.ResponseWriter, r *http.Request) {
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	list, err := h.Service.ListHealth(r.Context(), orgID)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	writeList(w, r, list)
}

func (h AdminHandler) handleGetHealth(w http.ResponseWriter, r *http.Request) {
	// Tenant resolution runs before the path id is read, so a forged query can
	// never be the only org-shaped value on the request.
	orgID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	resp, err := h.Service.GetHealthForOrganization(r.Context(), orgID, id)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, resp)
}

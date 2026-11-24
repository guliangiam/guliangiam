package tickets

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/tickets", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedListOrganizationID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		request := ListTicketsRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: organizationID,
			Keyword:        r.URL.Query().Get("keyword"),
			Assignee:       r.URL.Query().Get("assignee"),
			Creator:        r.URL.Query().Get("creator"),
			Priority:       r.URL.Query().Get("priority"),
			Type:           r.URL.Query().Get("type"),
		}
		if v := r.URL.Query().Get("status"); v != "" {
			request.Status = &v
		}
		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/tickets", func(w http.ResponseWriter, r *http.Request) {
		var request CreateTicketRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// The body's organizationId only requests a target tenant; the
		// authenticated scope decides which tenant is actually written.
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

	router.Get("/api/v1/tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Get(r.Context(), organizationID, GetTicketRequest{ID: chi.URLParam(r, "id")})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Patch("/api/v1/tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var request PatchTicketRequest
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

	router.Delete("/api/v1/tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
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

	router.Post("/api/v1/tickets/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		var req struct {
			Author     string `json:"author"`
			Content    string `json:"content"`
			Attachment string `json:"attachment,omitempty"`
		}
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		result, err := handler.AddTicketMessage(r.Context(), organizationID, chi.URLParam(r, "id"), trustedTicketAuthor(r, req.Author), req.Content, req.Attachment)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, result)
	})

	router.Get("/api/v1/tickets/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		page := route.QueryInt(r, "page", 1)
		pageSize := route.QueryInt(r, "pageSize", route.DefaultPageSize)

		response, err := handler.ListTicketMessages(r.Context(), organizationID, chi.URLParam(r, "id"), page, pageSize)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
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

// trustedListOrganizationID 解析列表的租户边界，与 /api/v1/search 同一口径：
// 平台管理员经 super_admin/system 升级后可留空 organizationId 做跨组织全量
// （可见范围由 RLS 决定），或显式指定收窄到单组织；普通成员一律限定在本组织
// scope，空值 fail-fast——在无 RLS 的方言上空 organization_id 会退化为全租户。
func trustedListOrganizationID(r *http.Request, requested string) (string, error) {
	scope := tenantctx.TenantScopeFromContext(r.Context())
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		return strings.TrimSpace(requested), nil
	}
	return trustedOrgID(r, requested)
}

// trustedTicketAuthor 以会话身份作为消息作者，客户端 body 的 author 仅在无会话
// 身份时兜底。作者决定回复通知的对手方（创建者回复提醒受理人，受理人回复提醒
// 创建者），若信任 body 既可伪造作者重定向通知，也会在缺省时把客服回复误判为
// 创建者自回复而漏发提醒。
func trustedTicketAuthor(r *http.Request, bodyAuthor string) string {
	if session, ok := authtransport.SessionContextFromContext(r.Context()); ok {
		if username := strings.TrimSpace(session.Username); username != "" {
			return username
		}
	}
	return strings.TrimSpace(bodyAuthor)
}

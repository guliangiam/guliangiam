package records

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// recordsPath 是审计记录资源的规范化路径（API 命名统一：/api/v1/<resource>）。
// /api/v1/audit/records 保留为向后兼容 307 redirect。
const recordsPath = "/api/v1/records"

func RegisterChiRoutes(router chi.Router, handler Handler) {
	// ── 规范化主路由：/api/v1/records ───────────────────────────────────────
	router.Get(recordsPath, func(w http.ResponseWriter, r *http.Request) {
		// 支持 audit/records 的 query param 别名
		q := r.URL.Query()
		request := ListRecordsRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: authtransport.TrustedOrganizationID(r, firstNonEmpty(q.Get("organizationId"), q.Get("organization_id"))),
			ActorID:        firstNonEmpty(q.Get("actorId"), q.Get("actor_id")),
			Action:         q.Get("action"),
			ResourceType:   firstNonEmpty(q.Get("resourceType"), q.Get("resource_type")),
			Keyword:        q.Get("keyword"),
			SortField:      firstNonEmpty(q.Get("sortField"), q.Get("sort_field")),
			SortOrder:      firstNonEmpty(q.Get("sortOrder"), q.Get("sort_order")),
		}

		response, err := handler.List(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post(recordsPath, func(w http.ResponseWriter, r *http.Request) {
		var request CreateRecordRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		// Tenant authority is the authenticated scope; the body may only
		// request a target tenant for a platform administrator.
		request.OrganizationID = authtransport.TrustedOrganizationID(r, request.OrganizationID)
		response, err := handler.Create(r, request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, response)
	})

	router.Get(recordsPath+"/export", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		parseTime := func(name string) (*time.Time, bool) {
			raw := q.Get(name)
			if raw == "" {
				return nil, true
			}
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				route.WriteError(w, r, apperrors.NewInvalidArgument(name+" must be RFC3339"))
				return nil, false
			}
			return &parsed, true
		}
		start, ok := parseTime("startTime")
		if !ok {
			return
		}
		end, ok := parseTime("endTime")
		if !ok {
			return
		}
		request := SearchRecordsRequest{
			OrganizationID: authtransport.TrustedOrganizationID(r, firstNonEmpty(q.Get("organizationId"), q.Get("organization_id"))),
			ActorID:        firstNonEmpty(q.Get("actorId"), q.Get("actor_id")),
			Action:         q.Get("action"),
			ResourceType:   firstNonEmpty(q.Get("resourceType"), q.Get("resource_type")),
			ResourceID:     firstNonEmpty(q.Get("resourceId"), q.Get("resource_id")),
			RequestID:      q.Get("requestId"),
			Keyword:        q.Get("keyword"),
			SortField:      firstNonEmpty(q.Get("sortField"), q.Get("sort_field")),
			SortOrder:      firstNonEmpty(q.Get("sortOrder"), q.Get("sort_order")),
			StartTime:      start,
			EndTime:        end,
		}
		response, err := handler.Export(r.Context(), r, request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ── 审计中心：分类日志与登录日志 ─────────────────────────────────
	auditRequest := func(w http.ResponseWriter, r *http.Request) (AuditLogListRequest, bool) {
		q := r.URL.Query()
		start, ok := parseOptionalTime(w, r, q.Get("startTime"))
		if !ok {
			return AuditLogListRequest{}, false
		}
		end, ok := parseOptionalTime(w, r, q.Get("endTime"))
		if !ok {
			return AuditLogListRequest{}, false
		}
		return AuditLogListRequest{
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
			OrganizationID: authtransport.TrustedOrganizationID(r, firstNonEmpty(q.Get("organizationId"), q.Get("organization_id"))),
			Category:       firstNonEmpty(q.Get("category"), q.Get("type")),
			Result:         q.Get("result"),
			ApplicationID:  firstNonEmpty(q.Get("applicationId"), q.Get("application_id")),
			Keyword:        q.Get("keyword"),
			StartTime:      start,
			EndTime:        end,
		}, true
	}

	router.Get("/api/v1/records/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		request, ok := auditRequest(w, r)
		if !ok {
			return
		}
		response, err := handler.ListAuditLogs(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/records/login-logs", func(w http.ResponseWriter, r *http.Request) {
		request, ok := auditRequest(w, r)
		if !ok {
			return
		}
		response, err := handler.ListLoginLogs(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post(recordsPath+"/search", func(w http.ResponseWriter, r *http.Request) {
		var request SearchRecordsRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = authtransport.TrustedOrganizationID(r, request.OrganizationID)
		if err := platformpagination.ValidateBodyPageParams(request.Page, request.PageSize); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Search(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ── 向后兼容 redirect：/api/v1/audit/records → /api/v1/records ──────
	redirect := func(w http.ResponseWriter, r *http.Request) {
		target := "/api/v1/records"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		if r.URL.Path == "/api/v1/audit/records/search" {
			target = "/api/v1/records/search"
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect) // 307 — 保持 POST
	}
	router.Get("/api/v1/audit/records", redirect)
	router.Post("/api/v1/audit/records", redirect)
	router.Post("/api/v1/audit/records/search", redirect)
}

// parseOptionalTime 解析可选的 RFC3339 时间参数；失败时已写入 400。
func parseOptionalTime(w http.ResponseWriter, r *http.Request, raw string) (*time.Time, bool) {
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		route.WriteError(w, r, apperrors.NewInvalidArgument("time parameters must be RFC3339"))
		return nil, false
	}
	return &parsed, true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

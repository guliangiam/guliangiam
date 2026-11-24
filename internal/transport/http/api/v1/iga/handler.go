package iga

import (
	"net/http"
	"time"

	app "github.com/guliangiam/guliangiam/internal/app/iga"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

type Handler struct{ Service app.Service }

func NewHandler(uow app.UnitOfWork) Handler { return Handler{Service: app.NewService(uow)} }

// orgFrom 从 TenantFilterMiddleware 注入的 typed context 读取 organization_id。
// 不信任裸 X-Organization-Id header（客户端可伪造），也不信任 string-key context。
// 必须走 tenantctx.TenantScopeFromContext(type-key)。
func orgFrom(r *http.Request) string {
	return tenantctx.TenantScopeFromContext(r.Context()).OrganizationID
}

func pageFrom(r *http.Request) (int, int) {
	return route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize)
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	route.WriteError(w, r, err)
}

// ─── Campaigns ─────────────────────────────────────────────────────────────

func (h Handler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	p, ps := pageFrom(r)
	q := app.ListCampaignsQuery{OrganizationID: org, Page: p, PageSize: ps}
	if v := r.URL.Query().Get("status"); v != "" {
		s := app.CampaignStatus(v)
		q.Status = &s
	}
	items, total, err := h.Service.ListCampaigns(r.Context(), q)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, p, ps, total))
}

func (h Handler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	var body struct {
		Name      string  `json:"name"`
		StartDate *string `json:"startDate,omitempty"`
		EndDate   *string `json:"endDate,omitempty"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	cmd := app.CreateCampaignCommand{OrganizationID: org, Name: body.Name}
	parse := func(field string, s *string) (*time.Time, error) {
		if s == nil || *s == "" {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339, *s)
		if err != nil {
			return nil, apperrors.NewInvalidArgument(field + " must be a valid RFC3339 timestamp")
		}
		return &t, nil
	}
	startDate, err := parse("startDate", body.StartDate)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	endDate, err := parse("endDate", body.EndDate)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	cmd.StartDate = startDate
	cmd.EndDate = endDate
	c, err := h.Service.CreateCampaign(r.Context(), cmd)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteCreated(w, "/api/v1/iga/access-campaigns/"+c.ID, c)
}

func (h Handler) GetCampaign(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	id := r.PathValue("id")
	c, err := h.Service.GetCampaign(r.Context(), org, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, c)
}

func (h Handler) UpdateCampaignStatus(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	id := r.PathValue("id")
	var body struct {
		Status string `json:"status"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	c, err := h.Service.UpdateCampaignStatus(r.Context(), org, id, app.CampaignStatus(body.Status))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, c)
}

// ─── SOD Rules ─────────────────────────────────────────────────────────────

func (h Handler) ListSODRules(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	p, ps := pageFrom(r)
	items, total, err := h.Service.ListSODRules(r.Context(), app.ListSODRulesQuery{OrganizationID: org, Page: p, PageSize: ps})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, p, ps, total))
}

func (h Handler) CreateSODRule(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	var body struct {
		Name    string `json:"name"`
		RoleAID string `json:"roleAId"`
		RoleBID string `json:"roleBId"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	c, err := h.Service.CreateSODRule(r.Context(), app.CreateSODRuleCommand{OrganizationID: org, Name: body.Name, RoleAID: body.RoleAID, RoleBID: body.RoleBID})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteCreated(w, "/api/v1/iga/sod-rules/"+c.ID, c)
}

func (h Handler) GetSODRule(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	id := r.PathValue("id")
	c, err := h.Service.GetSODRule(r.Context(), org, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, c)
}

// ─── Lifecycle ─────────────────────────────────────────────────────────────

func (h Handler) ListLifecycle(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	p, ps := pageFrom(r)
	q := app.ListLifecycleQuery{OrganizationID: org, Page: p, PageSize: ps}
	if v := r.URL.Query().Get("userId"); v != "" {
		q.UserID = &v
	}
	if v := r.URL.Query().Get("roleId"); v != "" {
		q.RoleID = &v
	}
	if v := r.URL.Query().Get("status"); v != "" {
		s := app.LifecycleStatus(v)
		q.Status = &s
	}
	items, total, err := h.Service.ListLifecycle(r.Context(), q)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, p, ps, total))
}

// ─── CRUD completeness (opt-backend-api) ────────────────────────────────────
// These endpoints are declared in routes.go for the full API contract.
// They now delegate to the app service layer (DeleteCampaign / UpdateSODRule /
// DeleteSODRule / CreateLifecycle / GetLifecycle / UpdateLifecycleStatus).

func (h Handler) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	id := r.PathValue("id")
	if err := h.Service.DeleteCampaign(r.Context(), org, id); err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusNoContent, nil)
}

func (h Handler) UpdateSODRule(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	id := r.PathValue("id")
	var body struct {
		Name    string `json:"name"`
		RoleAID string `json:"roleAId"`
		RoleBID string `json:"roleBId"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	d, err := h.Service.UpdateSODRule(r.Context(), org, id, body.Name, body.RoleAID, body.RoleBID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, d)
}

func (h Handler) DeleteSODRule(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	id := r.PathValue("id")
	if err := h.Service.DeleteSODRule(r.Context(), org, id); err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusNoContent, nil)
}

func (h Handler) CreateLifecycle(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	var body struct {
		UserID    string  `json:"userId"`
		RoleID    string  `json:"roleId"`
		ExpiresAt *string `json:"expiresAt,omitempty"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	// Default 24h if expiresAt omitted; an explicitly provided but invalid
	// timestamp is a client error, never silently replaced by the default.
	expires := time.Now().UTC().Add(24 * time.Hour)
	if body.ExpiresAt != nil && *body.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *body.ExpiresAt)
		if err != nil {
			writeErr(w, r, apperrors.NewInvalidArgument("expiresAt must be a valid RFC3339 timestamp"))
			return
		}
		expires = t
	}
	d, err := h.Service.CreateLifecycle(r.Context(), org, body.UserID, body.RoleID, expires)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteCreated(w, "/api/v1/iga/access-lifecycle/"+d.ID, d)
}

func (h Handler) GetLifecycle(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	id := r.PathValue("id")
	d, err := h.Service.GetLifecycle(r.Context(), org, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, d)
}

func (h Handler) UpdateLifecycleStatus(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r)
	id := r.PathValue("id")
	var body struct {
		Status string `json:"status"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	d, err := h.Service.UpdateLifecycleStatus(r.Context(), org, id, app.LifecycleStatus(body.Status))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, d)
}

func actorFrom(r *http.Request) (string, string) {
	session, _ := authtransport.SessionContextFromContext(r.Context())
	return session.UserID, session.ID
}

func requestActionCommand(r *http.Request, requestID string, revision int64, comment string) app.AccessRequestActionCommand {
	actorID, _ := actorFrom(r)
	return app.AccessRequestActionCommand{OrganizationID: orgFrom(r), RequestID: requestID, ActorID: actorID, Revision: revision, Comment: comment}
}

func (h Handler) ListAccessRequests(w http.ResponseWriter, r *http.Request) {
	p, ps := pageFrom(r)
	q := app.ListAccessRequestsQuery{OrganizationID: orgFrom(r), RequesterID: r.URL.Query().Get("requesterId"), Page: p, PageSize: ps}
	if value := r.URL.Query().Get("status"); value != "" {
		status := app.RequestStatus(value)
		q.Status = &status
	}
	items, total, err := h.Service.ListAccessRequests(r.Context(), q)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, p, ps, total))
}

func (h Handler) CreateAccessRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RoleID        string `json:"roleId"`
		ApplicationID string `json:"applicationId"`
		EnforcerID    string `json:"enforcerId"`
		Resource      string `json:"resource"`
		Action        string `json:"action"`
		Reason        string `json:"reason"`
		StartsAt      string `json:"startsAt"`
		ExpiresAt     string `json:"expiresAt"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	startsAt, err := time.Parse(time.RFC3339, body.StartsAt)
	if err != nil {
		writeErr(w, r, apperrors.NewInvalidArgument("valid startsAt is required"))
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, body.ExpiresAt)
	if err != nil {
		writeErr(w, r, apperrors.NewInvalidArgument("valid expiresAt is required"))
		return
	}
	actorID, _ := actorFrom(r)
	item, err := h.Service.CreateAccessRequest(r.Context(), app.CreateAccessRequestCommand{
		OrganizationID: orgFrom(r), RequesterID: actorID, RoleID: body.RoleID,
		ApplicationID: body.ApplicationID, EnforcerID: body.EnforcerID,
		Resource: body.Resource, Action: body.Action, Reason: body.Reason,
		StartsAt: startsAt, ExpiresAt: expiresAt,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteCreated(w, "/api/v1/iga/access-requests/"+item.ID, item)
}

func (h Handler) GetAccessRequest(w http.ResponseWriter, r *http.Request) {
	item, err := h.Service.GetAccessRequest(r.Context(), orgFrom(r), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, item)
}

func (h Handler) SubmitAccessRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Revision int64  `json:"revision"`
		Comment  string `json:"comment"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := h.Service.SubmitAccessRequest(r.Context(), requestActionCommand(r, r.PathValue("id"), body.Revision, body.Comment))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, item)
}

func (h Handler) ApproveAccessRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Stage    string `json:"stage"`
		Revision int64  `json:"revision"`
		Comment  string `json:"comment"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := h.Service.ApproveAccessRequest(r.Context(), body.Stage, requestActionCommand(r, r.PathValue("id"), body.Revision, body.Comment))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, item)
}

func (h Handler) ProvisionAccessRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Revision int64 `json:"revision"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := h.Service.ProvisionAccessRequest(r.Context(), requestActionCommand(r, r.PathValue("id"), body.Revision, ""))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, item)
}

func (h Handler) RejectAccessRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Stage    string `json:"stage"`
		Revision int64  `json:"revision"`
		Comment  string `json:"comment"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := h.Service.RejectAccessRequest(r.Context(), body.Stage, requestActionCommand(r, r.PathValue("id"), body.Revision, body.Comment))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, item)
}

func (h Handler) WithdrawAccessRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Revision int64  `json:"revision"`
		Comment  string `json:"comment"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := h.Service.WithdrawAccessRequest(r.Context(), requestActionCommand(r, r.PathValue("id"), body.Revision, body.Comment))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusOK, item)
}

func (h Handler) StartPrivilegedSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RequestID string `json:"requestId"`
		Revision  int64  `json:"revision"`
	}
	if err := route.DecodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	_, sessionID := actorFrom(r)
	item, err := h.Service.StartPrivilegedSession(r.Context(), requestActionCommand(r, body.RequestID, body.Revision, ""), sessionID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	route.WriteJSON(w, http.StatusCreated, item)
}

func (h Handler) ListPrivilegedSessions(w http.ResponseWriter, r *http.Request) {
	actorID, _ := actorFrom(r)
	items, err := h.Service.ListPrivilegedSessions(r.Context(), orgFrom(r), actorID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	page, pageSize := pageFrom(r)
	total := int64(len(items))
	start, end := privilegedSessionBounds(len(items), page, pageSize)
	pageItems := items[start:end]
	if pageItems == nil {
		pageItems = make([]app.PrivilegedSessionDTO, 0)
	}
	route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(pageItems, page, pageSize, total))
}

// privilegedSessionBounds maps a 1-based page onto the half-open range of the
// in-memory privileged-session list. The page size is clamped before the offset
// and the end are computed, and Offset clamps the page before multiplying, so a
// hostile page cannot wrap the product negative.
func privilegedSessionBounds(total, page, pageSize int) (start, end int) {
	pageSize = platformpagination.ClampPageSize(pageSize)
	start = platformpagination.Offset(page, pageSize)
	if start > total {
		start = total
	}
	end = total
	if start < total {
		end = min(start+pageSize, total)
	}
	return start, end
}

package actions

import (
	"context"
	"encoding/json"

	appactions "github.com/guliangiam/guliangiam/internal/app/actions"
)

// Handler adapts HTTP requests to the app-layer actions service. It is
// injected by bootstrap and keeps chi thin — no business logic lives here.
type Handler struct {
	Service appactions.Service
}

// ─── ListActions ─────────────────────────────────────────────────────────

type ListActionsQuery struct {
	OrganizationID string
	TriggerType    *string
	Status         *string
	Page           int
	PageSize       int
}

func (h Handler) ListActions(ctx context.Context, q ListActionsQuery) ([]appactions.ActionDTO, int64, error) {
	var triggerPtr *appactions.TriggerType
	if q.TriggerType != nil {
		t := appactions.TriggerType(*q.TriggerType)
		triggerPtr = &t
	}
	var statusPtr *appactions.ActionStatus
	if q.Status != nil {
		s := appactions.ActionStatus(*q.Status)
		statusPtr = &s
	}
	return h.Service.List(ctx, appactions.ListActionsQuery{
		OrganizationID: q.OrganizationID,
		TriggerType:    triggerPtr,
		Status:         statusPtr,
		Page:           q.Page,
		PageSize:       q.PageSize,
	})
}

// ─── GetAction ───────────────────────────────────────────────────────────

func (h Handler) GetAction(ctx context.Context, organizationID, id string) (*appactions.ActionDTO, error) {
	return h.Service.Get(ctx, organizationID, id)
}

// ─── CreateAction ─────────────────────────────────────────────────────────

type CreateActionRequest struct {
	Name        string `json:"name"`
	TriggerType string `json:"triggerType"`
	Code        string `json:"code"`
}

func (h Handler) CreateAction(ctx context.Context, organizationID string, req CreateActionRequest) (*appactions.ActionDTO, error) {
	return h.Service.Create(ctx, appactions.CreateActionCommand{
		OrganizationID: organizationID,
		Name:           req.Name,
		TriggerType:    appactions.TriggerType(req.TriggerType),
		Code:           req.Code,
	})
}

// ─── UpdateActionStatus ───────────────────────────────────────────────────

type UpdateStatusRequest struct {
	Status   string `json:"status"`
	Revision int64  `json:"revision,omitempty"`
}

func (h Handler) UpdateActionStatus(ctx context.Context, organizationID, id string, req UpdateStatusRequest) (*appactions.ActionDTO, error) {
	return h.Service.UpdateStatus(ctx, appactions.UpdateStatusCommand{
		OrganizationID: organizationID,
		ActionID:       id,
		Status:         appactions.ActionStatus(req.Status),
		Revision:       req.Revision,
	})
}

// ─── ListVersions ─────────────────────────────────────────────────────────

type ListVersionsQuery struct {
	OrganizationID string
	ActionID       string
	Page           int
	PageSize       int
}

func (h Handler) ListVersions(ctx context.Context, q ListVersionsQuery) ([]appactions.ActionVersionDTO, int64, error) {
	return h.Service.ListVersions(ctx, appactions.ListActionVersionsQuery{
		OrganizationID: q.OrganizationID,
		ActionID:       q.ActionID,
		Page:           q.Page,
		PageSize:       q.PageSize,
	})
}

// ─── PublishVersion ───────────────────────────────────────────────────────

type PublishVersionRequest struct {
	Code string `json:"code"`
}

func (h Handler) PublishVersion(ctx context.Context, organizationID, actionID string, req PublishVersionRequest) (*appactions.ActionVersionDTO, error) {
	return h.Service.PublishVersion(ctx, appactions.PublishVersionCommand{
		OrganizationID: organizationID,
		ActionID:       actionID,
		Code:           req.Code,
	})
}

// ─── RunAction ────────────────────────────────────────────────────────────

type RunActionRequest struct {
	Input json.RawMessage `json:"input"`
}

func (h Handler) RunAction(ctx context.Context, organizationID, actionID string, req RunActionRequest) (*appactions.ExecutionLogDTO, error) {
	return h.Service.Run(ctx, organizationID, actionID, req.Input)
}

// ─── ListExecutionLogs ─────────────────────────────────────────────────────

type ListExecLogsQuery struct {
	OrganizationID string
	ActionID       string
	Status         *string
	Page           int
	PageSize       int
}

func (h Handler) ListExecutionLogs(ctx context.Context, q ListExecLogsQuery) ([]appactions.ExecutionLogDTO, int64, error) {
	var statusPtr *appactions.ExecutionStatus
	if q.Status != nil {
		s := appactions.ExecutionStatus(*q.Status)
		statusPtr = &s
	}
	return h.Service.ListExecutionLogs(ctx, appactions.ListExecutionLogsQuery{
		OrganizationID: q.OrganizationID,
		ActionID:       q.ActionID,
		Status:         statusPtr,
		Page:           q.Page,
		PageSize:       q.PageSize,
	})
}

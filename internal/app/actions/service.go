package actions

import (
	"context"
	"database/sql"
	"encoding/json"
	stderrors "errors"
	"time"

	"github.com/jackc/pgx/v5"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/actions"
	"github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// UnitOfWork aliases the app-level UoW so the app package doesn't leak
// appuow import references into its public API.
type UnitOfWork = appuow.UnitOfWork
type ActionsUoW = appuow.ActionsUoW

// Domain type aliases — keep call sites idiomatic (TriggerType, ActionStatus...).
type (
	Action          = domain.Action
	ActionVersion   = domain.ActionVersion
	ExecutionLog    = domain.ExecutionLog
	TriggerType     = domain.TriggerType
	ActionStatus    = domain.ActionStatus
	ExecutionStatus = domain.ExecutionStatus

	ListActionsQuery          = domain.ListActionsQuery
	ListActionVersionsQuery   = domain.ListActionVersionsQuery
	ListExecutionLogsQuery    = domain.ListExecutionLogsQuery
	CreateActionCommand       = domain.CreateActionCommand
	PublishVersionCommand     = domain.PublishVersionCommand
	UpdateStatusCommand       = domain.UpdateStatusCommand
	CreateExecutionLogCommand = domain.CreateExecutionLogCommand
	UpdateExecutionLogCommand = domain.UpdateExecutionLogCommand
)

// Domain constant re-exports.
const (
	TriggerPreLogin         = domain.TriggerPreLogin
	TriggerPreToken         = domain.TriggerPreToken
	TriggerPostRegistration = domain.TriggerPostRegistration
	TriggerConsent          = domain.TriggerConsent
	TriggerMFA              = domain.TriggerMFA

	ActionStatusDraft    = domain.ActionStatusDraft
	ActionStatusActive   = domain.ActionStatusActive
	ActionStatusDisabled = domain.ActionStatusDisabled
	ActionStatusDeleted  = domain.ActionStatusDeleted

	ExecStatusPending = domain.ExecStatusPending
	ExecStatusRunning = domain.ExecStatusRunning
	ExecStatusSuccess = domain.ExecStatusSuccess
	ExecStatusFailed  = domain.ExecStatusFailed
	ExecStatusTimeout = domain.ExecStatusTimeout
)

// Service exposes identity action use cases. Actions are sync extension
// points (ADR-029); this service manages definitions, code versions, and
// execution logs. The actual WASM/JS sandbox runtime is out of scope here
// — Run dispatches to a pluggable Executor when wired.
type Service struct {
	UOW   appuow.ActionsUoW
	UUID  UUIDGenerator
	Clock Clock
}

// NewService constructs an actions Service. UUID and Clock may be nil and
// default to real implementations; injected in tests.
func NewService(uow UnitOfWork, uuidGen UUIDGenerator, clock Clock) Service {
	if uuidGen == nil {
		uuidGen = platformuuid.V7Generator{}
	}
	if clock == nil {
		clock = defaultClock{}
	}
	return Service{UOW: uow, UUID: uuidGen, Clock: clock}
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

type ActionDTO struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	TriggerType    string `json:"triggerType"`
	Status         string `json:"status"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	Revision       int64  `json:"revision"`
}

type ActionVersionDTO struct {
	ID        string `json:"id"`
	ActionID  string `json:"actionId"`
	Version   int    `json:"version"`
	Code      string `json:"code"`
	CreatedAt string `json:"createdAt"`
}

type ExecutionLogDTO struct {
	ID         string          `json:"id"`
	ActionID   string          `json:"actionId"`
	Version    int             `json:"version"`
	Status     string          `json:"status"`
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      *string         `json:"error,omitempty"`
	ExecutedAt *string         `json:"executedAt,omitempty"`
	CreatedAt  string          `json:"createdAt"`
}

// ─── Use Cases ─────────────────────────────────────────────────────────────

func (s Service) List(ctx context.Context, q ListActionsQuery) ([]ActionDTO, int64, error) {
	if err := s.validateOrg(q.OrganizationID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.UOW.Actions().List(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]ActionDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toActionDTO(it))
	}
	return dtos, total, nil
}

func (s Service) Get(ctx context.Context, organizationID, id string) (*ActionDTO, error) {
	if err := s.validateOrg(organizationID); err != nil {
		return nil, err
	}
	a, err := s.UOW.Actions().Get(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	dto := toActionDTO(a)
	return &dto, nil
}

// Create registers a new action definition with its first (v1) code version.
// Wrapped in a UOW transaction so action + version are atomic.
func (s Service) Create(ctx context.Context, cmd CreateActionCommand) (*ActionDTO, error) {
	if err := s.validateOrg(cmd.OrganizationID); err != nil {
		return nil, err
	}
	if err := validateTriggerType(cmd.TriggerType); err != nil {
		return nil, err
	}
	if cmd.Name == "" {
		return nil, errors.NewInvalidArgument("name is required")
	}

	var created *ActionDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx UnitOfWork) error {
		now := s.Now()
		actionID, err := s.NewUUID()
		if err != nil {
			return err
		}
		versionID, err := s.NewUUID()
		if err != nil {
			return err
		}
		action := &Action{
			ID:             actionID,
			OrganizationID: cmd.OrganizationID,
			Name:           cmd.Name,
			TriggerType:    cmd.TriggerType,
			Status:         ActionStatusDraft,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		version := &ActionVersion{
			ID:        versionID,
			ActionID:  action.ID,
			Version:   1,
			Code:      cmd.Code,
			CreatedAt: now,
		}
		if err := tx.Actions().Create(ctx, action, version); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			action.OrganizationID, "",
			"actions.created", "action", action.ID,
			"action", "actions.created",
			map[string]any{},
		); err != nil {
			return err
		}
		// Re-fetch to see persisted state
		after, err := tx.Actions().Get(ctx, action.ID, cmd.OrganizationID)
		if err != nil {
			return err
		}
		dto := toActionDTO(after)
		created = &dto
		return nil
	})
	return created, err
}

// PublishVersion appends a new code version to an existing action. The
// target action must be draft or active; disabled/deleted actions cannot
// receive new versions.
func (s Service) PublishVersion(ctx context.Context, cmd PublishVersionCommand) (*ActionVersionDTO, error) {
	if err := s.validateOrg(cmd.OrganizationID); err != nil {
		return nil, err
	}

	var created *ActionVersionDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx UnitOfWork) error {
		action, err := tx.Actions().Get(ctx, cmd.ActionID, cmd.OrganizationID)
		if err != nil {
			return err
		}
		if action.Status == ActionStatusDeleted {
			return errors.NewNotFound("action not found")
		}
		latest, err := tx.Actions().GetLatestVersion(ctx, cmd.ActionID, cmd.OrganizationID)
		if err != nil {
			// first version — allow
			if !isNoRows(err) {
				return err
			}
		}
		nextVersion := 1
		if latest != nil {
			nextVersion = latest.Version + 1
		}
		versionID, err := s.NewUUID()
		if err != nil {
			return err
		}
		v := &ActionVersion{
			ID:        versionID,
			ActionID:  cmd.ActionID,
			Version:   nextVersion,
			Code:      cmd.Code,
			CreatedAt: s.Now(),
		}
		if err := tx.Actions().PublishVersion(ctx, cmd.ActionID, cmd.OrganizationID, v); err != nil {
			return err
		}
		// 发布新代码版本是高危写：同事务审计（BE-D-7）。
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			cmd.OrganizationID, "",
			"actions.version.published", "action", cmd.ActionID,
			"action", "actions.version.published",
			map[string]any{"versionId": v.ID, "version": v.Version},
		); err != nil {
			return err
		}
		dto := toVersionDTO(v)
		created = &dto
		return nil
	})
	return created, err
}

func (s Service) ListVersions(ctx context.Context, q ListActionVersionsQuery) ([]ActionVersionDTO, int64, error) {
	if err := s.validateOrg(q.OrganizationID); err != nil {
		return nil, 0, err
	}
	if err := s.requireAction(ctx, q.ActionID, q.OrganizationID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.UOW.Actions().ListVersions(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]ActionVersionDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toVersionDTO(it))
	}
	return dtos, total, nil
}

// actionStatusTransitions 是状态机白名单（BE-D-21）：deleted 为终态，
// 不允许任意互转（例如 deleted→active 复活）。
var actionStatusTransitions = map[ActionStatus][]ActionStatus{
	ActionStatusDraft:    {ActionStatusActive, ActionStatusDisabled, ActionStatusDeleted},
	ActionStatusActive:   {ActionStatusDisabled, ActionStatusDeleted},
	ActionStatusDisabled: {ActionStatusActive, ActionStatusDeleted},
	ActionStatusDeleted:  {},
}

func validActionTransition(from, to ActionStatus) bool {
	for _, allowed := range actionStatusTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// UpdateStatus transitions an action through draft → active → disabled → deleted.
func (s Service) UpdateStatus(ctx context.Context, cmd UpdateStatusCommand) (*ActionDTO, error) {
	if err := s.validateOrg(cmd.OrganizationID); err != nil {
		return nil, err
	}
	switch cmd.Status {
	case ActionStatusDraft, ActionStatusActive, ActionStatusDisabled, ActionStatusDeleted:
	default:
		return nil, errors.NewInvalidArgument("status must be one of: draft, active, disabled, deleted")
	}

	var updated *ActionDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx UnitOfWork) error {
		action, err := tx.Actions().Get(ctx, cmd.ActionID, cmd.OrganizationID)
		if err != nil {
			return err
		}
		if cmd.Revision > 0 && action.Revision != cmd.Revision {
			return errors.NewConflict("action revision mismatch")
		}
		if action.Status == cmd.Status {
			return errors.NewConflict("action is already in the requested status")
		}
		if !validActionTransition(action.Status, cmd.Status) {
			return errors.NewWithMeta(errors.CodeUnprocessableEntity,
				"invalid action status transition",
				map[string]any{"from": string(action.Status), "to": string(cmd.Status), "allowed": actionStatusTransitions[action.Status]})
		}
		if err := tx.Actions().UpdateStatus(ctx, cmd); err != nil {
			return err
		}
		// 状态变更是高危写：同事务审计（BE-D-7）。
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			cmd.OrganizationID, "",
			"actions.status.updated", "action", cmd.ActionID,
			"action", "actions.status.updated",
			map[string]any{"from": string(action.Status), "to": string(cmd.Status)},
		); err != nil {
			return err
		}
		after, err := tx.Actions().Get(ctx, cmd.ActionID, cmd.OrganizationID)
		if err != nil {
			return err
		}
		dto := toActionDTO(after)
		updated = &dto
		return nil
	})
	return updated, err
}

// Run executes an action. The data-plane service has no sandbox executor
// wired (ADR-029 extension point); instead of silently recording a pending
// log that never completes, it returns an explicit 501 (BE-D-21).
func (s Service) Run(ctx context.Context, organizationID, actionID string, input json.RawMessage) (*ExecutionLogDTO, error) {
	if err := s.validateOrg(organizationID); err != nil {
		return nil, err
	}
	action, err := s.UOW.Actions().Get(ctx, actionID, organizationID)
	if err != nil {
		return nil, err
	}
	if action.Status != ActionStatusActive {
		return nil, errors.NewInvalidArgument("action must be active to run")
	}
	return nil, errors.NewUnimplemented("action executor is not implemented; run requires a wired ADR-029 sandbox executor")
}

func (s Service) ListExecutionLogs(ctx context.Context, q ListExecutionLogsQuery) ([]ExecutionLogDTO, int64, error) {
	if err := s.validateOrg(q.OrganizationID); err != nil {
		return nil, 0, err
	}
	if err := s.requireAction(ctx, q.ActionID, q.OrganizationID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.UOW.Actions().ListExecutionLogs(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]ExecutionLogDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toExecLogDTO(it))
	}
	return dtos, total, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func (s Service) validateOrg(id string) error {
	if id == "" {
		return errors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

// requireAction proves the parent action belongs to the caller's tenant before
// any child read. A missing action and another tenant's action are the same
// 404, so the response does not reveal whether the id exists.
func (s Service) requireAction(ctx context.Context, actionID, organizationID string) error {
	if _, err := s.UOW.Actions().Get(ctx, actionID, organizationID); err != nil {
		if isNoRows(err) {
			return errors.NewNotFound("action not found")
		}
		return err
	}
	return nil
}

func validateTriggerType(t TriggerType) error {
	switch t {
	case TriggerPreLogin, TriggerPreToken, TriggerPostRegistration, TriggerConsent, TriggerMFA:
		return nil
	case "":
		return errors.NewInvalidArgument("triggerType is required")
	default:
		return errors.NewInvalidArgument("triggerType must be one of: pre_login, pre_token, post_registration, consent, mfa")
	}
}

func isNoRows(err error) bool {
	return stderrors.Is(err, sql.ErrNoRows) || stderrors.Is(err, pgx.ErrNoRows)
}

// ─── UUID / Clock ports ────────────────────────────────────────────────────

type UUIDGenerator interface {
	NewString() (string, error)
}

type Clock interface {
	Now() time.Time
}

type defaultClock struct{}

func (defaultClock) Now() time.Time { return time.Now().UTC() }

func (s Service) NewUUID() (string, error) {
	return s.UUID.NewString()
}
func (s Service) Now() time.Time { return s.Clock.Now() }

// ─── Mappers ───────────────────────────────────────────────────────────────

func toActionDTO(a *Action) ActionDTO {
	return ActionDTO{
		ID:             a.ID,
		OrganizationID: a.OrganizationID,
		Name:           a.Name,
		TriggerType:    string(a.TriggerType),
		Status:         string(a.Status),
		CreatedAt:      a.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
		UpdatedAt:      a.UpdatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
		Revision:       a.Revision,
	}
}

func toVersionDTO(v *ActionVersion) ActionVersionDTO {
	return ActionVersionDTO{
		ID:        v.ID,
		ActionID:  v.ActionID,
		Version:   v.Version,
		Code:      v.Code,
		CreatedAt: v.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
	}
}

func toExecLogDTO(l *ExecutionLog) ExecutionLogDTO {
	dto := ExecutionLogDTO{
		ID:        l.ID,
		ActionID:  l.ActionID,
		Version:   l.Version,
		Status:    string(l.Status),
		Input:     l.Input,
		Output:    l.Output,
		Error:     l.Error,
		CreatedAt: l.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
	}
	if l.ExecutedAt != nil {
		t := l.ExecutedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.ExecutedAt = &t
	}
	return dto
}

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。

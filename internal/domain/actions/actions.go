package actions

import (
	"encoding/json"
	"time"
)

// ActionStatus mirrors the DB CHECK constraint ck_identity_actions_status.
type ActionStatus string

const (
	ActionStatusDraft    ActionStatus = "draft"
	ActionStatusActive   ActionStatus = "active"
	ActionStatusDisabled ActionStatus = "disabled"
	ActionStatusDeleted  ActionStatus = "deleted"
)

// TriggerType enumerates the five sync extension points per ADR-029.
type TriggerType string

const (
	TriggerPreLogin         TriggerType = "pre_login"
	TriggerPreToken         TriggerType = "pre_token"
	TriggerPostRegistration TriggerType = "post_registration"
	TriggerConsent          TriggerType = "consent"
	TriggerMFA              TriggerType = "mfa"
)

// ExecutionStatus mirrors the DB CHECK constraint ck_execution_logs_status.
type ExecutionStatus string

const (
	ExecStatusPending ExecutionStatus = "pending"
	ExecStatusRunning ExecutionStatus = "running"
	ExecStatusSuccess ExecutionStatus = "success"
	ExecStatusFailed  ExecutionStatus = "failed"
	ExecStatusTimeout ExecutionStatus = "timeout"
)

// Action is an identity action definition — sync extension point per ADR-029.
type Action struct {
	ID             string
	OrganizationID string
	Name           string
	TriggerType    TriggerType
	Status         ActionStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// ActionVersion is an immutable snapshot of an action's code.
type ActionVersion struct {
	ID        string
	ActionID  string
	Version   int
	Code      string
	CreatedAt time.Time
}

// ExecutionLog records a single execution of an action at some version.
type ExecutionLog struct {
	ID         string
	ActionID   string
	Version    int
	Status     ExecutionStatus
	Input      json.RawMessage
	Output     json.RawMessage
	Error      *string
	ExecutedAt *time.Time
	CreatedAt  time.Time
}

// ─── Queries ───────────────────────────────────────────────────────────────

type ListActionsQuery struct {
	OrganizationID string
	TriggerType    *TriggerType
	Status         *ActionStatus
	Page           int
	PageSize       int
}

type ListActionVersionsQuery struct {
	OrganizationID string
	ActionID       string
	Page           int
	PageSize       int
}

type ListExecutionLogsQuery struct {
	OrganizationID string
	ActionID       string
	Status         *ExecutionStatus
	Page           int
	PageSize       int
}

// ─── Commands ──────────────────────────────────────────────────────────────

type CreateActionCommand struct {
	OrganizationID string
	Name           string
	TriggerType    TriggerType
	Code           string
}

type PublishVersionCommand struct {
	OrganizationID string
	ActionID       string
	Code           string
}

type UpdateStatusCommand struct {
	OrganizationID string
	ActionID       string
	Status         ActionStatus
	// Revision 为可选乐观并发检查：>0 时必须与当前行 revision 一致。
	Revision int64
}

type CreateExecutionLogCommand struct {
	ID             string
	OrganizationID string
	ActionID       string
	Version        int
	Input          json.RawMessage
}

type UpdateExecutionLogCommand struct {
	OrganizationID string
	LogID          string
	Status         ExecutionStatus
	Output         json.RawMessage
	Error          *string
}

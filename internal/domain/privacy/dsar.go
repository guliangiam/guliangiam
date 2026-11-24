package privacy

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DSAR 请求类型（design/privacy §4 六类）。
const (
	RequestTypeAccess          = "access"
	RequestTypeRectify         = "rectify"
	RequestTypeFreeze          = "freeze"
	RequestTypeErase           = "erase"
	RequestTypePortability     = "portability"
	RequestTypeWithdrawConsent = "withdraw_consent"
)

// DSAR 状态机（design/privacy §3）。
const (
	StateReceived   = "received"
	StateVerifying  = "verifying"
	StateInProgress = "in_progress"
	StateFulfilled  = "fulfilled"
	StateRejected   = "rejected"
)

// 数据主体类型（契约：subject_type IN ('user','contact')）。
const (
	SubjectTypeUser    = "user"
	SubjectTypeContact = "contact"
)

// SLABudget 是 DSAR 的法定响应窗口上限（≤30 天）。
const SLABudget = 30 * 24 * time.Hour

// CoolingWindow 是擦除请求的冷静期（design/privacy §4 撤回同意/擦除）。
// 进入 in_progress 的 erase 请求在冷静期内可由数据主体撤回（-> rejected），
// 冷静期结束前不得标记 fulfilled（处置不可逆）。默认 7 天，符合 AC-PRIV-002。
const CoolingWindow = 7 * 24 * time.Hour

var requestTypes = []string{
	RequestTypeAccess, RequestTypeRectify, RequestTypeFreeze,
	RequestTypeErase, RequestTypePortability, RequestTypeWithdrawConsent,
}

var subjectTypes = []string{SubjectTypeUser, SubjectTypeContact}

var states = []string{StateReceived, StateVerifying, StateInProgress, StateFulfilled, StateRejected}

// transitions 是状态机的唯一声明；终态没有出边（不可逆）。
var transitions = map[string][]string{
	StateReceived:   {StateVerifying},
	StateVerifying:  {StateInProgress, StateRejected},
	StateInProgress: {StateFulfilled, StateRejected},
	StateFulfilled:  {},
	StateRejected:   {},
}

// DsarRequest 是数据主体权利请求聚合根。
type DsarRequest struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organizationId"`
	SubjectType     string     `json:"subjectType"`
	SubjectID       string     `json:"subjectId"`
	RequestType     string     `json:"requestType"`
	State           string     `json:"state"`
	SLADueAt        time.Time  `json:"slaDueAt"`
	ClosedAt        *time.Time `json:"closedAt,omitempty"`
	CoolingUntil    *time.Time `json:"coolingUntil,omitempty"`
	RejectionReason *string    `json:"rejectionReason,omitempty"`
	CreatedBy       *string    `json:"createdBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	Revision        int64      `json:"revision"`
}

// NewDsarRequest 构造一个 received 态请求，并登记 SLA 起始时刻。
func NewDsarRequest(id, organizationID, subjectType, subjectID, requestType string, createdBy *string, now time.Time) (*DsarRequest, error) {
	request := &DsarRequest{
		ID:             id,
		OrganizationID: organizationID,
		SubjectType:    subjectType,
		SubjectID:      subjectID,
		RequestType:    requestType,
		State:          StateReceived,
		SLADueAt:       now.Add(SLABudget),
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return request, nil
}

// Validate 校验聚合的必备字段与状态/终态不变量（对齐表 CHECK 约束）。
func (r *DsarRequest) Validate() error {
	if r.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if r.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if !contains(subjectTypes, r.SubjectType) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid subjectType",
			map[string]any{"field": "subjectType", "value": r.SubjectType, "allowed": subjectTypes})
	}
	if r.SubjectID == "" {
		return apperrors.NewInvalidArgument("subjectId is required")
	}
	if !contains(requestTypes, r.RequestType) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid requestType",
			map[string]any{"field": "requestType", "value": r.RequestType, "allowed": requestTypes})
	}
	if !contains(states, r.State) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid state",
			map[string]any{"field": "state", "value": r.State, "allowed": states})
	}
	if r.SLADueAt.IsZero() {
		return apperrors.NewInvalidArgument("slaDueAt is required")
	}
	if !r.IsTerminal() && r.ClosedAt != nil {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "closedAt must be empty while request is open",
			map[string]any{"field": "closedAt", "value": r.State})
	}
	if r.IsTerminal() && r.ClosedAt == nil {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "closedAt is required for terminal states",
			map[string]any{"field": "closedAt", "value": r.State})
	}
	if r.State == StateRejected {
		if r.RejectionReason == nil || *r.RejectionReason == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "rejectionReason is required for rejected requests",
				map[string]any{"field": "rejectionReason", "value": r.State})
		}
	}
	if r.State != StateRejected && r.RejectionReason != nil {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "rejectionReason is only allowed for rejected requests",
			map[string]any{"field": "rejectionReason", "value": r.State})
	}
	return nil
}

// TransitionTo 执行一次状态迁移；终态不可逆，rejected 必须带理由。
// InCooling reports whether an erase request is still inside its cooling window.
func (r *DsarRequest) InCooling(now time.Time) bool {
	return r.CoolingUntil != nil && now.Before(*r.CoolingUntil)
}

func (r *DsarRequest) TransitionTo(target, reason string, now time.Time) error {
	// 冷静期规则（AC-PRIV-002）：
	//   1. erase 进入 in_progress 时登记 cooling_until；
	//   2. 冷静期内允许 in_progress -> rejected 撤回（可恢复、全程审计）；
	//   3. 冷静期结束前禁止 in_progress -> fulfilled（处置不可逆）。
	if r.State == StateInProgress && target == StateRejected {
		if r.RequestType != RequestTypeErase {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"withdrawal during cooling is only allowed for erase requests",
				map[string]any{"field": "requestType", "value": r.RequestType})
		}
		if !r.InCooling(now) {
			return apperrors.NewConflict("cooling window has ended; erasure disposal is irreversible")
		}
	}
	if r.RequestType == RequestTypeErase && target == StateFulfilled && r.State == StateInProgress {
		if r.CoolingUntil == nil {
			return apperrors.NewConflict("erase request has no cooling window")
		}
		if now.Before(*r.CoolingUntil) {
			return apperrors.NewConflict("cooling window is still open; erasure cannot be disposed yet")
		}
	}
	if !r.CanTransitionTo(target) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"cannot transition dsar request from "+r.State+" to "+target,
			map[string]any{"field": "state", "value": r.State, "transition": target, "allowed": r.AllowedTransitions()})
	}
	if target == StateRejected {
		if reason == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "rejectionReason is required for rejected requests",
				map[string]any{"field": "rejectionReason"})
		}
		reasonCopy := reason
		r.RejectionReason = &reasonCopy
	}
	if r.RequestType == RequestTypeErase && target == StateInProgress && r.CoolingUntil == nil {
		cooling := now.Add(CoolingWindow)
		r.CoolingUntil = &cooling
	}
	r.State = target
	r.UpdatedAt = now
	r.Revision++
	if r.IsTerminal() {
		closedAt := now
		r.ClosedAt = &closedAt
	}
	if err := r.Validate(); err != nil {
		return err
	}
	return nil
}

func (r *DsarRequest) CanTransitionTo(target string) bool {
	return contains(transitions[r.State], target)
}

func (r *DsarRequest) AllowedTransitions() []string {
	allowed := transitions[r.State]
	return append([]string{}, allowed...)
}

func (r *DsarRequest) IsTerminal() bool {
	return r.State == StateFulfilled || r.State == StateRejected
}

// IsOverdue 表示已过 SLA 且仍未终结；超期只触发告警，不改变 HTTP 语义。
func (r *DsarRequest) IsOverdue(now time.Time) bool {
	return !r.IsTerminal() && now.After(r.SLADueAt)
}

// DsarEvent 是不可变的状态迁移证据（dsar_events 只追加）。
type DsarEvent struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	DsarID         string         `json:"dsarId"`
	Action         string         `json:"action"`
	ActorID        *string        `json:"actorId,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
}

// NewDsarEvent 记录一次迁移；action 与状态机动作同名（created/verifying/in_progress/fulfilled/rejected）。
func NewDsarEvent(id, organizationID, dsarID, action string, actorID *string, payload map[string]any, now time.Time) (*DsarEvent, error) {
	switch action {
	case "created", StateVerifying, StateInProgress, StateFulfilled, StateRejected:
	default:
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid dsar event action",
			map[string]any{"field": "action", "value": action})
	}
	return &DsarEvent{
		ID: id, OrganizationID: organizationID, DsarID: dsarID, Action: action,
		ActorID: actorID, Payload: payload, CreatedAt: now,
	}, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

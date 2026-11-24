package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/domain/notifications"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DecisionAction mirrors the DB CHECK constraint risk_decisions_decision.
type DecisionAction string

const (
	DecisionAllow     DecisionAction = "allow"
	DecisionChallenge DecisionAction = "challenge"
	DecisionDeny      DecisionAction = "deny"
)

// GovernanceAction mirrors the DB CHECK constraint session_governance_action.
type GovernanceAction string

const (
	GovernanceMonitor GovernanceAction = "monitor"
	GovernanceStepUp  GovernanceAction = "step_up"
	GovernanceRevoke  GovernanceAction = "revoke"
)

// Signal is an observed risk signal attached to an identity event.
type Signal struct {
	ID             string
	OrganizationID string
	UserID         *string
	SignalType     string
	SignalValue    json.RawMessage
	IP             *string
	CreatedAt      time.Time
}

// ReviewStatus is the manual-review lifecycle of a risk decision.
type ReviewStatus string

const (
	ReviewPending   ReviewStatus = "pending"
	ReviewCompleted ReviewStatus = "completed"
)

// AppealStatus is the false-positive appeal lifecycle of a risk decision.
type AppealStatus string

const (
	AppealNone     AppealStatus = "none"
	AppealPending  AppealStatus = "pending"
	AppealAccepted AppealStatus = "accepted"
	AppealRejected AppealStatus = "rejected"
)

// The three closed sets below are the single source for the values the API
// accepts (手册 4.14.3). They feed both the predicate helpers and the 422
// metadata (field, value, allowed, set), so the message and the check can no
// longer disagree. The console mirrors them in
// web/src/shared/constants/enums.ts via @enumset.
var (
	DecisionActions = enumset.NewSet("riskDecisionAction",
		string(DecisionAllow), string(DecisionChallenge), string(DecisionDeny))
	GovernanceActions = enumset.NewSet("riskGovernanceAction",
		string(GovernanceMonitor), string(GovernanceStepUp), string(GovernanceRevoke))
	AppealResolutions = enumset.NewSet("riskAppealResolution",
		string(AppealAccepted), string(AppealRejected))
)

// DecisionWorkflowEvent is one immutable workflow history entry. At most three
// entries are stored per decision: one review, one appeal and one resolution.
type DecisionWorkflowEvent struct {
	Revision     int64          `json:"revision"`
	Type         string         `json:"type"`
	ActorID      string         `json:"actorId"`
	Reason       string         `json:"reason"`
	FromDecision DecisionAction `json:"fromDecision,omitempty"`
	ToDecision   DecisionAction `json:"toDecision,omitempty"`
	AppealStatus AppealStatus   `json:"appealStatus,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
}

type DecisionReview struct {
	Status       ReviewStatus   `json:"status"`
	ActorID      string         `json:"actorId,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	FromDecision DecisionAction `json:"fromDecision,omitempty"`
	ToDecision   DecisionAction `json:"toDecision,omitempty"`
	ReviewedAt   *time.Time     `json:"reviewedAt,omitempty"`
}

type DecisionAppeal struct {
	Status           AppealStatus `json:"status"`
	ActorID          string       `json:"actorId,omitempty"`
	Reason           string       `json:"reason,omitempty"`
	CreatedAt        *time.Time   `json:"createdAt,omitempty"`
	ResolvedBy       string       `json:"resolvedBy,omitempty"`
	ResolutionReason string       `json:"resolutionReason,omitempty"`
	ResolvedAt       *time.Time   `json:"resolvedAt,omitempty"`
}

// DecisionWorkflow is versioned in the existing risk_decisions.factors JSONB
// column, so optimistic concurrency and history do not require schema drift.
type DecisionWorkflow struct {
	Revision int64                   `json:"revision"`
	Review   DecisionReview          `json:"review"`
	Appeal   DecisionAppeal          `json:"appeal"`
	History  []DecisionWorkflowEvent `json:"history"`
}

func NewDecisionWorkflow() DecisionWorkflow {
	return DecisionWorkflow{
		Review:  DecisionReview{Status: ReviewPending},
		Appeal:  DecisionAppeal{Status: AppealNone},
		History: make([]DecisionWorkflowEvent, 0),
	}
}

func IsDecisionActionValid(action DecisionAction) bool {
	return DecisionActions.Contains(string(action))
}

func IsGovernanceActionValid(action GovernanceAction) bool {
	return GovernanceActions.Contains(string(action))
}

func decisionStrength(action DecisionAction) int {
	switch action {
	case DecisionAllow:
		return 0
	case DecisionChallenge:
		return 1
	case DecisionDeny:
		return 2
	default:
		return -1
	}
}

// ApplyReview permits one manual review and never weakens the enforcement
// already selected by the engine. An accepted appeal is deliberately separate:
// it cannot become a one-click bypass for a critical/static policy.
func (w *DecisionWorkflow) ApplyReview(current, next DecisionAction, actorID, reason string, now time.Time) error {
	if w.Review.Status != ReviewPending {
		return apperrors.NewConflict("risk decision review is already completed")
	}
	if err := DecisionActions.ValidateRequired("decision", string(next)); err != nil {
		return err
	}
	if decisionStrength(next) < decisionStrength(current) {
		return apperrors.NewConflict("risk decision review cannot weaken enforcement")
	}
	at := now.UTC()
	w.Revision++
	w.Review = DecisionReview{
		Status: ReviewCompleted, ActorID: actorID, Reason: reason,
		FromDecision: current, ToDecision: next, ReviewedAt: &at,
	}
	w.History = append(w.History, DecisionWorkflowEvent{
		Revision: w.Revision, Type: "decision.reviewed", ActorID: actorID, Reason: reason,
		FromDecision: current, ToDecision: next, CreatedAt: at,
	})
	return nil
}

func (w *DecisionWorkflow) SubmitAppeal(current DecisionAction, actorID, reason string, now time.Time) error {
	if w.Appeal.Status != AppealNone {
		return apperrors.NewConflict("risk decision appeal already exists")
	}
	if current != DecisionChallenge && current != DecisionDeny {
		return apperrors.NewConflict("only challenge or deny decisions can be appealed")
	}
	at := now.UTC()
	w.Revision++
	w.Appeal = DecisionAppeal{
		Status: AppealPending, ActorID: actorID, Reason: reason, CreatedAt: &at,
	}
	w.History = append(w.History, DecisionWorkflowEvent{
		Revision: w.Revision, Type: "appeal.submitted", ActorID: actorID, Reason: reason,
		AppealStatus: AppealPending, CreatedAt: at,
	})
	return nil
}

func (w *DecisionWorkflow) ResolveAppeal(next AppealStatus, actorID, reason string, now time.Time) error {
	if err := AppealResolutions.ValidateRequired("resolution", string(next)); err != nil {
		return err
	}
	if w.Appeal.Status != AppealPending {
		return apperrors.NewConflict("only a pending risk decision appeal can be resolved")
	}
	at := now.UTC()
	w.Revision++
	w.Appeal.Status = next
	w.Appeal.ResolvedBy = actorID
	w.Appeal.ResolutionReason = reason
	w.Appeal.ResolvedAt = &at
	w.History = append(w.History, DecisionWorkflowEvent{
		Revision: w.Revision, Type: "appeal.resolved", ActorID: actorID, Reason: reason,
		AppealStatus: next, CreatedAt: at,
	})
	return nil
}

func ValidateGovernanceTransition(current, next GovernanceAction) error {
	if err := GovernanceActions.ValidateRequired("action", string(next)); err != nil {
		return err
	}
	if current == next {
		return apperrors.NewConflict("session governance transition must change the action")
	}
	switch current {
	case GovernanceMonitor:
		if next == GovernanceStepUp || next == GovernanceRevoke {
			return nil
		}
	case GovernanceStepUp:
		if next == GovernanceMonitor || next == GovernanceRevoke {
			return nil
		}
	case GovernanceRevoke:
		return apperrors.NewConflict("revoked session governance is terminal")
	default:
		return apperrors.NewConflict("unknown current session governance action")
	}
	return apperrors.NewConflictf("session governance cannot transition from %s to %s", current, next)
}

// Decision is the risk engine's assessment for an event — score, decision
// (allow / challenge / deny), and contributing factors.
type Decision struct {
	ID             string
	OrganizationID string
	UserID         *string
	Score          int
	Decision       DecisionAction
	Factors        json.RawMessage
	CreatedAt      time.Time
}

const decisionWorkflowMarker = "guliangiam.risk_decision.workflow.v1"

type decisionWorkflowEnvelope struct {
	Marker        string           `json:"_guliangiam"`
	EngineFactors json.RawMessage  `json:"engineFactors"`
	Workflow      DecisionWorkflow `json:"workflow"`
}

// DecodeDecisionFactors separates engine-owned factors from the reserved
// workflow envelope. Legacy and engine-written JSON values pass through intact.
func DecodeDecisionFactors(raw json.RawMessage) (json.RawMessage, DecisionWorkflow, error) {
	workflow := NewDecisionWorkflow()
	if len(raw) == 0 {
		raw = json.RawMessage(`[]`)
	}
	if !json.Valid(raw) {
		return nil, workflow, fmt.Errorf("invalid risk decision factors JSON")
	}
	item, ok := singleWorkflowItem(raw)
	if !ok {
		// Not a workflow envelope — legacy and engine-written JSON pass through.
		return raw, workflow, nil
	}
	var envelope decisionWorkflowEnvelope
	if err := json.Unmarshal(item, &envelope); err != nil {
		return nil, workflow, fmt.Errorf("decode risk decision workflow: %w", err)
	}
	if len(envelope.EngineFactors) == 0 || !json.Valid(envelope.EngineFactors) {
		return nil, workflow, fmt.Errorf("invalid engine factors in risk decision workflow")
	}
	if envelope.Workflow.History == nil {
		envelope.Workflow.History = make([]DecisionWorkflowEvent, 0)
	}
	if err := envelope.Workflow.validate(); err != nil {
		return nil, workflow, err
	}
	return envelope.EngineFactors, envelope.Workflow, nil
}

// singleWorkflowItem reports whether raw is a one-element JSON array whose
// element carries the workflow marker, and returns that element. Every other
// shape (legacy factor arrays, engine-written objects) is not an envelope.
// The boolean result keeps "not an envelope" separate from decode errors.
func singleWorkflowItem(raw json.RawMessage) (json.RawMessage, bool) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, false
	}
	if len(items) != 1 {
		return nil, false
	}
	var probe struct {
		Marker string `json:"_guliangiam"`
	}
	if err := json.Unmarshal(items[0], &probe); err != nil {
		return nil, false
	}
	if probe.Marker != decisionWorkflowMarker {
		return nil, false
	}
	return items[0], true
}

func EncodeDecisionFactors(engineFactors json.RawMessage, workflow DecisionWorkflow) (json.RawMessage, error) {
	if len(engineFactors) == 0 {
		engineFactors = json.RawMessage(`[]`)
	}
	if !json.Valid(engineFactors) {
		return nil, fmt.Errorf("invalid engine factors JSON")
	}
	if err := workflow.validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal([]decisionWorkflowEnvelope{{
		Marker: decisionWorkflowMarker, EngineFactors: engineFactors, Workflow: workflow,
	}})
	if err != nil {
		return nil, fmt.Errorf("encode risk decision workflow: %w", err)
	}
	return encoded, nil
}

func (w DecisionWorkflow) validate() error {
	if w.Revision < 0 || w.Revision != int64(len(w.History)) {
		return fmt.Errorf("invalid risk decision workflow revision")
	}
	if w.Review.Status != ReviewPending && w.Review.Status != ReviewCompleted {
		return fmt.Errorf("invalid risk decision review status")
	}
	switch w.Appeal.Status {
	case AppealNone, AppealPending, AppealAccepted, AppealRejected:
	default:
		return fmt.Errorf("invalid risk decision appeal status")
	}
	for i := range w.History {
		if w.History[i].Revision != int64(i+1) {
			return fmt.Errorf("invalid risk decision workflow history revision")
		}
	}
	return nil
}

// SessionGovernance is an enforcement action applied to a live session.
// Revision is the one-based event sequence derived from existing history rows.
type SessionGovernance struct {
	ID             string
	OrganizationID string
	SessionID      *string
	RiskScore      int
	Action         GovernanceAction
	Reason         *string
	CreatedAt      time.Time
	Revision       int64
}

// ListSignalsQuery filters paginated risk signals.
type ListSignalsQuery struct {
	OrganizationID string
	UserID         *string
	SignalType     *string
	Page           int
	PageSize       int
}

// ListDecisionsQuery filters paginated risk decisions.
type ListDecisionsQuery struct {
	OrganizationID string
	UserID         *string
	Decision       *DecisionAction
	Page           int
	PageSize       int
}

type CommandMetadata struct {
	ActorID   string
	IPAddress string
	UserAgent string
	Surface   string
	RequestID string
	TraceID   string
}

// ReviewDecisionCommand records a manual override on a risk decision.
type ReviewDecisionCommand struct {
	OrganizationID   string
	DecisionID       string
	NewDecision      DecisionAction
	Reason           string
	ExpectedRevision *int64
	Metadata         CommandMetadata
}

type SubmitAppealCommand struct {
	OrganizationID   string
	DecisionID       string
	Reason           string
	ExpectedRevision *int64
	Metadata         CommandMetadata
}

type ResolveAppealCommand struct {
	OrganizationID   string
	DecisionID       string
	Resolution       AppealStatus
	Reason           string
	ExpectedRevision *int64
	Metadata         CommandMetadata
}

// UpdateDecisionCommand is a compare-and-swap on all engine-owned mutable
// fields. ExpectedFactors prevents two workflow writers from losing a revision.
type UpdateDecisionCommand struct {
	OrganizationID   string
	DecisionID       string
	ExpectedDecision DecisionAction
	ExpectedFactors  json.RawMessage
	NewDecision      DecisionAction
	NewFactors       json.RawMessage
}

// ListGovernanceQuery filters session governance records.
type ListGovernanceQuery struct {
	OrganizationID string
	Action         *GovernanceAction
	Page           int
	PageSize       int
}

type TransitionGovernanceCommand struct {
	OrganizationID   string
	GovernanceID     string
	NewAction        GovernanceAction
	Reason           string
	ExpectedRevision *int64
	Metadata         CommandMetadata
}

type AuditIntegrityQuery struct {
	OrganizationID string
	ChainID        string
}

type AuditChainRecord struct {
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     string
	IP             string
	UserAgent      string
	Surface        string
	RequestID      string
	TraceID        string
	Metadata       map[string]any
	CreatedAt      time.Time
	ChainSeq       int64
	PrevHash       []byte
	EntryHash      []byte
	ChainAlgo      string
}

type AuditChain struct {
	Entries []AuditChainRecord
	Anchors []AuditChainAnchor
}

type AuditChainAnchor struct {
	AnchorSeq int64
	EntryHash []byte
}

// ── Risk engine evaluation entry point ───────────────────────────────

// LoginChannel identifies which login path triggered the evaluation.
type LoginChannel string

const (
	ChannelPassword   LoginChannel = "password"
	ChannelOIDC       LoginChannel = "oidc"
	ChannelOAuth2     LoginChannel = "oauth2"
	ChannelSAML       LoginChannel = "saml"
	ChannelCAS        LoginChannel = "cas"
	ChannelWebAuthn   LoginChannel = "webauthn"
	ChannelWeb3       LoginChannel = "web3"
	ChannelQRCode     LoginChannel = "qrcode"
	ChannelDeviceFlow LoginChannel = "device_flow"
	ChannelFaceID     LoginChannel = "faceid"
)

// EvaluateRiskCommand is the engine input called before session creation.
type EvaluateRiskCommand struct {
	OrganizationID string
	UserID         *string
	LoginChannel   LoginChannel
	IP             *string
	UserAgent      string
	// Dynamic signals collected by the caller: geo, device fingerprint,
	// recent failure count, time-of-day, etc.
	Signals map[string]any
}

// EvaluateRiskResult is the engine output persisted as a Decision.
type EvaluateRiskResult struct {
	DecisionID string
	Decision   DecisionAction
	Score      int
	Factors    json.RawMessage
}

// RuleEngine is the strategy interface so that a rule-based engine (the
// baseline) and a future ML-backed engine can coexist behind one port.
type RuleEngine interface {
	Evaluate(ctx context.Context, cmd EvaluateRiskCommand) EvaluateRiskResult
}

// ── Cross-app port: login-time risk evaluation ──────────────────────────
//
// LoginRiskEvaluator is the minimal port that auth (and every other login
// surface) calls before granting a session. Defined in domain so both the
// risk engine and the auth layer can depend on it without a cycle.

type LoginRiskCommand struct {
	OrganizationID string
	UserID         *string
	LoginChannel   string
	IP             *string
	UserAgent      string
	Signals        map[string]any
}

type LoginRiskResult struct {
	DecisionID string
	Decision   string // "allow" | "challenge" | "deny"
	Score      int
}

// LoginRiskTx is the narrow slice of the transaction the risk engine needs to
// write its evidence atomically: the decision/signal rows, the audit chain, and
// the critical-alert notifications. Structural — the app-layer UnitOfWork
// satisfies it, but domain/risk cannot import domain/uow (that package already
// imports domain/risk, so a shared interface would be a cycle).
//
// Passing the caller's transaction is what keeps "decision + audit + notify"
// in ONE transaction. Opening a second transaction here would silently split
// the login write path across two connections.
type LoginRiskTx interface {
	Risk() Repository
	Users() users.Repository
	Notifications() notifications.Repository
	NotificationProviders() notificationproviders.Repository
}

// LoginRiskEvaluator is the port the auth layer calls before session creation.
// It records the decision through the caller's transaction so the risk
// evidence and the session either commit together or not at all.
// nil → every login is treated as "allow" (backward-compatible fallback).
type LoginRiskEvaluator interface {
	EvaluateAndRecordLoginRisk(ctx context.Context, tx LoginRiskTx, cmd LoginRiskCommand) (LoginRiskResult, error)
}

// The risk → session revocation port lives in the app layer
// (app/risk.SessionRevoker), not here: its implementation needs the full
// transaction to keep the governance row and the kill atomic, and domain/risk
// cannot import domain/uow (that package already imports this one).

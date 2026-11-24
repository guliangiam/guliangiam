package risk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformaudit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// Service implements risk queries and the workflow enforcement that can be
// completed inside the risk domain. Session revocation itself remains owned by
// the sessions domain; governance transitions emit an outbox event for it.
type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.RiskUoW
	// SessionRevoker bridges governance revoke → actual session kill + SSF
	// outbox. It takes the caller's transaction so the governance record and
	// the kill commit together; opening a nested transaction would be rejected
	// by the MySQL unit of work. nil → revoke is pure bookkeeping (test mode).
	SessionRevoker SessionRevoker
}

// SessionRevoker is the risk domain's port to session revocation. Declared in
// the app layer (not domain/risk) because the implementation needs the full
// transaction, and domain/risk cannot import domain/uow without a cycle.
type SessionRevoker interface {
	RevokeSessionInTx(ctx context.Context, tx appuow.UnitOfWork, sessionID, trigger string) error
}

func NewService(uow appuow.UnitOfWork) Service {
	return Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uow}
}

type (
	DecisionAction   = domain.DecisionAction
	GovernanceAction = domain.GovernanceAction
	AppealStatus     = domain.AppealStatus
	DecisionWorkflow = domain.DecisionWorkflow

	ListSignalsQuery            = domain.ListSignalsQuery
	ListDecisionsQuery          = domain.ListDecisionsQuery
	ListGovernanceQuery         = domain.ListGovernanceQuery
	ReviewDecisionCommand       = domain.ReviewDecisionCommand
	SubmitAppealCommand         = domain.SubmitAppealCommand
	ResolveAppealCommand        = domain.ResolveAppealCommand
	TransitionGovernanceCommand = domain.TransitionGovernanceCommand
	AuditIntegrityQuery         = domain.AuditIntegrityQuery
	CommandMetadata             = domain.CommandMetadata
)

const (
	AppealAccepted = domain.AppealAccepted
	AppealRejected = domain.AppealRejected
)

type SignalDTO struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	UserID         *string         `json:"userId,omitempty"`
	SignalType     string          `json:"signalType"`
	SignalValue    json.RawMessage `json:"signalValue"`
	IP             *string         `json:"ip,omitempty"`
	CreatedAt      string          `json:"createdAt"`
}

// ReportSignalCommand is the operator-facing manual signal write. The
// organization comes from the trusted tenant context, never from the body.
type ReportSignalCommand struct {
	OrganizationID string
	UserID         *string
	SignalType     string
	SignalValue    json.RawMessage
	IP             *string
	Metadata       CommandMetadata
}

type DecisionDTO struct {
	ID             string           `json:"id"`
	OrganizationID string           `json:"organizationId"`
	UserID         *string          `json:"userId,omitempty"`
	Score          int              `json:"score"`
	Decision       string           `json:"decision"`
	Factors        json.RawMessage  `json:"factors"`
	CreatedAt      string           `json:"createdAt"`
	Revision       int64            `json:"revision"`
	Workflow       DecisionWorkflow `json:"workflow"`
}

type SessionGovernanceDTO struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	SessionID      *string `json:"sessionId,omitempty"`
	RiskScore      int     `json:"riskScore"`
	Action         string  `json:"action"`
	Reason         *string `json:"reason,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	Revision       int64   `json:"revision"`
}

type AuditIntegrityFailure struct {
	ChainSeq int64  `json:"chainSeq"`
	Reason   string `json:"reason"`
}

type AuditIntegrityDTO struct {
	OrganizationID string                 `json:"organizationId"`
	ChainID        string                 `json:"chainId"`
	Status         string                 `json:"status"`
	Valid          bool                   `json:"valid"`
	TotalCount     int                    `json:"totalCount"`
	VerifiedCount  int                    `json:"verifiedCount"`
	PendingCount   int                    `json:"pendingCount"`
	Failure        *AuditIntegrityFailure `json:"failure,omitempty"`
	VerifiedAt     string                 `json:"verifiedAt"`
}

func validateDecisionWrite(orgID, decisionID, reason string, expectedRevision *int64, metadata CommandMetadata) error {
	if err := validateUUID("organizationId", orgID); err != nil {
		return err
	}
	if err := validateUUID(keyDecisionID, decisionID); err != nil {
		return err
	}
	if err := validateReason(reason); err != nil {
		return err
	}
	if err := validateExpectedRevision(expectedRevision); err != nil {
		return err
	}
	return validateUUID("actorId", strings.TrimSpace(metadata.ActorID))
}

func requireRevision(expected, current int64) error {
	if expected != current {
		return apperrors.NewConflict("risk resource revision mismatch")
	}
	return nil
}

func normalizeMetadata(metadata CommandMetadata) CommandMetadata {
	metadata.ActorID = strings.TrimSpace(metadata.ActorID)
	metadata.IPAddress = strings.TrimSpace(metadata.IPAddress)
	metadata.Surface = strings.TrimSpace(metadata.Surface)
	metadata.RequestID = strings.TrimSpace(metadata.RequestID)
	metadata.TraceID = strings.TrimSpace(metadata.TraceID)
	return metadata
}

func normalizeReportSignal(cmd *ReportSignalCommand) {
	cmd.OrganizationID = strings.TrimSpace(cmd.OrganizationID)
	cmd.SignalType = strings.TrimSpace(cmd.SignalType)
	cmd.Metadata = normalizeMetadata(cmd.Metadata)
	if cmd.UserID != nil {
		value := strings.TrimSpace(*cmd.UserID)
		if value == "" {
			cmd.UserID = nil
		} else {
			cmd.UserID = &value
		}
	}
	if cmd.IP != nil {
		value := strings.TrimSpace(*cmd.IP)
		if value == "" {
			cmd.IP = nil
		} else {
			cmd.IP = &value
		}
	}
	if len(cmd.SignalValue) == 0 {
		cmd.SignalValue = json.RawMessage(`{}`)
	}
}

func validateReportSignal(cmd ReportSignalCommand) error {
	if err := validateUUID("organizationId", cmd.OrganizationID); err != nil {
		return err
	}
	if cmd.SignalType == "" {
		return apperrors.NewInvalidArgument("signalType is required")
	}
	if err := validateSignalType(cmd.SignalType); err != nil {
		return err
	}
	if cmd.UserID != nil {
		if err := validateUUID("userId", *cmd.UserID); err != nil {
			return err
		}
	}
	if !json.Valid(cmd.SignalValue) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"signalValue must be valid JSON",
			map[string]any{"field": "signalValue", "format": "json"},
		)
	}
	if cmd.IP != nil && net.ParseIP(*cmd.IP) == nil {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"ip must be a valid IP address",
			map[string]any{"field": "ip", "value": *cmd.IP, "format": "inet"},
		)
	}
	return validateUUID("actorId", cmd.Metadata.ActorID)
}

func (s *Service) ListSignals(ctx context.Context, q ListSignalsQuery) ([]SignalDTO, int64, error) {
	q.OrganizationID = strings.TrimSpace(q.OrganizationID)
	if err := validateListQuery(q.OrganizationID, q.Page, q.PageSize); err != nil {
		return nil, 0, err
	}
	if q.UserID != nil {
		userID := strings.TrimSpace(*q.UserID)
		if userID == "" {
			q.UserID = nil
		} else {
			if err := validateUUID("userId", userID); err != nil {
				return nil, 0, err
			}
			q.UserID = &userID
		}
	}
	if q.SignalType != nil {
		signalType := strings.TrimSpace(*q.SignalType)
		if signalType == "" {
			q.SignalType = nil
		} else {
			if err := validateSignalType(signalType); err != nil {
				return nil, 0, err
			}
			q.SignalType = &signalType
		}
	}
	items, total, err := s.UOW.Risk().ListSignals(ctx, q)
	if err != nil {
		slog.ErrorContext(ctx, "risk.ListSignals failed", "organization_id", q.OrganizationID, "error", err)
		return nil, 0, err
	}
	dtos := make([]SignalDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toSignalDTO(it))
	}
	return dtos, total, nil
}

func (s *Service) ListDecisions(ctx context.Context, q ListDecisionsQuery) ([]DecisionDTO, int64, error) {
	q.OrganizationID = strings.TrimSpace(q.OrganizationID)
	if err := validateListQuery(q.OrganizationID, q.Page, q.PageSize); err != nil {
		return nil, 0, err
	}
	if q.UserID != nil {
		userID := strings.TrimSpace(*q.UserID)
		if userID == "" {
			q.UserID = nil
		} else {
			if err := validateUUID("userId", userID); err != nil {
				return nil, 0, err
			}
			q.UserID = &userID
		}
	}
	if err := validateDecisionFilter(q.Decision); err != nil {
		return nil, 0, err
	}
	items, total, err := s.UOW.Risk().ListDecisions(ctx, q)
	if err != nil {
		slog.ErrorContext(ctx, "risk.ListDecisions failed", "organization_id", q.OrganizationID, "error", err)
		return nil, 0, err
	}
	dtos := make([]DecisionDTO, 0, len(items))
	for _, it := range items {
		dto, err := toDecisionDTO(it)
		if err != nil {
			return nil, 0, apperrors.WrapInternal(err, "decode risk decision workflow")
		}
		dtos = append(dtos, dto)
	}
	return dtos, total, nil
}

// GetSignal returns one manually-or-engine recorded signal. The read is an
// optional repository capability; backends without it report 501 instead of
// pretending the row does not exist.
func (s *Service) GetSignal(ctx context.Context, organizationID, signalID string) (*SignalDTO, error) {
	organizationID = strings.TrimSpace(organizationID)
	signalID = strings.TrimSpace(signalID)
	if err := validateUUID("organizationId", organizationID); err != nil {
		return nil, err
	}
	if err := validateUUID("signalId", signalID); err != nil {
		return nil, err
	}
	reader, ok := s.UOW.Risk().(domain.SignalReader)
	if !ok {
		return nil, apperrors.NewUnimplemented("risk signal detail is not supported by this storage backend")
	}
	signal, err := reader.GetSignal(ctx, signalID, organizationID)
	if err != nil {
		return nil, err
	}
	dto := toSignalDTO(signal)
	return &dto, nil
}

// ReportSignal records an operator-supplied signal with the same audit/outbox
// evidence as engine-generated signals. Signal and audit commit together.
func (s *Service) ReportSignal(ctx context.Context, cmd ReportSignalCommand) (*SignalDTO, error) {
	normalizeReportSignal(&cmd)
	if err := validateReportSignal(cmd); err != nil {
		return nil, err
	}
	signalID, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate signal id")
	}
	signal := domain.Signal{
		ID: signalID, OrganizationID: cmd.OrganizationID, UserID: cmd.UserID,
		SignalType: cmd.SignalType, SignalValue: cmd.SignalValue, IP: cmd.IP,
		CreatedAt: s.Clock.Now().UTC(),
	}
	var dto SignalDTO
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Risk().AppendSignal(ctx, signal); err != nil {
			return err
		}
		dto = toSignalDTO(&signal)
		payload := map[string]any{
			"signalId": signalID, keyOrgID: cmd.OrganizationID, "signalType": cmd.SignalType,
		}
		if cmd.UserID != nil {
			payload["userId"] = *cmd.UserID
		}
		return s.embedAuditEvent(
			tx, ctx,
			cmd.Metadata.IPAddress, cmd.Metadata.UserAgent, cmd.Metadata.Surface, cmd.Metadata.RequestID, cmd.Metadata.TraceID,
			cmd.OrganizationID, cmd.Metadata.ActorID,
			"risk.signal.reported", "risk_signal", signalID,
			"risk_signal", "risk.signal.reported",
			payload,
		)
	})
	if err != nil {
		slog.ErrorContext(ctx, "risk.ReportSignal failed", "signal_id", signalID, "organization_id", cmd.OrganizationID, "error", err)
		return nil, err
	}
	return &dto, nil
}

func (s *Service) GetDecision(ctx context.Context, organizationID, decisionID string) (*DecisionDTO, error) {
	organizationID = strings.TrimSpace(organizationID)
	decisionID = strings.TrimSpace(decisionID)
	if err := validateUUID("organizationId", organizationID); err != nil {
		return nil, err
	}
	if err := validateUUID(keyDecisionID, decisionID); err != nil {
		return nil, err
	}
	decision, err := s.UOW.Risk().GetDecision(ctx, decisionID, organizationID)
	if err != nil {
		return nil, err
	}
	dto, err := toDecisionDTO(decision)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "decode risk decision workflow")
	}
	return &dto, nil
}

func (s *Service) ReviewDecision(ctx context.Context, cmd ReviewDecisionCommand) (*DecisionDTO, error) {
	cmd.OrganizationID = strings.TrimSpace(cmd.OrganizationID)
	cmd.DecisionID = strings.TrimSpace(cmd.DecisionID)
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	cmd.Metadata = normalizeMetadata(cmd.Metadata)
	if err := validateDecisionWrite(cmd.OrganizationID, cmd.DecisionID, cmd.Reason, cmd.ExpectedRevision, cmd.Metadata); err != nil {
		return nil, err
	}
	if err := validateNewDecision(cmd.NewDecision); err != nil {
		return nil, err
	}

	var updated *DecisionDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		current, err := tx.Risk().GetDecisionForUpdate(ctx, cmd.DecisionID, cmd.OrganizationID)
		if err != nil {
			return err
		}
		engineFactors, workflow, err := domain.DecodeDecisionFactors(current.Factors)
		if err != nil {
			return apperrors.WrapInternal(err, "decode risk decision workflow")
		}
		if err := requireRevision(*cmd.ExpectedRevision, workflow.Revision); err != nil {
			return err
		}
		previous := current.Decision
		if err := workflow.ApplyReview(previous, cmd.NewDecision, cmd.Metadata.ActorID, cmd.Reason, s.Clock.Now()); err != nil {
			return err
		}
		newFactors, err := domain.EncodeDecisionFactors(engineFactors, workflow)
		if err != nil {
			return apperrors.WrapInternal(err, "encode risk decision workflow")
		}
		if err := tx.Risk().UpdateDecision(ctx, domain.UpdateDecisionCommand{
			OrganizationID: cmd.OrganizationID, DecisionID: cmd.DecisionID,
			ExpectedDecision: previous, ExpectedFactors: current.Factors,
			NewDecision: cmd.NewDecision, NewFactors: newFactors,
		}); err != nil {
			return err
		}
		after := *current
		after.Decision = cmd.NewDecision
		after.Factors = newFactors
		dto, err := toDecisionDTO(&after)
		if err != nil {
			return err
		}
		updated = &dto
		return s.embedAuditEvent(
			tx, ctx,
			cmd.Metadata.IPAddress, cmd.Metadata.UserAgent, cmd.Metadata.Surface, cmd.Metadata.RequestID, cmd.Metadata.TraceID,
			cmd.OrganizationID, cmd.Metadata.ActorID,
			"risk.decision.reviewed", "decision", cmd.DecisionID,
			"decision", "risk.decision.reviewed",
			map[string]any{
				keyDecisionID: cmd.DecisionID, keyOrgID: cmd.OrganizationID,
				"fromDecision": previous, "toDecision": cmd.NewDecision, keyRevision: workflow.Revision,
			},
		)
	})
	if err != nil {
		slog.ErrorContext(ctx, "risk.ReviewDecision failed", "decision_id", cmd.DecisionID, "organization_id", cmd.OrganizationID, "error", err)
		return nil, err
	}
	return updated, nil
}

func (s *Service) SubmitAppeal(ctx context.Context, cmd SubmitAppealCommand) (*DecisionDTO, error) {
	cmd.OrganizationID = strings.TrimSpace(cmd.OrganizationID)
	cmd.DecisionID = strings.TrimSpace(cmd.DecisionID)
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	cmd.Metadata = normalizeMetadata(cmd.Metadata)
	if err := validateDecisionWrite(cmd.OrganizationID, cmd.DecisionID, cmd.Reason, cmd.ExpectedRevision, cmd.Metadata); err != nil {
		return nil, err
	}

	var updated *DecisionDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		current, err := tx.Risk().GetDecisionForUpdate(ctx, cmd.DecisionID, cmd.OrganizationID)
		if err != nil {
			return err
		}
		engineFactors, workflow, err := domain.DecodeDecisionFactors(current.Factors)
		if err != nil {
			return apperrors.WrapInternal(err, "decode risk decision workflow")
		}
		if err := requireRevision(*cmd.ExpectedRevision, workflow.Revision); err != nil {
			return err
		}
		if err := workflow.SubmitAppeal(current.Decision, cmd.Metadata.ActorID, cmd.Reason, s.Clock.Now()); err != nil {
			return err
		}
		newFactors, err := domain.EncodeDecisionFactors(engineFactors, workflow)
		if err != nil {
			return apperrors.WrapInternal(err, "encode risk decision workflow")
		}
		if err := tx.Risk().UpdateDecision(ctx, domain.UpdateDecisionCommand{
			OrganizationID: cmd.OrganizationID, DecisionID: cmd.DecisionID,
			ExpectedDecision: current.Decision, ExpectedFactors: current.Factors,
			NewDecision: current.Decision, NewFactors: newFactors,
		}); err != nil {
			return err
		}
		after := *current
		after.Factors = newFactors
		dto, err := toDecisionDTO(&after)
		if err != nil {
			return err
		}
		updated = &dto
		return s.embedAuditEvent(
			tx, ctx,
			cmd.Metadata.IPAddress, cmd.Metadata.UserAgent, cmd.Metadata.Surface, cmd.Metadata.RequestID, cmd.Metadata.TraceID,
			cmd.OrganizationID, cmd.Metadata.ActorID,
			"risk.decision.appeal.submitted", "decision", cmd.DecisionID,
			"decision", "risk.decision.appeal.submitted",
			map[string]any{
				keyDecisionID: cmd.DecisionID, keyOrgID: cmd.OrganizationID,
				"appealStatus": domain.AppealPending, keyRevision: workflow.Revision,
			},
		)
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) ResolveAppeal(ctx context.Context, cmd ResolveAppealCommand) (*DecisionDTO, error) {
	cmd.OrganizationID = strings.TrimSpace(cmd.OrganizationID)
	cmd.DecisionID = strings.TrimSpace(cmd.DecisionID)
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	cmd.Metadata = normalizeMetadata(cmd.Metadata)
	if err := validateDecisionWrite(cmd.OrganizationID, cmd.DecisionID, cmd.Reason, cmd.ExpectedRevision, cmd.Metadata); err != nil {
		return nil, err
	}
	if err := validateAppealResolution(cmd.Resolution); err != nil {
		return nil, err
	}

	var updated *DecisionDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		current, err := tx.Risk().GetDecisionForUpdate(ctx, cmd.DecisionID, cmd.OrganizationID)
		if err != nil {
			return err
		}
		engineFactors, workflow, err := domain.DecodeDecisionFactors(current.Factors)
		if err != nil {
			return apperrors.WrapInternal(err, "decode risk decision workflow")
		}
		if err := requireRevision(*cmd.ExpectedRevision, workflow.Revision); err != nil {
			return err
		}
		if err := workflow.ResolveAppeal(cmd.Resolution, cmd.Metadata.ActorID, cmd.Reason, s.Clock.Now()); err != nil {
			return err
		}
		newFactors, err := domain.EncodeDecisionFactors(engineFactors, workflow)
		if err != nil {
			return apperrors.WrapInternal(err, "encode risk decision workflow")
		}
		if err := tx.Risk().UpdateDecision(ctx, domain.UpdateDecisionCommand{
			OrganizationID: cmd.OrganizationID, DecisionID: cmd.DecisionID,
			ExpectedDecision: current.Decision, ExpectedFactors: current.Factors,
			NewDecision: current.Decision, NewFactors: newFactors,
		}); err != nil {
			return err
		}
		after := *current
		after.Factors = newFactors
		dto, err := toDecisionDTO(&after)
		if err != nil {
			return err
		}
		updated = &dto
		action := "risk.decision.appeal.rejected"
		if cmd.Resolution == domain.AppealAccepted {
			action = "risk.decision.appeal.accepted"
		}
		return s.embedAuditEvent(
			tx, ctx,
			cmd.Metadata.IPAddress, cmd.Metadata.UserAgent, cmd.Metadata.Surface, cmd.Metadata.RequestID, cmd.Metadata.TraceID,
			cmd.OrganizationID, cmd.Metadata.ActorID,
			action, "decision", cmd.DecisionID,
			"decision", action,
			map[string]any{
				keyDecisionID: cmd.DecisionID, keyOrgID: cmd.OrganizationID,
				"appealStatus": cmd.Resolution, keyRevision: workflow.Revision,
			},
		)
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) ListGovernance(ctx context.Context, q ListGovernanceQuery) ([]SessionGovernanceDTO, int64, error) {
	q.OrganizationID = strings.TrimSpace(q.OrganizationID)
	if err := validateListQuery(q.OrganizationID, q.Page, q.PageSize); err != nil {
		return nil, 0, err
	}
	if err := validateGovernanceFilter(q.Action); err != nil {
		return nil, 0, err
	}
	items, total, err := s.UOW.Risk().ListGovernance(ctx, q)
	if err != nil {
		slog.ErrorContext(ctx, "risk.ListGovernance failed", "organization_id", q.OrganizationID, "error", err)
		return nil, 0, err
	}
	dtos := make([]SessionGovernanceDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toGovernanceDTO(it))
	}
	return dtos, total, nil
}

func (s *Service) GetGovernance(ctx context.Context, organizationID, governanceID string) (*SessionGovernanceDTO, error) {
	organizationID = strings.TrimSpace(organizationID)
	governanceID = strings.TrimSpace(governanceID)
	if err := validateUUID("organizationId", organizationID); err != nil {
		return nil, err
	}
	if err := validateUUID("governanceId", governanceID); err != nil {
		return nil, err
	}
	governance, err := s.UOW.Risk().GetGovernance(ctx, governanceID, organizationID)
	if err != nil {
		return nil, err
	}
	dto := toGovernanceDTO(governance)
	return &dto, nil
}

func (s *Service) TransitionGovernance(ctx context.Context, cmd TransitionGovernanceCommand) (*SessionGovernanceDTO, error) {
	cmd.OrganizationID = strings.TrimSpace(cmd.OrganizationID)
	cmd.GovernanceID = strings.TrimSpace(cmd.GovernanceID)
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	cmd.Metadata = normalizeMetadata(cmd.Metadata)
	if err := validateUUID("organizationId", cmd.OrganizationID); err != nil {
		return nil, err
	}
	if err := validateUUID("governanceId", cmd.GovernanceID); err != nil {
		return nil, err
	}
	if err := validateReason(cmd.Reason); err != nil {
		return nil, err
	}
	if err := validateExpectedRevision(cmd.ExpectedRevision); err != nil {
		return nil, err
	}
	if err := validateUUID("actorId", cmd.Metadata.ActorID); err != nil {
		return nil, err
	}
	if err := validateNewAction(cmd.NewAction); err != nil {
		return nil, err
	}

	var updated *SessionGovernanceDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		current, err := tx.Risk().GetGovernanceForTransition(ctx, cmd.GovernanceID, cmd.OrganizationID)
		if err != nil {
			return err
		}
		if err := requireRevision(*cmd.ExpectedRevision, current.Revision); err != nil {
			return err
		}
		if err := domain.ValidateGovernanceTransition(current.Action, cmd.NewAction); err != nil {
			return err
		}
		newID, err := s.UUID.NewString()
		if err != nil {
			return err
		}
		createdAt := s.Clock.Now().UTC()
		if !createdAt.After(current.CreatedAt) {
			// MySQL TIMESTAMP has second precision; preserve append order even
			// when the clock is frozen or moves behind the latest stored event.
			createdAt = current.CreatedAt.UTC().Add(time.Second)
		}
		next := domain.SessionGovernance{
			ID: newID, OrganizationID: cmd.OrganizationID, SessionID: current.SessionID,
			RiskScore: current.RiskScore, Action: cmd.NewAction, Reason: &cmd.Reason,
			CreatedAt: createdAt, Revision: current.Revision + 1,
		}
		if err := tx.Risk().AppendGovernance(ctx, next); err != nil {
			return err
		}
		// ── Governance → actual session revocation ────────────────────
		// The kill runs inside THIS transaction (repo + SSF outbox + audit on
		// the same tx), so a revoke decision and the dead session commit
		// together. Calling sessions.Service.RevokeSession here would nest a
		// second transaction, which MySQL rejects outright — leaving the
		// session alive while the governance row said "revoked".
		if cmd.NewAction == domain.GovernanceRevoke && current.SessionID != nil && s.SessionRevoker != nil {
			if err := s.SessionRevoker.RevokeSessionInTx(ctx, tx, *current.SessionID, "risk_governance"); err != nil {
				return err
			}
		}
		dto := toGovernanceDTO(&next)
		updated = &dto
		return s.embedAuditEvent(
			tx, ctx,
			cmd.Metadata.IPAddress, cmd.Metadata.UserAgent, cmd.Metadata.Surface, cmd.Metadata.RequestID, cmd.Metadata.TraceID,
			cmd.OrganizationID, cmd.Metadata.ActorID,
			"risk.session_governance.transitioned", "session_governance", next.ID,
			"session_governance", "risk.session_governance.transitioned",
			map[string]any{
				"governanceId": next.ID, "previousGovernanceId": current.ID,
				"sessionId": current.SessionID, keyOrgID: cmd.OrganizationID,
				"fromAction": current.Action, "toAction": cmd.NewAction, keyRevision: next.Revision,
			},
		)
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ── Risk engine evaluation ────────────────────────────────────────────

// ChallengeRequestedError is raised when the risk engine decides challenge.
// The auth layer catches this and triggers MFA/step-up instead of creating a session.
type ChallengeRequestedError struct {
	DecisionID string
	Reason     string
}

func (e *ChallengeRequestedError) Error() string { return "risk challenge required: " + e.Reason }

// EvaluationDeniedError is raised when the risk engine denies a login outright.
type EvaluationDeniedError struct {
	DecisionID string
	Reason     string
}

func (e *EvaluationDeniedError) Error() string { return "risk denied: " + e.Reason }

// builtinRuleEngine is the default rule-based engine. Scores accumulate
// from multiple independent signals; the final DecisionAction is derived
// from the score bracket. ML-backed engines can plug in via domain.RuleEngine.
type builtinRuleEngine struct{}

func (builtinRuleEngine) Evaluate(ctx context.Context, cmd domain.EvaluateRiskCommand) domain.EvaluateRiskResult {
	score := 0
	reasons := make([]string, 0, 4)

	// is_vpn/is_tor/geo_country/user_home_country 依赖外部 IP 情报源，
	// 无内部生产者（OPEN-API-77）；缺失键按不命中处理。内部事实信号
	// （recent_failures_1h/device_fingerprint_new）由 auth 登录链采集。
	if v, _ := cmd.Signals["is_vpn"].(bool); v {
		score += 30
		reasons = append(reasons, "vpn_exit_node")
	}
	if v, _ := cmd.Signals["is_tor"].(bool); v {
		score += 40
		reasons = append(reasons, "tor_exit_node")
	}
	if v, _ := cmd.Signals["recent_failures_1h"].(int); v >= 3 {
		score += 25
		reasons = append(reasons, "recent_login_failures")
	}
	if v, _ := cmd.Signals["device_fingerprint_new"].(bool); v {
		score += 15
		reasons = append(reasons, "unknown_device")
	}
	if home, ok := cmd.Signals["user_home_country"].(string); ok && home != "" {
		if geo, ok := cmd.Signals["geo_country"].(string); ok && geo != "" && geo != home {
			score += 20
			reasons = append(reasons, "impossible_travel")
		}
	}

	// risk_decisions.score has a CHECK (score BETWEEN 0 AND 100) in the schema.
	// Independent signals can add up to 130, so the raw sum must be clamped
	// before it is persisted — otherwise the insert fails and the login path
	// turns a deny into a 500.
	if score > 100 {
		score = 100
	}

	var action domain.DecisionAction
	switch {
	case score >= 70:
		action = domain.DecisionDeny
	case score >= 40:
		action = domain.DecisionChallenge
	default:
		action = domain.DecisionAllow
	}

	factors, _ := json.Marshal(map[string]any{
		keyScore:   score,
		"reasons":  reasons,
		keyChannel: cmd.LoginChannel,
	})

	return domain.EvaluateRiskResult{
		Score:    score,
		Decision: action,
		Factors:  factors,
	}
}

// EvaluateRisk is the standalone entry point used by API callers/tests. It
// opens its own transaction and delegates to recordRiskEvaluation, which is
// also what every login channel invokes through its own transaction.
func (s *Service) EvaluateRisk(ctx context.Context, cmd domain.EvaluateRiskCommand, metadata CommandMetadata) (domain.EvaluateRiskResult, error) {
	if err := validateEvaluateRiskCommand(cmd); err != nil {
		return domain.EvaluateRiskResult{}, err
	}
	var result domain.EvaluateRiskResult
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		recorded, err := s.recordRiskEvaluation(ctx, tx, cmd, metadata)
		if err != nil {
			return err
		}
		result = recorded
		return nil
	}); err != nil {
		slog.ErrorContext(ctx, "risk.EvaluateRisk failed",
			"organization_id", cmd.OrganizationID, "channel", cmd.LoginChannel, "error", err)
		return domain.EvaluateRiskResult{}, err
	}

	switch result.Decision {
	case domain.DecisionDeny:
		return result, &EvaluationDeniedError{DecisionID: result.DecisionID, Reason: "login blocked by risk engine"}
	case domain.DecisionChallenge:
		return result, &ChallengeRequestedError{DecisionID: result.DecisionID, Reason: "additional verification required"}
	case domain.DecisionAllow:
		// allow 无附加错误，落到 switch 后的正常返回
	}
	return result, nil
}

// EvaluateAndRecordLoginRisk satisfies domain.LoginRiskEvaluator. Unlike
// EvaluateRisk it does NOT open a transaction: the caller passes the active
// one so the risk evidence, the audit chain, and the session write commit or
// roll back together. It returns an error only for engine/DB failures; the
// allow/challenge/deny outcome is carried in the result so each channel can
// apply its own step-up semantics.
func (s *Service) EvaluateAndRecordLoginRisk(ctx context.Context, tx domain.LoginRiskTx, cmd domain.LoginRiskCommand) (domain.LoginRiskResult, error) {
	// The audit chain needs Records()+Outbox(), which the narrow domain port
	// deliberately omits (it would drag the whole UnitOfWork into domain/risk).
	// Every production transaction satisfies the full interface; a caller that
	// does not is a wiring bug, so fail loudly instead of skipping the audit.
	full, ok := tx.(appuow.UnitOfWork)
	if !ok {
		return domain.LoginRiskResult{}, apperrors.NewInternal("risk: transaction does not implement appuow.UnitOfWork")
	}

	evaluateCmd := domain.EvaluateRiskCommand{
		OrganizationID: cmd.OrganizationID,
		UserID:         cmd.UserID,
		LoginChannel:   normalizeLoginChannel(cmd.LoginChannel),
		IP:             cmd.IP,
		UserAgent:      cmd.UserAgent,
		Signals:        cmd.Signals,
	}
	if err := validateEvaluateRiskCommand(evaluateCmd); err != nil {
		return domain.LoginRiskResult{}, err
	}

	result, err := s.recordRiskEvaluation(ctx, full, evaluateCmd, loginMetadata(cmd))
	if err != nil {
		slog.ErrorContext(ctx, "risk.EvaluateAndRecordLoginRisk failed",
			"organization_id", evaluateCmd.OrganizationID, "channel", evaluateCmd.LoginChannel, "error", err)
		return domain.LoginRiskResult{}, err
	}
	return domain.LoginRiskResult{
		DecisionID: result.DecisionID,
		Decision:   string(result.Decision),
		Score:      result.Score,
	}, nil
}

// loginMetadata derives the audit metadata a login channel can supply.
func loginMetadata(cmd domain.LoginRiskCommand) CommandMetadata {
	return CommandMetadata{IPAddress: derefString(cmd.IP), UserAgent: cmd.UserAgent}
}

func normalizeLoginChannel(channel string) domain.LoginChannel {
	normalized := domain.LoginChannel(strings.ToLower(strings.TrimSpace(channel)))
	if normalized == "" {
		return domain.ChannelPassword
	}
	return normalized
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// evaluateRiskPure runs the rule engine without touching the database.
func (s *Service) evaluateRiskPure(ctx context.Context, cmd domain.EvaluateRiskCommand) domain.EvaluateRiskResult {
	engine := domain.RuleEngine(builtinRuleEngine{})
	return engine.Evaluate(ctx, cmd)
}

// recordRiskEvaluation scores the request and writes the evidence using the
// supplied transaction: signal → decision → audit → (critical) notifications.
//
// The notification step runs for deny decisions only (AC-RISK-001: critical
// rejects the login AND notifies the user and the administrators). It is
// deliberately best-effort: a missing provider or an unreachable recipient
// list must never turn a security decision into an error, because the deny
// itself is already enforced and audited.
func (s *Service) recordRiskEvaluation(ctx context.Context, tx appuow.UnitOfWork, cmd domain.EvaluateRiskCommand, metadata CommandMetadata) (domain.EvaluateRiskResult, error) {
	result := s.evaluateRiskPure(ctx, cmd)

	now := s.Clock.Now().UTC()
	signalID, err := s.UUID.NewString()
	if err != nil {
		return domain.EvaluateRiskResult{}, apperrors.WrapInternal(err, "generate signal id")
	}
	decisionID, err := s.UUID.NewString()
	if err != nil {
		return domain.EvaluateRiskResult{}, apperrors.WrapInternal(err, "generate decision id")
	}
	result.DecisionID = decisionID

	signalType := "login." + strings.ToLower(string(cmd.LoginChannel))
	// signal_value is JSONB NOT NULL. An empty signal set must still write an
	// object, otherwise the insert violates the constraint and the login path
	// fails closed on a legitimate allow decision.
	signalValue := json.RawMessage(`{}`)
	if len(cmd.Signals) > 0 {
		if raw, marshalErr := json.Marshal(cmd.Signals); marshalErr == nil {
			signalValue = raw
		}
	}
	// risk_signals.ip is an INET column: an arbitrary string (proxied requests
	// sometimes send "unknown" or a comma-separated hop list) would make the
	// insert fail and reject the login with a 500. Only a parseable address is
	// persisted; anything else is dropped rather than failing the decision.
	var ipPtr *string
	if ip := derefString(cmd.IP); ip != "" {
		if parsed := net.ParseIP(ip); parsed != nil {
			ipPtr = &ip
		}
	}
	if err := tx.Risk().AppendSignal(ctx, domain.Signal{
		ID: signalID, OrganizationID: cmd.OrganizationID, UserID: cmd.UserID,
		SignalType: signalType, SignalValue: signalValue, IP: ipPtr, CreatedAt: now,
	}); err != nil {
		return domain.EvaluateRiskResult{}, err
	}

	if err := tx.Risk().CreateDecision(ctx, domain.Decision{
		ID: decisionID, OrganizationID: cmd.OrganizationID, UserID: cmd.UserID,
		Score: result.Score, Decision: result.Decision, Factors: result.Factors, CreatedAt: now,
	}); err != nil {
		return domain.EvaluateRiskResult{}, err
	}

	metadata = normalizeMetadata(metadata)
	if err := s.embedAuditEvent(
		tx, ctx,
		metadata.IPAddress, metadata.UserAgent, metadata.Surface, metadata.RequestID, metadata.TraceID,
		cmd.OrganizationID, safeActorID(cmd.UserID),
		"risk.evaluation.completed", "risk_decision", decisionID,
		"risk", "risk.evaluation.completed",
		map[string]any{
			keyDecisionID: decisionID, keyOrgID: cmd.OrganizationID,
			keyChannel: cmd.LoginChannel, "decision": result.Decision,
			keyScore: result.Score,
		},
	); err != nil {
		return domain.EvaluateRiskResult{}, err
	}

	if result.Decision == domain.DecisionDeny {
		s.notifyCriticalLoginDenied(ctx, tx, criticalAlertEvent{
			OrganizationID: cmd.OrganizationID,
			UserID:         derefString(cmd.UserID),
			DecisionID:     decisionID,
			Channel:        string(cmd.LoginChannel),
			Score:          result.Score,
			IP:             metadata.IPAddress,
			UserAgent:      metadata.UserAgent,
		}, now)
	}

	return result, nil
}

func safeActorID(userID *string) string {
	if userID == nil || strings.TrimSpace(*userID) == "" {
		return ""
	}
	return *userID
}

// ── VerifyAuditIntegrity ──────────────────────────────────────────────

// auditChainSegmentSize bounds how many chain records one verification page
// holds in memory (BE-D-15). Long chains are streamed segment by segment
// instead of being materialised in a single slice.
const auditChainSegmentSize = 500

// VerifyAuditIntegrity performs no writes. Pending (not yet chained) records
// are reported distinctly from an actually broken or tampered chain.
func (s *Service) VerifyAuditIntegrity(ctx context.Context, query AuditIntegrityQuery) (*AuditIntegrityDTO, error) {
	query.OrganizationID = strings.TrimSpace(query.OrganizationID)
	query.ChainID = strings.TrimSpace(query.ChainID)
	if err := validateUUID("organizationId", query.OrganizationID); err != nil {
		return nil, err
	}
	if query.ChainID == "" {
		query.ChainID = query.OrganizationID
	}
	if err := validateUUID("chainId", query.ChainID); err != nil {
		return nil, err
	}
	if err := validateChainMatch(query.ChainID, query.OrganizationID); err != nil {
		return nil, err
	}
	repo := s.UOW.Risk()
	verifier := newAuditChainVerifier(query, s.Clock.Now())
	if segmentRepo, ok := repo.(domain.AuditChainSegmentReader); ok {
		// Anchors are few by design; loading them once up front is bounded.
		anchors, err := segmentRepo.ListAuditChainAnchors(ctx, query.OrganizationID, query.ChainID)
		if err != nil {
			return nil, err
		}
		verifier.anchors = anchors
		afterSeq := int64(0)
		for {
			entries, err := segmentRepo.ListAuditChainSegment(ctx, query.OrganizationID, query.ChainID, afterSeq, auditChainSegmentSize)
			if err != nil {
				return nil, err
			}
			if len(entries) == 0 {
				break
			}
			afterSeq = entries[len(entries)-1].ChainSeq
			if verifier.result.Failure == nil {
				verifier.consume(entries)
			} else {
				// The earliest failure is already located; keep walking the
				// chain only to preserve whole-chain TotalCount/PendingCount,
				// never verifying or retaining the remaining records.
				verifier.count(entries)
			}
			if len(entries) < auditChainSegmentSize {
				break
			}
		}
	} else {
		// Backends without the bounded-read capability keep the full-load
		// path; the verifier applies the identical checks in one pass.
		chain, err := repo.ListAuditChain(ctx, query)
		if err != nil {
			return nil, err
		}
		verifier.anchors = chain.Anchors
		verifier.consume(chain.Entries)
	}
	return verifier.finish(), nil
}

// auditChainVerifier carries the verified prefix across bounded segments:
// expectedSeq (one-based over hashed records), the baseline algorithm and the
// platform ChainVerifyState. Structural checks run per segment before hash
// verification so a failure still reports the earliest invalid record, and
// anchor results are deferred until the whole chain verifies — exactly the
// precedence of the previous full-load implementation.
type auditChainVerifier struct {
	chainID       string
	result        *AuditIntegrityDTO
	anchors       []domain.AuditChainAnchor
	anchorIndex   int
	anchorFailure *AuditIntegrityFailure
	expectedSeq   int64
	baselineAlgo  string
	state         platformaudit.ChainVerifyState
}

func newAuditChainVerifier(query AuditIntegrityQuery, now time.Time) *auditChainVerifier {
	return &auditChainVerifier{
		chainID: query.ChainID,
		result: &AuditIntegrityDTO{
			OrganizationID: query.OrganizationID, ChainID: query.ChainID,
			VerifiedAt: formatTime(now),
		},
		expectedSeq: 1,
	}
}

// count accumulates whole-chain totals without verification (drain mode).
func (v *auditChainVerifier) count(entries []domain.AuditChainRecord) {
	v.result.TotalCount += len(entries)
	for i := range entries {
		if len(entries[i].EntryHash) == 0 {
			v.result.PendingCount++
		}
	}
}

// consume structurally checks and hash-verifies one segment, then matches any
// anchors whose seq falls inside it. It stops at the earliest failure.
func (v *auditChainVerifier) consume(entries []domain.AuditChainRecord) {
	v.count(entries)
	records := make([]platformaudit.ChainRecord, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		if len(entry.EntryHash) == 0 {
			continue
		}
		records = append(records, platformaudit.ChainRecord{
			OrganizationID: entry.OrganizationID, ActorID: entry.ActorID,
			Action: entry.Action, ResourceType: entry.ResourceType, ResourceID: entry.ResourceID,
			IP: entry.IP, UserAgent: entry.UserAgent, Surface: entry.Surface,
			RequestID: entry.RequestID, TraceID: entry.TraceID, Metadata: entry.Metadata,
			CreatedAt: entry.CreatedAt, ChainSeq: entry.ChainSeq,
			PrevHash: entry.PrevHash, EntryHash: entry.EntryHash, ChainAlgo: entry.ChainAlgo,
		})
	}

	// AuditChainWriter allocates one-based, gap-free sequence numbers. Check
	// structure before full verification, then verify the preceding prefix so
	// the reported failure is still the earliest invalid record.
	structuralFailure := -1
	structuralReason := ""
	for i := range records {
		record := &records[i]
		expectedSeq := v.expectedSeq + int64(i)
		if record.ChainSeq != expectedSeq {
			structuralFailure = i
			structuralReason = fmt.Sprintf("audit chain sequence mismatch: expected %d", expectedSeq)
			break
		}
		algo, algoErr := platformcrypto.NormalizeChainAlgorithm(record.ChainAlgo)
		if algoErr != nil {
			structuralFailure = i
			structuralReason = algoErr.Error()
			break
		}
		if v.baselineAlgo == "" {
			v.baselineAlgo = algo
		} else if algo != v.baselineAlgo {
			structuralFailure = i
			structuralReason = fmt.Sprintf("audit chain algorithm changed from %s to %s at chain_seq %d",
				v.baselineAlgo, algo, record.ChainSeq)
			break
		}
	}
	if structuralFailure >= 0 {
		verified, _, verifyErr := platformaudit.VerifyChainRecordBatch(v.chainID, records[:structuralFailure], v.state)
		v.result.VerifiedCount += verified
		if verifyErr != nil {
			v.result.Failure = &AuditIntegrityFailure{ChainSeq: records[verified].ChainSeq, Reason: verifyErr.Error()}
		} else {
			v.result.Failure = &AuditIntegrityFailure{ChainSeq: records[structuralFailure].ChainSeq, Reason: structuralReason}
		}
		return
	}
	verified, state, verifyErr := platformaudit.VerifyChainRecordBatch(v.chainID, records, v.state)
	v.result.VerifiedCount += verified
	if verifyErr != nil {
		failure := AuditIntegrityFailure{Reason: verifyErr.Error()}
		if verified < len(records) {
			failure.ChainSeq = records[verified].ChainSeq
		}
		v.result.Failure = &failure
		return
	}
	v.state = state
	v.expectedSeq += int64(len(records))
	v.matchAnchors(records)
}

// matchAnchors compares anchors against the hashed records of a verified
// segment. Both sides are ascending, so a single cursor suffices; the first
// failing anchor is remembered and only surfaced by finish() if the whole
// chain verifies (chain failures outrank anchor failures, as before).
func (v *auditChainVerifier) matchAnchors(records []platformaudit.ChainRecord) {
	for i := range records {
		seq := records[i].ChainSeq
		for v.anchorIndex < len(v.anchors) && v.anchors[v.anchorIndex].AnchorSeq <= seq {
			anchor := v.anchors[v.anchorIndex]
			v.anchorIndex++
			if v.anchorFailure != nil {
				continue
			}
			if anchor.AnchorSeq != seq {
				v.anchorFailure = &AuditIntegrityFailure{
					ChainSeq: anchor.AnchorSeq,
					Reason:   "audit chain anchor references a missing or pending entry",
				}
			} else if !bytes.Equal(anchor.EntryHash, records[i].EntryHash) {
				v.anchorFailure = &AuditIntegrityFailure{
					ChainSeq: anchor.AnchorSeq,
					Reason:   "audit chain anchor entry_hash mismatch",
				}
			}
		}
	}
}

// finish reports anchors that never met a hashed entry, then derives the
// valid/pending/invalid status exactly as the full-load implementation did.
func (v *auditChainVerifier) finish() *AuditIntegrityDTO {
	if v.result.Failure == nil {
		if v.anchorFailure == nil && v.anchorIndex < len(v.anchors) {
			v.anchorFailure = &AuditIntegrityFailure{
				ChainSeq: v.anchors[v.anchorIndex].AnchorSeq,
				Reason:   "audit chain anchor references a missing or pending entry",
			}
		}
		v.result.Failure = v.anchorFailure
	}
	switch {
	case v.result.Failure != nil:
		v.result.Status = "invalid"
	case v.result.PendingCount > 0:
		v.result.Status = "pending"
	default:
		v.result.Status = "valid"
		v.result.Valid = true
	}
	return v.result
}

func toSignalDTO(s *domain.Signal) SignalDTO {
	return SignalDTO{
		ID: s.ID, OrganizationID: s.OrganizationID, UserID: s.UserID,
		SignalType: s.SignalType, SignalValue: s.SignalValue, IP: s.IP, CreatedAt: formatTime(s.CreatedAt),
	}
}

func toDecisionDTO(d *domain.Decision) (DecisionDTO, error) {
	engineFactors, workflow, err := domain.DecodeDecisionFactors(d.Factors)
	if err != nil {
		return DecisionDTO{}, err
	}
	return DecisionDTO{
		ID: d.ID, OrganizationID: d.OrganizationID, UserID: d.UserID,
		Score: d.Score, Decision: string(d.Decision), Factors: engineFactors,
		CreatedAt: formatTime(d.CreatedAt), Revision: workflow.Revision, Workflow: workflow,
	}, nil
}

func toGovernanceDTO(g *domain.SessionGovernance) SessionGovernanceDTO {
	return SessionGovernanceDTO{
		ID: g.ID, OrganizationID: g.OrganizationID, SessionID: g.SessionID,
		RiskScore: g.RiskScore, Action: string(g.Action), Reason: g.Reason,
		CreatedAt: formatTime(g.CreatedAt), Revision: g.Revision,
	}
}

func formatTime(value time.Time) string {
	return value.Format("2006-01-02T15:04:05.000Z07:00")
}

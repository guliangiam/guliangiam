package risk

import (
	"context"

	apprisk "github.com/guliangiam/guliangiam/internal/app/risk"
)

type Handler struct {
	Service apprisk.Service
}

type ListSignalsQuery struct {
	OrganizationID string
	UserID         *string
	SignalType     *string
	Page           int
	PageSize       int
}

func (h Handler) ListSignals(ctx context.Context, q ListSignalsQuery) ([]apprisk.SignalDTO, int64, error) {
	return h.Service.ListSignals(ctx, apprisk.ListSignalsQuery{
		OrganizationID: q.OrganizationID, UserID: q.UserID, SignalType: q.SignalType,
		Page: q.Page, PageSize: q.PageSize,
	})
}

type ListDecisionsQuery struct {
	OrganizationID string
	UserID         *string
	Decision       *string
	Page           int
	PageSize       int
}

func (h Handler) ListDecisions(ctx context.Context, q ListDecisionsQuery) ([]apprisk.DecisionDTO, int64, error) {
	var decision *apprisk.DecisionAction
	if q.Decision != nil {
		value := apprisk.DecisionAction(*q.Decision)
		decision = &value
	}
	return h.Service.ListDecisions(ctx, apprisk.ListDecisionsQuery{
		OrganizationID: q.OrganizationID, UserID: q.UserID, Decision: decision,
		Page: q.Page, PageSize: q.PageSize,
	})
}

func (h Handler) GetDecision(ctx context.Context, organizationID, decisionID string) (*apprisk.DecisionDTO, error) {
	return h.Service.GetDecision(ctx, organizationID, decisionID)
}

type ReviewDecisionRequest struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
	Revision *int64 `json:"revision"`
}

func (h Handler) ReviewDecision(ctx context.Context, organizationID, decisionID string, req ReviewDecisionRequest, metadata apprisk.CommandMetadata) (*apprisk.DecisionDTO, error) {
	return h.Service.ReviewDecision(ctx, apprisk.ReviewDecisionCommand{
		OrganizationID: organizationID, DecisionID: decisionID,
		NewDecision: apprisk.DecisionAction(req.Decision), Reason: req.Reason,
		ExpectedRevision: req.Revision, Metadata: metadata,
	})
}

type SubmitAppealRequest struct {
	Reason   string `json:"reason"`
	Revision *int64 `json:"revision"`
}

func (h Handler) SubmitAppeal(ctx context.Context, organizationID, decisionID string, req SubmitAppealRequest, metadata apprisk.CommandMetadata) (*apprisk.DecisionDTO, error) {
	return h.Service.SubmitAppeal(ctx, apprisk.SubmitAppealCommand{
		OrganizationID: organizationID, DecisionID: decisionID, Reason: req.Reason,
		ExpectedRevision: req.Revision, Metadata: metadata,
	})
}

type ResolveAppealRequest struct {
	Resolution string `json:"resolution"`
	Reason     string `json:"reason"`
	Revision   *int64 `json:"revision"`
}

func (h Handler) ResolveAppeal(ctx context.Context, organizationID, decisionID string, req ResolveAppealRequest, metadata apprisk.CommandMetadata) (*apprisk.DecisionDTO, error) {
	return h.Service.ResolveAppeal(ctx, apprisk.ResolveAppealCommand{
		OrganizationID: organizationID, DecisionID: decisionID,
		Resolution: apprisk.AppealStatus(req.Resolution), Reason: req.Reason,
		ExpectedRevision: req.Revision, Metadata: metadata,
	})
}

type ListGovernanceQuery struct {
	OrganizationID string
	Action         *string
	Page           int
	PageSize       int
}

func (h Handler) ListGovernance(ctx context.Context, q ListGovernanceQuery) ([]apprisk.SessionGovernanceDTO, int64, error) {
	var action *apprisk.GovernanceAction
	if q.Action != nil {
		value := apprisk.GovernanceAction(*q.Action)
		action = &value
	}
	return h.Service.ListGovernance(ctx, apprisk.ListGovernanceQuery{
		OrganizationID: q.OrganizationID, Action: action, Page: q.Page, PageSize: q.PageSize,
	})
}

func (h Handler) GetGovernance(ctx context.Context, organizationID, governanceID string) (*apprisk.SessionGovernanceDTO, error) {
	return h.Service.GetGovernance(ctx, organizationID, governanceID)
}

type TransitionGovernanceRequest struct {
	Action   string `json:"action"`
	Reason   string `json:"reason"`
	Revision *int64 `json:"revision"`
}

func (h Handler) TransitionGovernance(ctx context.Context, organizationID, governanceID string, req TransitionGovernanceRequest, metadata apprisk.CommandMetadata) (*apprisk.SessionGovernanceDTO, error) {
	return h.Service.TransitionGovernance(ctx, apprisk.TransitionGovernanceCommand{
		OrganizationID: organizationID, GovernanceID: governanceID,
		NewAction: apprisk.GovernanceAction(req.Action), Reason: req.Reason,
		ExpectedRevision: req.Revision, Metadata: metadata,
	})
}

func (h Handler) VerifyAuditIntegrity(ctx context.Context, organizationID, chainID string) (*apprisk.AuditIntegrityDTO, error) {
	return h.Service.VerifyAuditIntegrity(ctx, apprisk.AuditIntegrityQuery{
		OrganizationID: organizationID, ChainID: chainID,
	})
}

package risk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	notificationproviderdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	notificationdomain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	outboxdomain "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformaudit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOrgID        = "11111111-1111-4111-8111-111111111111"
	testDecisionID   = "22222222-2222-4222-8222-222222222222"
	testActorID      = "33333333-3333-4333-8333-333333333333"
	testSessionID    = "44444444-4444-4444-8444-444444444444"
	testGovernanceID = "55555555-5555-4555-8555-555555555555"
	testNewGovID     = "66666666-6666-4666-8666-666666666666"
)

var testNow = time.Date(2026, 10, 26, 8, 30, 0, 0, time.UTC)

type fakeClock struct{}

func (fakeClock) Now() time.Time { return testNow }

type fakeUUID struct{ err error }

func (f fakeUUID) NewString() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return testNewGovID, nil
}

type fakeRiskRepository struct {
	domain.Repository
	decision           *domain.Decision
	governance         *domain.SessionGovernance
	auditChain         *domain.AuditChain
	updatedDecision    *domain.UpdateDecisionCommand
	appendedGovernance []domain.SessionGovernance
	lastIntegrityQuery domain.AuditIntegrityQuery
}

func (r *fakeRiskRepository) AppendSignal(_ context.Context, signal domain.Signal) error {
	return nil
}

func (r *fakeRiskRepository) CreateDecision(_ context.Context, decision domain.Decision) error {
	snapshot := decision
	r.decision = &snapshot
	return nil
}

func (r *fakeRiskRepository) GetDecision(_ context.Context, id, organizationID string) (*domain.Decision, error) {
	return r.decisionFor(id, organizationID)
}

func (r *fakeRiskRepository) GetDecisionForUpdate(_ context.Context, id, organizationID string) (*domain.Decision, error) {
	return r.decisionFor(id, organizationID)
}

func (r *fakeRiskRepository) decisionFor(id, organizationID string) (*domain.Decision, error) {
	if r.decision == nil || r.decision.ID != id || r.decision.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("risk decision not found")
	}
	snapshot := *r.decision
	return &snapshot, nil
}

func (r *fakeRiskRepository) UpdateDecision(_ context.Context, cmd domain.UpdateDecisionCommand) error {
	if r.decision == nil || r.decision.ID != cmd.DecisionID || r.decision.OrganizationID != cmd.OrganizationID ||
		r.decision.Decision != cmd.ExpectedDecision || !bytes.Equal(r.decision.Factors, cmd.ExpectedFactors) {
		return apperrors.NewConflict("risk decision revision mismatch")
	}
	r.updatedDecision = &cmd
	r.decision.Decision = cmd.NewDecision
	r.decision.Factors = cmd.NewFactors
	return nil
}

func (r *fakeRiskRepository) GetGovernance(_ context.Context, id, organizationID string) (*domain.SessionGovernance, error) {
	if r.governance == nil || r.governance.ID != id || r.governance.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("session governance not found")
	}
	snapshot := *r.governance
	return &snapshot, nil
}

func (r *fakeRiskRepository) GetGovernanceForTransition(ctx context.Context, id, organizationID string) (*domain.SessionGovernance, error) {
	return r.GetGovernance(ctx, id, organizationID)
}

func (r *fakeRiskRepository) AppendGovernance(_ context.Context, governance domain.SessionGovernance) error {
	r.appendedGovernance = append(r.appendedGovernance, governance)
	snapshot := governance
	r.governance = &snapshot
	return nil
}

func (r *fakeRiskRepository) ListAuditChain(_ context.Context, query domain.AuditIntegrityQuery) (*domain.AuditChain, error) {
	r.lastIntegrityQuery = query
	if r.auditChain == nil {
		return &domain.AuditChain{}, nil
	}
	return r.auditChain, nil
}

// ListAuditChainSegment serves the bounded-read path from the same fixture
// chain: entries are stored ascending, pages filter chain_seq > afterSeq.
func (r *fakeRiskRepository) ListAuditChainSegment(_ context.Context, organizationID, chainID string, afterSeq int64, limit int) ([]domain.AuditChainRecord, error) {
	r.lastIntegrityQuery = domain.AuditIntegrityQuery{OrganizationID: organizationID, ChainID: chainID}
	if r.auditChain == nil {
		return nil, nil
	}
	entries := make([]domain.AuditChainRecord, 0, limit)
	for _, entry := range r.auditChain.Entries {
		if entry.ChainSeq <= afterSeq {
			continue
		}
		entries = append(entries, entry)
		if len(entries) >= limit {
			break
		}
	}
	return entries, nil
}

func (r *fakeRiskRepository) ListAuditChainAnchors(_ context.Context, organizationID, chainID string) ([]domain.AuditChainAnchor, error) {
	r.lastIntegrityQuery = domain.AuditIntegrityQuery{OrganizationID: organizationID, ChainID: chainID}
	if r.auditChain == nil {
		return nil, nil
	}
	return r.auditChain.Anchors, nil
}

type recordingRecordsRepository struct {
	recordsdomain.Repository
	records []*recordsdomain.Record
	err     error
}

func (r *recordingRecordsRepository) Create(_ context.Context, record *recordsdomain.Record) error {
	if r.err != nil {
		return r.err
	}
	snapshot := *record
	r.records = append(r.records, &snapshot)
	return nil
}

type recordingOutboxRepository struct {
	events []*outboxdomain.OutboxEvent
	err    error
}

func (r *recordingOutboxRepository) Enqueue(_ context.Context, event *outboxdomain.OutboxEvent) error {
	if r.err != nil {
		return r.err
	}
	snapshot := *event
	r.events = append(r.events, &snapshot)
	return nil
}

func (r *recordingOutboxRepository) EnqueueBatch(_ context.Context, events []*outboxdomain.OutboxEvent) error {
	for _, event := range events {
		if err := r.Enqueue(context.Background(), event); err != nil {
			return err
		}
	}
	return nil
}

type fakeUnitOfWork struct {
	appuow.UnitOfWork
	riskRepository        *fakeRiskRepository
	records               *recordingRecordsRepository
	outbox                *recordingOutboxRepository
	users                 *fakeUserRepository
	notifications         *fakeNotificationRepository
	notificationProviders *fakeNotificationProviderRepository
	commits               int
	rollbacks             int
}

func (u *fakeUnitOfWork) Risk() domain.Repository { return u.riskRepository }

func (u *fakeUnitOfWork) Records() recordsdomain.Repository {
	if u.records == nil {
		return nil
	}
	return u.records
}

func (u *fakeUnitOfWork) Outbox() outboxdomain.Repository {
	if u.outbox == nil {
		return nil
	}
	return u.outbox
}

func (u *fakeUnitOfWork) Users() userdomain.Repository {
	if u.users == nil {
		return nil
	}
	return u.users
}

func (u *fakeUnitOfWork) Notifications() notificationdomain.Repository {
	if u.notifications == nil {
		return nil
	}
	return u.notifications
}

func (u *fakeUnitOfWork) NotificationProviders() notificationproviderdomain.Repository {
	if u.notificationProviders == nil {
		return nil
	}
	return u.notificationProviders
}

func (u *fakeUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	err := fn(ctx, u)
	if err != nil {
		u.rollbacks++
		return err
	}
	u.commits++
	return nil
}

type serviceFixture struct {
	service Service
	repo    *fakeRiskRepository
	uow     *fakeUnitOfWork
}

func newServiceFixture() *serviceFixture {
	repo := &fakeRiskRepository{}
	uow := &fakeUnitOfWork{
		riskRepository: repo,
		records:        &recordingRecordsRepository{},
		outbox:         &recordingOutboxRepository{},
	}
	return &serviceFixture{service: Service{Clock: fakeClock{}, UUID: fakeUUID{}, UOW: uow}, repo: repo, uow: uow}
}

func testMetadata() CommandMetadata {
	return CommandMetadata{
		ActorID: testActorID, IPAddress: "127.0.0.1", UserAgent: "risk-test",
		Surface: "management", RequestID: "request-1", TraceID: "trace-1",
	}
}

func revisionPtr(value int64) *int64 { return &value }

func decisionFixture(action domain.DecisionAction) *domain.Decision {
	return &domain.Decision{
		ID: testDecisionID, OrganizationID: testOrgID, Score: 80, Decision: action,
		Factors: json.RawMessage(`[{"type":"new_device"}]`), CreatedAt: testNow.Add(-time.Minute),
	}
}

func TestReviewDecision_PersistsWorkflowAuditAndOutbox(t *testing.T) {
	fixture := newServiceFixture()
	fixture.repo.decision = decisionFixture(domain.DecisionChallenge)

	result, err := fixture.service.ReviewDecision(context.Background(), ReviewDecisionCommand{
		OrganizationID: testOrgID, DecisionID: testDecisionID, NewDecision: domain.DecisionDeny,
		Reason: "confirmed credential stuffing", ExpectedRevision: revisionPtr(0), Metadata: testMetadata(),
	})
	require.NoError(t, err)
	assert.Equal(t, string(domain.DecisionDeny), result.Decision)
	assert.Equal(t, int64(1), result.Revision)
	assert.Equal(t, domain.ReviewCompleted, result.Workflow.Review.Status)
	assert.Len(t, result.Workflow.History, 1)
	assert.JSONEq(t, `[{"type":"new_device"}]`, string(result.Factors))
	require.NotNil(t, fixture.repo.updatedDecision)
	assert.Equal(t, domain.DecisionChallenge, fixture.repo.updatedDecision.ExpectedDecision)
	assert.Equal(t, 1, fixture.uow.commits)
	assert.Zero(t, fixture.uow.rollbacks)

	require.Len(t, fixture.uow.records.records, 1)
	audit := fixture.uow.records.records[0]
	assert.Equal(t, "risk.decision.reviewed", audit.Action)
	assert.Equal(t, testActorID, audit.ActorID)
	assert.Equal(t, "127.0.0.1", audit.IP)
	assert.Equal(t, "request-1", audit.RequestID)
	assert.Equal(t, "trace-1", audit.TraceID)
	require.Len(t, fixture.uow.outbox.events, 1)
	event := fixture.uow.outbox.events[0]
	assert.Equal(t, "risk.decision.reviewed", event.EventType)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	assert.Equal(t, float64(1), payload["revision"])
	assert.Equal(t, string(domain.DecisionDeny), payload["toDecision"])
}

func TestReviewDecision_RejectsWeakeningAndStaleRevision(t *testing.T) {
	t.Run("weakening", func(t *testing.T) {
		fixture := newServiceFixture()
		fixture.repo.decision = decisionFixture(domain.DecisionDeny)
		_, err := fixture.service.ReviewDecision(context.Background(), ReviewDecisionCommand{
			OrganizationID: testOrgID, DecisionID: testDecisionID, NewDecision: domain.DecisionAllow,
			Reason: "attempted bypass", ExpectedRevision: revisionPtr(0), Metadata: testMetadata(),
		})
		assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
		assert.Nil(t, fixture.repo.updatedDecision)
		assert.Empty(t, fixture.uow.records.records)
		assert.Empty(t, fixture.uow.outbox.events)
		assert.Equal(t, 1, fixture.uow.rollbacks)
	})

	t.Run("stale revision", func(t *testing.T) {
		fixture := newServiceFixture()
		fixture.repo.decision = decisionFixture(domain.DecisionChallenge)
		_, err := fixture.service.ReviewDecision(context.Background(), ReviewDecisionCommand{
			OrganizationID: testOrgID, DecisionID: testDecisionID, NewDecision: domain.DecisionDeny,
			Reason: "stale writer", ExpectedRevision: revisionPtr(1), Metadata: testMetadata(),
		})
		assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
		assert.Nil(t, fixture.repo.updatedDecision)
		assert.Equal(t, 1, fixture.uow.rollbacks)
	})
}

func TestAppealWorkflow_SubmitResolveAndTerminalState(t *testing.T) {
	fixture := newServiceFixture()
	fixture.repo.decision = decisionFixture(domain.DecisionDeny)

	submitted, err := fixture.service.SubmitAppeal(context.Background(), SubmitAppealCommand{
		OrganizationID: testOrgID, DecisionID: testDecisionID, Reason: "managed device was misclassified",
		ExpectedRevision: revisionPtr(0), Metadata: testMetadata(),
	})
	require.NoError(t, err)
	assert.Equal(t, domain.AppealPending, submitted.Workflow.Appeal.Status)
	assert.Equal(t, int64(1), submitted.Revision)

	resolved, err := fixture.service.ResolveAppeal(context.Background(), ResolveAppealCommand{
		OrganizationID: testOrgID, DecisionID: testDecisionID, Resolution: AppealAccepted,
		Reason: "device evidence verified", ExpectedRevision: revisionPtr(1), Metadata: testMetadata(),
	})
	require.NoError(t, err)
	assert.Equal(t, domain.AppealAccepted, resolved.Workflow.Appeal.Status)
	assert.Equal(t, string(domain.DecisionDeny), resolved.Decision, "appeal acceptance must not bypass enforcement")
	assert.Equal(t, int64(2), resolved.Revision)
	assert.Len(t, resolved.Workflow.History, 2)
	require.Len(t, fixture.uow.records.records, 2)
	assert.Equal(t, "risk.decision.appeal.submitted", fixture.uow.records.records[0].Action)
	assert.Equal(t, "risk.decision.appeal.accepted", fixture.uow.records.records[1].Action)

	_, err = fixture.service.SubmitAppeal(context.Background(), SubmitAppealCommand{
		OrganizationID: testOrgID, DecisionID: testDecisionID, Reason: "duplicate",
		ExpectedRevision: revisionPtr(2), Metadata: testMetadata(),
	})
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	assert.Equal(t, 2, fixture.uow.commits)
	assert.Equal(t, 1, fixture.uow.rollbacks)
}

func TestTransitionGovernance_AppendsLegalTransitionAndEnforcesTerminalState(t *testing.T) {
	fixture := newServiceFixture()
	sessionID := testSessionID
	fixture.repo.governance = &domain.SessionGovernance{
		ID: testGovernanceID, OrganizationID: testOrgID, SessionID: &sessionID,
		RiskScore: 90, Action: domain.GovernanceStepUp, CreatedAt: testNow, Revision: 2,
	}

	result, err := fixture.service.TransitionGovernance(context.Background(), TransitionGovernanceCommand{
		OrganizationID: testOrgID, GovernanceID: testGovernanceID, NewAction: domain.GovernanceRevoke,
		Reason: "step-up failed repeatedly", ExpectedRevision: revisionPtr(2), Metadata: testMetadata(),
	})
	require.NoError(t, err)
	assert.Equal(t, testNewGovID, result.ID)
	assert.Equal(t, string(domain.GovernanceRevoke), result.Action)
	assert.Equal(t, int64(3), result.Revision)
	assert.Equal(t, testNow.Add(time.Second).Format("2006-01-02T15:04:05.000Z07:00"), result.CreatedAt)
	require.Len(t, fixture.repo.appendedGovernance, 1)
	assert.Equal(t, 90, fixture.repo.appendedGovernance[0].RiskScore)
	assert.Equal(t, "step-up failed repeatedly", *fixture.repo.appendedGovernance[0].Reason)
	require.Len(t, fixture.uow.records.records, 1)
	assert.Equal(t, "risk.session_governance.transitioned", fixture.uow.records.records[0].Action)
	require.Len(t, fixture.uow.outbox.events, 1)
	assert.Equal(t, testNewGovID, fixture.uow.outbox.events[0].AggregateID)

	_, err = fixture.service.TransitionGovernance(context.Background(), TransitionGovernanceCommand{
		OrganizationID: testOrgID, GovernanceID: testNewGovID, NewAction: domain.GovernanceMonitor,
		Reason: "illegal reopen", ExpectedRevision: revisionPtr(3), Metadata: testMetadata(),
	})
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	assert.Len(t, fixture.repo.appendedGovernance, 1)
}

func TestWriteFailure_DoesNotCommitOrEnqueueOutbox(t *testing.T) {
	fixture := newServiceFixture()
	fixture.repo.decision = decisionFixture(domain.DecisionChallenge)
	fixture.uow.records.err = errors.New("audit unavailable")

	_, err := fixture.service.ReviewDecision(context.Background(), ReviewDecisionCommand{
		OrganizationID: testOrgID, DecisionID: testDecisionID, NewDecision: domain.DecisionDeny,
		Reason: "confirmed risk", ExpectedRevision: revisionPtr(0), Metadata: testMetadata(),
	})
	require.Error(t, err)
	assert.Zero(t, fixture.uow.commits)
	assert.Equal(t, 1, fixture.uow.rollbacks)
	assert.Empty(t, fixture.uow.outbox.events)
}

func TestVerifyAuditIntegrity_ReadOnlyResult(t *testing.T) {
	t.Run("empty chain is valid", func(t *testing.T) {
		fixture := newServiceFixture()
		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
		require.NoError(t, err)
		assert.True(t, result.Valid)
		assert.Equal(t, "valid", result.Status)
		assert.Equal(t, testOrgID, result.ChainID)
		assert.Equal(t, "2026-10-26T08:30:00.000Z", result.VerifiedAt)
		assert.Zero(t, fixture.uow.commits)
		assert.Empty(t, fixture.uow.records.records)
		assert.Empty(t, fixture.uow.outbox.events)
	})

	t.Run("tamper is located", func(t *testing.T) {
		fixture := newServiceFixture()
		fixture.repo.auditChain = &domain.AuditChain{Entries: []domain.AuditChainRecord{{
			OrganizationID: testOrgID, ActorID: testActorID, Action: "risk.decision.reviewed",
			ResourceType: "decision", ResourceID: testDecisionID, Surface: "management",
			Metadata: map[string]any{}, CreatedAt: testNow, ChainSeq: 1, EntryHash: []byte{1}, ChainAlgo: "sha256",
		}}}
		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID, ChainID: testOrgID})
		require.NoError(t, err)
		assert.False(t, result.Valid)
		assert.Equal(t, "invalid", result.Status)
		require.NotNil(t, result.Failure)
		assert.Equal(t, int64(1), result.Failure.ChainSeq)
		assert.Contains(t, result.Failure.Reason, "entry_hash mismatch")
	})

	t.Run("sequence gap is located", func(t *testing.T) {
		fixture := newServiceFixture()
		fixture.repo.auditChain = &domain.AuditChain{Entries: []domain.AuditChainRecord{{
			OrganizationID: testOrgID, ChainSeq: 2, EntryHash: []byte{1}, ChainAlgo: "sha256",
		}}}
		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
		require.NoError(t, err)
		assert.Equal(t, "invalid", result.Status)
		require.NotNil(t, result.Failure)
		assert.Equal(t, int64(2), result.Failure.ChainSeq)
		assert.Contains(t, result.Failure.Reason, "sequence mismatch")
	})

	t.Run("sm3 algorithm is accepted", func(t *testing.T) {
		fixture := newServiceFixture()
		fixture.repo.auditChain = &domain.AuditChain{Entries: []domain.AuditChainRecord{{
			OrganizationID: testOrgID, ChainSeq: 1, EntryHash: []byte{1}, ChainAlgo: "sm3",
		}}}
		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
		require.NoError(t, err)
		assert.Equal(t, "invalid", result.Status)
		require.NotNil(t, result.Failure)
		// 该夹具的 entry_hash 是占位值，因此应进入重算校验而不是被算法白名单拒绝。
		assert.Contains(t, result.Failure.Reason, "sm3")
		assert.NotContains(t, result.Failure.Reason, "unsupported")
	})

	t.Run("unknown algorithm is located", func(t *testing.T) {
		fixture := newServiceFixture()
		fixture.repo.auditChain = &domain.AuditChain{Entries: []domain.AuditChainRecord{{
			OrganizationID: testOrgID, ChainSeq: 1, EntryHash: []byte{1}, ChainAlgo: "md5",
		}}}
		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
		require.NoError(t, err)
		assert.Equal(t, "invalid", result.Status)
		require.NotNil(t, result.Failure)
		assert.Equal(t, int64(1), result.Failure.ChainSeq)
		assert.Contains(t, result.Failure.Reason, "unsupported")
		assert.Contains(t, result.Failure.Reason, "md5")
	})

	t.Run("mixed algorithms are invalid", func(t *testing.T) {
		fixture := newServiceFixture()
		fixture.repo.auditChain = &domain.AuditChain{Entries: []domain.AuditChainRecord{
			{OrganizationID: testOrgID, ChainSeq: 1, EntryHash: []byte{1}, ChainAlgo: "sha256"},
			{OrganizationID: testOrgID, ChainSeq: 2, EntryHash: []byte{2}, ChainAlgo: "sm3"},
		}}
		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
		require.NoError(t, err)
		assert.Equal(t, "invalid", result.Status)
		require.NotNil(t, result.Failure)
	})

	t.Run("pending is not reported as a healthy complete chain", func(t *testing.T) {
		fixture := newServiceFixture()
		// Chain sequences are one-based (the segment cursor starts at 0).
		fixture.repo.auditChain = &domain.AuditChain{Entries: []domain.AuditChainRecord{{
			OrganizationID: testOrgID, ChainSeq: 1, ChainAlgo: "sha256",
		}}}
		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
		require.NoError(t, err)
		assert.False(t, result.Valid)
		assert.Equal(t, "pending", result.Status)
		assert.Equal(t, 1, result.PendingCount)
		assert.Nil(t, result.Failure)
	})

	t.Run("matching anchor is verified", func(t *testing.T) {
		fixture := newServiceFixture()
		record := domain.AuditChainRecord{
			OrganizationID: testOrgID, ActorID: testActorID, Action: "risk.decision.reviewed",
			ResourceType: "decision", ResourceID: testDecisionID, Surface: "management",
			Metadata: map[string]any{}, CreatedAt: testNow, ChainSeq: 1, ChainAlgo: "sha256",
		}
		hash, err := platformaudit.RecomputeEntryHash("sha256", platformaudit.ChainRecord{
			OrganizationID: record.OrganizationID, ActorID: record.ActorID, Action: record.Action,
			ResourceType: record.ResourceType, ResourceID: record.ResourceID, IP: record.IP,
			UserAgent: record.UserAgent, Surface: record.Surface, RequestID: record.RequestID,
			TraceID: record.TraceID, Metadata: record.Metadata, CreatedAt: record.CreatedAt,
			ChainSeq: record.ChainSeq, PrevHash: record.PrevHash, ChainAlgo: record.ChainAlgo,
		})
		require.NoError(t, err)
		record.EntryHash = hash
		fixture.repo.auditChain = &domain.AuditChain{
			Entries: []domain.AuditChainRecord{record},
			Anchors: []domain.AuditChainAnchor{{AnchorSeq: 1, EntryHash: hash}},
		}

		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
		require.NoError(t, err)
		assert.True(t, result.Valid)
		assert.Nil(t, result.Failure)
	})

	t.Run("anchor hash mismatch is invalid", func(t *testing.T) {
		fixture := newServiceFixture()
		record := domain.AuditChainRecord{
			OrganizationID: testOrgID, ActorID: testActorID, Action: "risk.decision.reviewed",
			ResourceType: "decision", ResourceID: testDecisionID, Surface: "management",
			Metadata: map[string]any{}, CreatedAt: testNow, ChainSeq: 1, ChainAlgo: "sha256",
		}
		hash, err := platformaudit.RecomputeEntryHash("sha256", platformaudit.ChainRecord{
			OrganizationID: record.OrganizationID, ActorID: record.ActorID, Action: record.Action,
			ResourceType: record.ResourceType, ResourceID: record.ResourceID, Surface: record.Surface,
			Metadata: record.Metadata, CreatedAt: record.CreatedAt, ChainSeq: record.ChainSeq,
			ChainAlgo: record.ChainAlgo,
		})
		require.NoError(t, err)
		record.EntryHash = hash
		fixture.repo.auditChain = &domain.AuditChain{
			Entries: []domain.AuditChainRecord{record},
			Anchors: []domain.AuditChainAnchor{{AnchorSeq: 1, EntryHash: []byte("tampered")}},
		}

		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
		require.NoError(t, err)
		assert.Equal(t, "invalid", result.Status)
		require.NotNil(t, result.Failure)
		assert.Equal(t, int64(1), result.Failure.ChainSeq)
		assert.Contains(t, result.Failure.Reason, "anchor entry_hash mismatch")
	})

	t.Run("anchor cannot target a missing or pending entry", func(t *testing.T) {
		fixture := newServiceFixture()
		fixture.repo.auditChain = &domain.AuditChain{
			Anchors: []domain.AuditChainAnchor{{AnchorSeq: 1, EntryHash: []byte{1}}},
		}

		result, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{OrganizationID: testOrgID})
		require.NoError(t, err)
		assert.Equal(t, "invalid", result.Status)
		require.NotNil(t, result.Failure)
		assert.Equal(t, int64(1), result.Failure.ChainSeq)
		assert.Contains(t, result.Failure.Reason, "missing or pending")
	})
}

func TestVerifyAuditIntegrity_RejectsCrossTenantChain(t *testing.T) {
	fixture := newServiceFixture()
	_, err := fixture.service.VerifyAuditIntegrity(context.Background(), AuditIntegrityQuery{
		OrganizationID: testOrgID, ChainID: testDecisionID,
	})
	// A chain pinned to another tenant is a cross-field contradiction: the body
	// parsed, the rule rejected it (手册 4.14.1 → 422).
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	assert.Empty(t, fixture.repo.lastIntegrityQuery.ChainID)
	assert.Zero(t, fixture.uow.commits)
}

func TestWriteValidation_RequiresReasonRevisionAndActor(t *testing.T) {
	fixture := newServiceFixture()
	base := ReviewDecisionCommand{
		OrganizationID: testOrgID, DecisionID: testDecisionID, NewDecision: domain.DecisionDeny,
		Reason: "reason", ExpectedRevision: revisionPtr(0), Metadata: testMetadata(),
	}

	withoutReason := base
	withoutReason.Reason = " "
	_, err := fixture.service.ReviewDecision(context.Background(), withoutReason)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)

	withoutRevision := base
	withoutRevision.ExpectedRevision = nil
	_, err = fixture.service.ReviewDecision(context.Background(), withoutRevision)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)

	withoutActor := base
	withoutActor.Metadata.ActorID = ""
	_, err = fixture.service.ReviewDecision(context.Background(), withoutActor)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Zero(t, fixture.uow.commits)
}

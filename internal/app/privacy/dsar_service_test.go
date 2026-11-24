package privacy

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

type fakeUUID struct{ next int }

func (f *fakeUUID) NewString() (string, error) {
	f.next++
	return "id-" + string(rune('0'+f.next)), nil
}

type fakeDsarRepo struct {
	stored           map[string]*domain.DsarRequest
	created          *domain.DsarRequest
	createdEvent     *domain.DsarEvent
	updated          *domain.DsarRequest
	updatedEvent     *domain.DsarEvent
	expectedRevision int64
	overdue          []*domain.DsarRequest
	overdueLimit     int
	listLimit        int
	listOffset       int
	getErr           error
	createErr        error
	updateErr        error
	exportRecords    []domain.ExportRecord
	exportRegion     string
	exportErr        error
}

func newFakeDsarRepo() *fakeDsarRepo {
	return &fakeDsarRepo{stored: map[string]*domain.DsarRequest{}}
}

func (f *fakeDsarRepo) Create(ctx context.Context, request *domain.DsarRequest, event *domain.DsarEvent) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = request
	f.createdEvent = event
	f.stored[request.ID] = request
	return nil
}

func (f *fakeDsarRepo) CollectSubjectData(ctx context.Context, organizationID, subjectType, subjectID string) ([]domain.ExportRecord, string, error) {
	if f.exportErr != nil {
		return nil, "", f.exportErr
	}
	return f.exportRecords, f.exportRegion, nil
}

func (f *fakeDsarRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.DsarRequest, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	request, ok := f.stored[id]
	if !ok || request.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("dsar request not found")
	}
	return request, nil
}

func (f *fakeDsarRepo) UpdateWithEvent(ctx context.Context, request *domain.DsarRequest, event *domain.DsarEvent, expectedRevision int64) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = request
	f.updatedEvent = event
	f.expectedRevision = expectedRevision
	f.stored[request.ID] = request
	return nil
}

func (f *fakeDsarRepo) ListOverdue(ctx context.Context, now time.Time, limit int) ([]*domain.DsarRequest, error) {
	f.overdueLimit = limit
	return f.overdue, nil
}

func (f *fakeDsarRepo) ListByOrganization(ctx context.Context, organizationID string, limit, offset int) ([]*domain.DsarRequest, int64, error) {
	f.listLimit = limit
	f.listOffset = offset
	items := make([]*domain.DsarRequest, 0, len(f.stored))
	for _, request := range f.stored {
		if request.OrganizationID == organizationID {
			items = append(items, request)
		}
	}
	return items, int64(len(items)), nil
}

type fakeTx struct {
	consents domain.ConsentRepository
	repo     *fakeDsarRepo
	audits   []AuditEvent
}

func (t *fakeTx) Privacy() domain.DsarRepository { return t.repo }

func (t *fakeTx) Consents() domain.ConsentRepository { return t.consents }

func (t *fakeTx) EmbedAudit(ctx context.Context, event AuditEvent) error {
	t.audits = append(t.audits, event)
	return nil
}

type fakeRunner struct{ tx *fakeTx }

func (r fakeRunner) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error {
	return fn(ctx, r.tx)
}

func newDSARService(repo *fakeDsarRepo, now time.Time) (Service, *fakeTx) {
	tx := &fakeTx{repo: repo}
	return NewService(fakeClock{now: now}, &fakeUUID{}, fakeRunner{tx: tx}), tx
}

func TestService_Create(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	svc, tx := newDSARService(repo, now)
	_ = tx

	request, err := svc.Create(context.Background(), CreateDsarRequestCommand{
		OrganizationID: "org-1",
		SubjectType:    domain.SubjectTypeUser,
		SubjectID:      "user-1",
		RequestType:    domain.RequestTypeAccess,
		ActorID:        "admin-1",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StateReceived, request.State)
	assert.Equal(t, now.Add(domain.SLABudget), request.SLADueAt)
	require.NotNil(t, request.CreatedBy)
	assert.Equal(t, "admin-1", *request.CreatedBy)
	require.NotNil(t, repo.created)
	assert.Equal(t, request.ID, repo.created.ID)
	require.NotNil(t, repo.createdEvent)
	assert.Equal(t, "created", repo.createdEvent.Action)
	assert.Equal(t, request.ID, repo.createdEvent.DsarID)
	require.Len(t, tx.audits, 1)
	assert.Equal(t, "dsar.request.created", tx.audits[0].Action)
	assert.Equal(t, request.ID, tx.audits[0].ResourceID)
}

func TestService_Create_InvalidRequestType(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	svc, tx := newDSARService(repo, now)
	_ = tx

	_, err := svc.Create(context.Background(), CreateDsarRequestCommand{
		OrganizationID: "org-1",
		SubjectType:    domain.SubjectTypeUser,
		SubjectID:      "user-1",
		RequestType:    "delete_all",
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Nil(t, repo.created)
	assert.Nil(t, repo.createdEvent)
}

func TestService_Transition_HappyPath(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	svc, tx := newDSARService(repo, now)
	_ = tx

	request, err := svc.Create(context.Background(), CreateDsarRequestCommand{
		OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeErase, ActorID: "admin-1",
	})
	require.NoError(t, err)

	updated, err := svc.Transition(context.Background(), TransitionDsarRequestCommand{
		OrganizationID: "org-1", ID: request.ID, Target: domain.StateVerifying, ActorID: "admin-1", Revision: request.Revision,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StateVerifying, updated.State)
	assert.Equal(t, int64(2), updated.Revision)
	assert.Equal(t, int64(1), repo.expectedRevision)
	require.NotNil(t, repo.updatedEvent)
	assert.Equal(t, domain.StateVerifying, repo.updatedEvent.Action)
	require.Len(t, tx.audits, 2)
	assert.Equal(t, "dsar.request.updated", tx.audits[1].Action)
	assert.Equal(t, "dsar.request.verifying", tx.audits[1].EventType)
}

func TestService_Transition_RevisionMismatch(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	svc, tx := newDSARService(repo, now)
	_ = tx
	request, err := svc.Create(context.Background(), CreateDsarRequestCommand{
		OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeAccess,
	})
	require.NoError(t, err)

	_, err = svc.Transition(context.Background(), TransitionDsarRequestCommand{
		OrganizationID: "org-1", ID: request.ID, Target: domain.StateVerifying, Revision: 99,
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
	assert.Nil(t, repo.updated)
}

func TestService_Transition_InvalidEdge(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	svc, tx := newDSARService(repo, now)
	_ = tx
	request, err := svc.Create(context.Background(), CreateDsarRequestCommand{
		OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeAccess,
	})
	require.NoError(t, err)

	_, err = svc.Transition(context.Background(), TransitionDsarRequestCommand{
		OrganizationID: "org-1", ID: request.ID, Target: domain.StateFulfilled,
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Nil(t, repo.updated)
}

func TestService_Transition_RequiresRejectionReason(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	svc, tx := newDSARService(repo, now)
	_ = tx
	request, err := svc.Create(context.Background(), CreateDsarRequestCommand{
		OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeAccess,
	})
	require.NoError(t, err)
	_, err = svc.Transition(context.Background(), TransitionDsarRequestCommand{
		OrganizationID: "org-1", ID: request.ID, Target: domain.StateVerifying,
	})
	require.NoError(t, err)
	repo.updated = nil
	_, err = svc.Transition(context.Background(), TransitionDsarRequestCommand{
		OrganizationID: "org-1", ID: request.ID, Target: domain.StateRejected,
	})
	require.Error(t, err)
	assert.Nil(t, repo.updated)

	updated, err := svc.Transition(context.Background(), TransitionDsarRequestCommand{
		OrganizationID: "org-1", ID: request.ID, Target: domain.StateRejected, Reason: "out of scope",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StateRejected, updated.State)
	require.NotNil(t, updated.ClosedAt)
}

func TestService_ListOverdue_DefaultLimit(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	repo.overdue = []*domain.DsarRequest{{ID: "dsar-1"}}
	svc, tx := newDSARService(repo, now)
	_ = tx

	requests, err := svc.ListOverdue(context.Background(), now, 0)
	require.NoError(t, err)
	assert.Len(t, requests, 1)
	assert.Equal(t, 100, repo.overdueLimit)
}

func TestService_ListAndGet(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	svc, tx := newDSARService(repo, now)
	_ = tx

	created, err := svc.Create(context.Background(), CreateDsarRequestCommand{
		OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeAccess,
	})
	require.NoError(t, err)

	items, total, err := svc.List(context.Background(), "org-1", 0, -1)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 50, repo.listLimit)
	assert.Equal(t, 0, repo.listOffset)

	got, err := svc.Get(context.Background(), "org-1", created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	_, err = svc.Get(context.Background(), "org-2", created.ID)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestService_Export_FulfilledAccessRequest(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	repo.stored["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeAccess,
		State: domain.StateFulfilled, SLADueAt: now.Add(domain.SLABudget), Revision: 2,
	}
	repo.exportRecords = []domain.ExportRecord{{Category: "user", SubjectID: "user-1", Payload: map[string]any{"username": "alice"}}}
	repo.exportRegion = "cn"
	svc, tx := newDSARService(repo, now)

	pkg, err := svc.Export(context.Background(), "org-1", "dsar-1", "admin-1")
	require.NoError(t, err)
	require.NotNil(t, pkg)
	assert.Equal(t, "cn", pkg.Region)
	assert.Equal(t, 1, pkg.LineCount)
	assert.NotEmpty(t, pkg.Checksum)
	require.Len(t, tx.audits, 1)
	assert.Equal(t, "privacy.data_exported", tx.audits[0].Action)
	assert.Equal(t, pkg.Checksum, tx.audits[0].Payload["checksum"])
}

func TestService_Export_RequiresFulfilledState(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	repo.stored["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeAccess,
		State: domain.StateInProgress, SLADueAt: now.Add(domain.SLABudget), Revision: 2,
	}
	svc, _ := newDSARService(repo, now)

	_, err := svc.Export(context.Background(), "org-1", "dsar-1", "admin-1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
}

func TestService_Export_RejectsEraseRequest(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	repo.stored["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeErase,
		State: domain.StateFulfilled, SLADueAt: now.Add(domain.SLABudget), Revision: 2,
	}
	svc, _ := newDSARService(repo, now)

	_, err := svc.Export(context.Background(), "org-1", "dsar-1", "admin-1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
}

func TestService_ReviewErasure_AllowedScope(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	repo.stored["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeErase,
		State: domain.StateInProgress, Revision: 2,
	}
	svc, tx := newDSARService(repo, now)
	scope, err := svc.ReviewErasure(context.Background(), ReviewErasureCommand{
		OrganizationID: "org-1", ID: "dsar-1", ActorID: "compliance-1",
		Targets:    []ErasureTarget{{Category: "profile", TargetID: "user-1"}, {Category: "audit", TargetID: "user-1"}},
		Retentions: []RetentionRule{{Category: "audit", LegalBasis: "legal-obligation", Blocks: false}},
	})
	require.NoError(t, err)
	require.Len(t, scope.Erase, 1)
	require.Len(t, scope.Retain, 1)
	assert.Equal(t, "profile", scope.Erase[0].Category)
	require.Len(t, tx.audits, 1)
	assert.Equal(t, "privacy.erasure.reviewed", tx.audits[0].Action)
}

func TestService_ReviewErasure_LegalRetentionBlocks(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	repo.stored["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeErase,
		State: domain.StateInProgress, Revision: 2,
	}
	svc, tx := newDSARService(repo, now)
	_, err := svc.ReviewErasure(context.Background(), ReviewErasureCommand{
		OrganizationID: "org-1", ID: "dsar-1", ActorID: "compliance-1",
		Targets:    []ErasureTarget{{Category: "audit", TargetID: "user-1"}},
		Retentions: []RetentionRule{{Category: "audit", LegalBasis: "legal-obligation", Blocks: true}},
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
	assert.Empty(t, tx.audits)
}

func TestService_ReviewErasure_RequiresEraseRequest(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	repo := newFakeDsarRepo()
	repo.stored["dsar-1"] = &domain.DsarRequest{
		ID: "dsar-1", OrganizationID: "org-1", SubjectType: domain.SubjectTypeUser,
		SubjectID: "user-1", RequestType: domain.RequestTypeAccess,
		State: domain.StateInProgress, Revision: 2,
	}
	svc, _ := newDSARService(repo, now)
	_, err := svc.ReviewErasure(context.Background(), ReviewErasureCommand{
		OrganizationID: "org-1", ID: "dsar-1",
		Targets: []ErasureTarget{{Category: "profile", TargetID: "user-1"}},
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
}

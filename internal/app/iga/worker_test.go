package iga_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/app/iga"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainiga "github.com/guliangiam/guliangiam/internal/domain/iga"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type workerTransactionContextKey struct{}

func workerInTransaction(ctx context.Context) bool {
	inTransaction, _ := ctx.Value(workerTransactionContextKey{}).(bool)
	return inTransaction
}

type mockIGARepo struct {
	domainiga.Repository
	lifetimes       map[string]*domainiga.AccessLifecycle
	listed          []*domainiga.AccessLifecycle
	listOrgs        []string
	listInTx        []bool
	listScopes      []tenantctx.TenantScope
	reclaimInTx     []bool
	failureInTx     []bool
	revokeCalls     int
	sessionRevokes  int
	completeCalls   int
	failureCalls    int
	revokeErr       error
	completedSignal chan struct{}
}

func (m *mockIGARepo) ListExpiringLifecycle(ctx context.Context, orgID string, before time.Time, limit int) ([]*domainiga.AccessLifecycle, error) {
	m.listOrgs = append(m.listOrgs, orgID)
	m.listInTx = append(m.listInTx, workerInTransaction(ctx))
	m.listScopes = append(m.listScopes, tenantctx.TenantScopeFromContext(ctx))
	items := make([]*domainiga.AccessLifecycle, 0)
	candidates := m.listed
	if candidates == nil {
		for _, item := range m.lifetimes {
			candidates = append(candidates, item)
		}
	}
	for _, item := range candidates {
		if item.Status == domainiga.LifecycleActive && !item.ExpiresAt.After(before) && (orgID == "" || item.OrganizationID == orgID) {
			items = append(items, item)
		}
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (m *mockIGARepo) GetLifecycleForUpdate(ctx context.Context, orgID, id string) (*domainiga.AccessLifecycle, error) {
	m.reclaimInTx = append(m.reclaimInTx, workerInTransaction(ctx))
	item := m.lifetimes[id]
	if item == nil || item.OrganizationID != orgID {
		return nil, errors.New("lifecycle not found")
	}
	return item, nil
}

func (m *mockIGARepo) RevokeJITArtifacts(context.Context, string, string) error {
	m.revokeCalls++
	return m.revokeErr
}

func (m *mockIGARepo) RevokePrivilegedSessions(context.Context, string, time.Time) (int64, error) {
	m.sessionRevokes++
	return 1, nil
}

func (m *mockIGARepo) CompleteLifecycle(_ context.Context, orgID, id string, status domainiga.LifecycleStatus, at time.Time) (bool, error) {
	item := m.lifetimes[id]
	if item == nil || item.OrganizationID != orgID || item.Status != domainiga.LifecycleActive {
		return false, nil
	}
	item.Status = status
	item.RevokedAt = &at
	m.completeCalls++
	if m.completedSignal != nil {
		m.completedSignal <- struct{}{}
	}
	return true, nil
}

func (m *mockIGARepo) RecordLifecycleFailure(ctx context.Context, _, _, _ string) error {
	m.failureInTx = append(m.failureInTx, workerInTransaction(ctx))
	m.failureCalls++
	return nil
}

type stubUOW struct {
	appuow.UnitOfWork
	repo              *mockIGARepo
	transactionScopes []tenantctx.TenantScope
}

func (s *stubUOW) IGA() domainiga.Repository         { return s.repo }
func (s *stubUOW) Records() domainrecords.Repository { return nil }
func (s *stubUOW) Outbox() domainoutbox.Repository   { return nil }
func (s *stubUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	s.transactionScopes = append(s.transactionScopes, tenantctx.TenantScopeFromContext(ctx))
	return fn(context.WithValue(ctx, workerTransactionContextKey{}, true), s)
}

var _ appuow.UnitOfWork = (*stubUOW)(nil)

func expiredLifecycle(id, orgID string) *domainiga.AccessLifecycle {
	return &domainiga.AccessLifecycle{
		ID: id, OrganizationID: orgID, UserID: "user-1", RoleID: "role-1",
		PolicyID: "policy-1", Status: domainiga.LifecycleActive,
		StartsAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}
}

func runUntilCompleted(t *testing.T, worker iga.ExpireWorker, repo *mockIGARepo, count int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	for i := 0; i < count; i++ {
		select {
		case <-repo.completedSignal:
		case <-time.After(time.Second):
			t.Fatal("worker did not reclaim lifecycle")
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("worker stopped with unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestExpireWorker_ReclaimsResidualWithoutBinding(t *testing.T) {
	item := expiredLifecycle("lc-residual", "org-a")
	repo := &mockIGARepo{lifetimes: map[string]*domainiga.AccessLifecycle{item.ID: item}, completedSignal: make(chan struct{}, 2)}
	uow := &stubUOW{repo: repo}
	worker := iga.NewExpireWorker(uow, time.Hour, 100, func() time.Time {
		return time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	}, []string{"org-a"})

	runUntilCompleted(t, worker, repo, 1)
	assert.Equal(t, 1, repo.revokeCalls, "policy-only residue must still be reclaimed")
	assert.Equal(t, 1, repo.sessionRevokes)
	assert.Equal(t, domainiga.LifecycleExpired, item.Status)
	require.Equal(t, []bool{true}, repo.listInTx)
	require.Equal(t, []bool{true}, repo.reclaimInTx)
	require.NotEmpty(t, repo.listScopes)
	assert.Equal(t, tenantctx.TenantFilterSystem, repo.listScopes[0].Mode)
	require.NotEmpty(t, uow.transactionScopes)
	for _, scope := range uow.transactionScopes {
		assert.Equal(t, tenantctx.TenantFilterSystem, scope.Mode)
	}
}

func TestExpireWorker_RepeatedScanHasNoDuplicateSideEffects(t *testing.T) {
	item := expiredLifecycle("lc-idempotent", "org-a")
	repo := &mockIGARepo{lifetimes: map[string]*domainiga.AccessLifecycle{item.ID: item}, completedSignal: make(chan struct{}, 2)}
	uow := &stubUOW{repo: repo}
	now := func() time.Time { return time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC) }
	runUntilCompleted(t, iga.NewExpireWorker(uow, time.Hour, 100, now, []string{"org-a"}), repo, 1)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	second := iga.NewExpireWorker(uow, time.Hour, 100, now, []string{"org-a"})
	go func() { done <- second.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	assert.Equal(t, 1, repo.revokeCalls)
	assert.Equal(t, 1, repo.sessionRevokes)
	assert.Equal(t, 1, repo.completeCalls)
}

func TestExpireWorker_FailureIsRecordedAndRetried(t *testing.T) {
	item := expiredLifecycle("lc-retry", "org-a")
	repo := &mockIGARepo{lifetimes: map[string]*domainiga.AccessLifecycle{item.ID: item}, revokeErr: errors.New("delete failed"), completedSignal: make(chan struct{}, 1)}
	uow := &stubUOW{repo: repo}
	now := func() time.Time { return time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC) }
	worker := iga.NewExpireWorker(uow, 5*time.Millisecond, 100, now, []string{"org-a"})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	time.Sleep(15 * time.Millisecond)
	require.GreaterOrEqual(t, repo.failureCalls, 1)
	repo.revokeErr = nil
	select {
	case <-repo.completedSignal:
	case <-time.After(time.Second):
		t.Fatal("worker did not retry failed reclaim")
	}
	cancel()
	// The tick failure recorded earlier is the error the worker exits with.
	require.Error(t, <-done)
	assert.Equal(t, domainiga.LifecycleExpired, item.Status)
}

func TestExpireWorker_RereadsAndSkipsFutureGrant(t *testing.T) {
	current := expiredLifecycle("lc-future", "org-a")
	current.StartsAt = time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	current.ExpiresAt = time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	staleCandidate := expiredLifecycle(current.ID, current.OrganizationID)
	repo := &mockIGARepo{
		lifetimes: map[string]*domainiga.AccessLifecycle{current.ID: current},
		listed:    []*domainiga.AccessLifecycle{staleCandidate},
	}
	worker := iga.NewExpireWorker(&stubUOW{repo: repo}, time.Hour, 100, func() time.Time {
		return time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	}, []string{"org-a"})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)

	assert.Zero(t, repo.revokeCalls)
	assert.Zero(t, repo.sessionRevokes)
	assert.Zero(t, repo.completeCalls)
	assert.Equal(t, domainiga.LifecycleActive, current.Status)
}

func TestExpireWorker_EmptyOrgsUsesGlobalScan(t *testing.T) {
	repo := &mockIGARepo{lifetimes: map[string]*domainiga.AccessLifecycle{}}
	uow := &stubUOW{repo: repo}
	worker := iga.NewExpireWorker(uow, time.Hour, 100, time.Now, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	assert.ErrorIs(t, <-done, context.Canceled)
	require.NotEmpty(t, repo.listOrgs)
	assert.Equal(t, "", repo.listOrgs[0])
	require.Equal(t, []bool{true}, repo.listInTx)
	require.NotEmpty(t, repo.listScopes)
	assert.Equal(t, tenantctx.TenantFilterSystem, repo.listScopes[0].Mode)
}

func TestExpireAccesses_ScansCandidatesInsideTenantTransaction(t *testing.T) {
	repo := &mockIGARepo{lifetimes: map[string]*domainiga.AccessLifecycle{}}
	uow := &stubUOW{repo: repo}
	service := iga.NewService(uow)
	ctx := tenantctx.WithTenantScope(context.Background(), "org-a", "acme", tenantctx.TenantFilterStrict)

	count, err := service.ExpireAccesses(ctx, "org-a", time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Zero(t, count)
	require.Equal(t, []bool{true}, repo.listInTx)
	require.NotEmpty(t, repo.listScopes)
	assert.Equal(t, tenantctx.TenantFilterStrict, repo.listScopes[0].Mode)
	assert.Equal(t, "org-a", repo.listScopes[0].OrganizationID)
}

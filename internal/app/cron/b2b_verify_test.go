package cron

import (
	"context"
	"log/slog"
	"testing"
	"time"

	appb2b "github.com/guliangiam/guliangiam/internal/app/b2b"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainb2b "github.com/guliangiam/guliangiam/internal/domain/b2b"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestNewB2BVerifyWorkerDefaults(t *testing.T) {
	w := NewB2BVerifyWorker(appb2b.Service{})
	if w.interval != 2*time.Minute {
		t.Fatalf("interval = %v, want 2m", w.interval)
	}
	if w.log == nil || w.verified == nil || w.failed == nil {
		t.Fatalf("worker defaults incomplete: %+v", w)
	}
}

func TestWorkerOptionsOverrideIntervalAndLogger(t *testing.T) {
	w := NewB2BVerifyWorker(appb2b.Service{}, WithInterval(30*time.Second))
	if w.interval != 30*time.Second {
		t.Fatalf("interval = %v, want 30s", w.interval)
	}
}

// ── Worker 运行路径 ────────────────────────────────────────────────────────

type workerTestDNS struct {
	records map[string][]string
}

func (d workerTestDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	return d.records[name], nil
}

type workerTestRepo struct {
	domainb2b.Repository // 未实现的接口方法仅在被调用时 panic
	pending              []*domainb2b.OrganizationDomain
	updated              []*domainb2b.OrganizationDomain
	claimScope           tenantctx.TenantScope
	claimLimit           int
	claimLease           time.Duration
	updateErr            error
}

func (r *workerTestRepo) ClaimPending(ctx context.Context, limit int, lease time.Duration) ([]*domainb2b.OrganizationDomain, error) {
	r.claimScope = tenantctx.TenantScopeFromContext(ctx)
	r.claimLimit = limit
	r.claimLease = lease

	now := time.Now()
	out := make([]*domainb2b.OrganizationDomain, 0, limit)
	for _, d := range r.pending {
		if len(out) >= limit {
			break
		}
		if d.IsDue(now) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *workerTestRepo) Update(_ context.Context, d *domainb2b.OrganizationDomain) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updated = append(r.updated, d)
	if d.Status == domainb2b.StatusPending {
		return nil
	}
	for i, p := range r.pending {
		if p.ID == d.ID {
			r.pending = append(r.pending[:i], r.pending[i+1:]...)
			break
		}
	}
	return nil
}

type workerTestUOW struct {
	appuow.UnitOfWork // 未实现的方法仅在被调用时 panic
	repo              *workerTestRepo
}

func (u *workerTestUOW) B2BDomains() domainb2b.Repository { return u.repo }

func (u *workerTestUOW) Records() domainrecords.Repository { return nil }

func (u *workerTestUOW) Outbox() domainoutbox.Repository { return nil }

func (u *workerTestUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newWorkerTestService(repo *workerTestRepo, dns workerTestDNS) appb2b.Service {
	return appb2b.NewServiceWithDNS(
		clock.SystemClock{}, platformuuid.V7Generator{},
		&workerTestUOW{repo: repo}, dns,
	)
}

// counterValue 从默认 registry 读取指标值 —— 与 /metrics 的采集面一致。
func counterValue(t *testing.T, name string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather default registry: %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		var total float64
		for _, metric := range family.GetMetric() {
			total += metric.GetCounter().GetValue()
		}
		return total
	}
	return 0
}

func TestB2BVerifyWorker_RunVerifiesPendingAndCounts(t *testing.T) {
	const token = "GuliangIAM-Verification=v1:test-token"
	repo := &workerTestRepo{pending: []*domainb2b.OrganizationDomain{{
		ID: "domain-1", OrganizationID: "org-1", Domain: "corp.example.com",
		VerificationToken: token, Status: domainb2b.StatusPending,
	}}}
	dns := workerTestDNS{records: map[string][]string{
		"_guliangiam-verification.corp.example.com": {token},
	}}

	w := NewB2BVerifyWorker(
		newWorkerTestService(repo, dns),
		WithInterval(10*time.Millisecond),
		WithLogger(slog.New(slog.DiscardHandler)),
	)
	before := counterValue(t, "b2b_domain_verified_total")

	w.Start(context.Background())
	deadline := time.Now().Add(2 * time.Second)
	for counterValue(t, "b2b_domain_verified_total") == before {
		if time.Now().After(deadline) {
			w.Stop()
			t.Fatal("verified counter did not increment")
		}
		time.Sleep(5 * time.Millisecond)
	}
	w.Stop()

	if got := counterValue(t, "b2b_domain_verified_total") - before; got != 1 {
		t.Fatalf("verified counter delta = %v, want 1", got)
	}
	if repo.claimScope.Mode != tenantctx.TenantFilterSystem {
		t.Fatalf("ClaimPending scope = %q, want system", repo.claimScope.Mode)
	}
	if repo.claimLimit != b2bClaimBatch {
		t.Fatalf("claim limit = %d, want %d", repo.claimLimit, b2bClaimBatch)
	}
	if repo.claimLease != b2bClaimLease {
		t.Fatalf("claim lease = %v, want %v", repo.claimLease, b2bClaimLease)
	}
	if len(repo.updated) != 1 || repo.updated[0].Status != domainb2b.StatusVerified {
		t.Fatalf("updated = %+v, want one verified row", repo.updated)
	}
	if len(repo.pending) != 0 {
		t.Fatalf("pending = %d, want 0 after verification", len(repo.pending))
	}
}

func TestB2BVerifyWorker_RetryScheduledDoesNotCountFailure(t *testing.T) {
	repo := &workerTestRepo{pending: []*domainb2b.OrganizationDomain{{
		ID: "domain-retry", OrganizationID: "org-1", Domain: "retry.example.com",
		VerificationToken: "expected-token", Status: domainb2b.StatusPending,
	}}}
	w := NewB2BVerifyWorker(
		newWorkerTestService(repo, workerTestDNS{}), // 无 TXT → txt_missing
		WithInterval(time.Hour),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	before := counterValue(t, "b2b_domain_failed_total")
	w.run(context.Background())

	if got := counterValue(t, "b2b_domain_failed_total") - before; got != 0 {
		t.Fatalf("failed counter delta = %v, want 0 for retry", got)
	}
	require.Len(t, repo.updated, 1)
	if repo.updated[0].Status != domainb2b.StatusPending {
		t.Fatalf("status = %q, want pending", repo.updated[0].Status)
	}
	if repo.updated[0].Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", repo.updated[0].Attempts)
	}
	if repo.updated[0].NextRetryAt.IsZero() {
		t.Fatal("next_retry_at must be scheduled")
	}
}

func TestB2BVerifyWorker_TerminalFailureCountsOnce(t *testing.T) {
	repo := &workerTestRepo{pending: []*domainb2b.OrganizationDomain{{
		ID: "domain-fail", OrganizationID: "org-1", Domain: "fail.example.com",
		VerificationToken: "expected-token", Status: domainb2b.StatusPending, Attempts: 4,
	}}}
	w := NewB2BVerifyWorker(
		newWorkerTestService(repo, workerTestDNS{}),
		WithInterval(time.Hour),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	before := counterValue(t, "b2b_domain_failed_total")
	w.run(context.Background())

	if got := counterValue(t, "b2b_domain_failed_total") - before; got != 1 {
		t.Fatalf("failed counter delta = %v, want 1", got)
	}
	require.Len(t, repo.updated, 1)
	if repo.updated[0].Status != domainb2b.StatusFailed {
		t.Fatalf("status = %q, want failed", repo.updated[0].Status)
	}
	if repo.updated[0].Attempts != 5 {
		t.Fatalf("attempts = %d, want 5", repo.updated[0].Attempts)
	}
}

func TestB2BVerifyWorker_ConflictSkipsFailureMetric(t *testing.T) {
	repo := &workerTestRepo{
		pending: []*domainb2b.OrganizationDomain{{
			ID: "domain-conflict", OrganizationID: "org-1", Domain: "conflict.example.com",
			VerificationToken: "token", Status: domainb2b.StatusPending,
		}},
		updateErr: apperrors.NewConflict("already terminalized"),
	}
	w := NewB2BVerifyWorker(
		newWorkerTestService(repo, workerTestDNS{records: map[string][]string{
			"_guliangiam-verification.conflict.example.com": {"token"},
		}}),
		WithInterval(time.Hour),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	verifiedBefore := counterValue(t, "b2b_domain_verified_total")
	failedBefore := counterValue(t, "b2b_domain_failed_total")
	w.run(context.Background())

	if got := counterValue(t, "b2b_domain_verified_total") - verifiedBefore; got != 0 {
		t.Fatalf("verified counter delta = %v, want 0", got)
	}
	if got := counterValue(t, "b2b_domain_failed_total") - failedBefore; got != 0 {
		t.Fatalf("failed counter delta = %v, want 0", got)
	}
	if len(repo.updated) != 0 {
		t.Fatalf("updated = %d, want 0 on conflict", len(repo.updated))
	}
}

func TestFailureCode(t *testing.T) {
	cases := map[string]string{
		"token_mismatch: TXT record does not contain the expected token": "token_mismatch",
		"dns_nxdomain: TXT record not found":                             "dns_nxdomain",
		"barecode":                                                       "barecode",
		"":                                                               "unknown",
	}
	for input, want := range cases {
		if got := failureCode(input); got != want {
			t.Errorf("failureCode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestB2BVerifyWorker_StartStopSemantics(t *testing.T) {
	repo := &workerTestRepo{}
	w := NewB2BVerifyWorker(
		newWorkerTestService(repo, workerTestDNS{}),
		WithInterval(time.Hour),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	w.Start(context.Background())
	w.mu.Lock()
	firstDone := w.done
	w.mu.Unlock()

	w.Start(context.Background()) // 已启动 → no-op
	w.mu.Lock()
	secondDone := w.done
	w.mu.Unlock()
	if firstDone != secondDone {
		t.Fatal("second Start replaced the running loop")
	}

	w.Stop()
	w.mu.Lock()
	if w.cancel != nil || w.done != nil {
		t.Fatalf("Stop left lifecycle fields set: cancel=%v done=%v", w.cancel, w.done)
	}
	w.mu.Unlock()

	w.Stop() // 未启动 → no-op
}

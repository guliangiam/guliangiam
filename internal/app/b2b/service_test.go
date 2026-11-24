package b2b

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainb2b "github.com/guliangiam/guliangiam/internal/domain/b2b"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
)

// ── fake DNSResolver ────────────────────────────────────────────────────────

type fakeDNSResolver struct {
	txtRecords map[string][]string // name → TXT records
	lookupErr  error               // 强制 lookup 失败
}

func newFakeDNS() *fakeDNSResolver { return &fakeDNSResolver{txtRecords: map[string][]string{}} }

func (f *fakeDNSResolver) with(name string, records ...string) {
	f.txtRecords[name] = records
}

func (f *fakeDNSResolver) withLookupError(err error) *fakeDNSResolver {
	f.lookupErr = err
	return f
}

func (f *fakeDNSResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	return f.txtRecords[name], nil
}

// ── fake B2B Repository ─────────────────────────────────────────────────────

type fakeRepo struct {
	items       map[string]*domainb2b.OrganizationDomain // id → entity
	byDomain    map[string]*domainb2b.OrganizationDomain // (orgID + "|" + domain) → entity
	domainIndex map[string]*domainb2b.OrganizationDomain // domain → entity（跨租户）
	listPending []*domainb2b.OrganizationDomain
	createErr   error
	listErr     error
	claimErr    error
	updateErr   error
	claimCalls  int
	claimLimit  int
	claimLease  time.Duration
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		items:       map[string]*domainb2b.OrganizationDomain{},
		byDomain:    map[string]*domainb2b.OrganizationDomain{},
		domainIndex: map[string]*domainb2b.OrganizationDomain{},
	}
}

func (f *fakeRepo) Create(_ context.Context, d *domainb2b.OrganizationDomain) error {
	if f.createErr != nil {
		return f.createErr
	}
	if _, exists := f.byDomain[d.OrganizationID+"|"+d.Domain]; exists {
		return apperrors.NewAlreadyExistsf("domain %q already claimed", d.Domain)
	}
	f.items[d.ID] = d
	f.byDomain[d.OrganizationID+"|"+d.Domain] = d
	if d.Status == domainb2b.StatusVerified {
		f.domainIndex[d.Domain] = d
	}
	if d.Status == domainb2b.StatusPending {
		f.listPending = append(f.listPending, d)
	}
	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (*domainb2b.OrganizationDomain, error) {
	d, ok := f.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("not found")
	}
	return d, nil
}

func (f *fakeRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domainb2b.OrganizationDomain, error) {
	d, ok := f.items[id]
	if !ok || d.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("not found")
	}
	return d, nil
}

func (f *fakeRepo) GetByDomain(_ context.Context, orgID, domain string) (*domainb2b.OrganizationDomain, error) {
	d := f.byDomain[orgID+"|"+domain]
	return d, nil
}

func (f *fakeRepo) FindVerifiedByDomain(_ context.Context, name string) (*domainb2b.OrganizationDomain, error) {
	return f.domainIndex[name], nil
}

func (f *fakeRepo) List(_ context.Context, _ domainb2b.ListQuery) ([]*domainb2b.OrganizationDomain, int64, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	out := make([]*domainb2b.OrganizationDomain, 0, len(f.items))
	for _, d := range f.items {
		out = append(out, d)
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) ClaimPending(_ context.Context, limit int, lease time.Duration) ([]*domainb2b.OrganizationDomain, error) {
	f.claimCalls++
	f.claimLimit = limit
	f.claimLease = lease
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	now := time.Now()
	out := make([]*domainb2b.OrganizationDomain, 0, limit)
	for _, d := range f.listPending {
		if len(out) >= limit {
			break
		}
		if !d.IsDue(now) {
			continue
		}
		d.NextRetryAt = now.Add(lease)
		d.UpdatedAt = now
		out = append(out, d)
	}
	return out, nil
}

func (f *fakeRepo) Update(_ context.Context, d *domainb2b.OrganizationDomain) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	old := f.items[d.ID]
	if old != nil {
		// 同步更新跨租户 domainIndex（verified → 加入 / 其他 → 移除）
		if old.Status == domainb2b.StatusVerified {
			delete(f.domainIndex, old.Domain)
		}
		if d.Status == domainb2b.StatusVerified {
			f.domainIndex[d.Domain] = d
		}
		// 更新 listPending（pending 变更 → 从 pending 切片移除）
		if old.Status == domainb2b.StatusPending && d.Status != domainb2b.StatusPending {
			for i, p := range f.listPending {
				if p.ID == d.ID {
					f.listPending = append(f.listPending[:i], f.listPending[i+1:]...)
					break
				}
			}
		}
	}
	f.items[d.ID] = d
	f.byDomain[d.OrganizationID+"|"+d.Domain] = d
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id, organizationID string) error {
	d, ok := f.items[id]
	if !ok || d.OrganizationID != organizationID {
		return apperrors.NewNotFound("not found")
	}
	if d != nil {
		delete(f.domainIndex, d.Domain)
		delete(f.byDomain, d.OrganizationID+"|"+d.Domain)
	}
	delete(f.items, id)
	return nil
}

// ── 测试夹具 ────────────────────────────────────────────────────────────────

type auditRecordsRepo struct {
	domainrecords.Repository
	created []*domainrecords.Record
}

func (r *auditRecordsRepo) Create(_ context.Context, record *domainrecords.Record) error {
	r.created = append(r.created, record)
	return nil
}

type auditOutboxRepo struct {
	events []*domainoutbox.OutboxEvent
}

func (r *auditOutboxRepo) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *auditOutboxRepo) EnqueueBatch(_ context.Context, events []*domainoutbox.OutboxEvent) error {
	r.events = append(r.events, events...)
	return nil
}

type auditTestUOW struct {
	appuow.UnitOfWork
	repo         *fakeRepo
	records      *auditRecordsRepo
	outbox       *auditOutboxRepo
	transactions int
}

func (u *auditTestUOW) B2BDomains() domainb2b.Repository  { return u.repo }
func (u *auditTestUOW) Records() domainrecords.Repository { return u.records }
func (u *auditTestUOW) Outbox() domainoutbox.Repository   { return u.outbox }
func (u *auditTestUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	u.transactions++
	return fn(ctx, u)
}

func setupAudit(t *testing.T) (Service, *fakeDNSResolver, *fakeRepo, *auditTestUOW) {
	t.Helper()
	repo := newFakeRepo()
	uow := &auditTestUOW{
		repo: repo, records: &auditRecordsRepo{}, outbox: &auditOutboxRepo{},
	}
	uuidGen := &stubUUIDGenerator{values: []string{
		"domain-id-001", "token-aaa-bbb-ccc", "domain-id-002", "token-xxx-yyy-zzz",
	}}
	dns := newFakeDNS()
	return NewServiceWithDNS(clock.SystemClock{}, uuidGen, uow, dns), dns, repo, uow
}

// stubUUIDGenerator 让每次 NewString() 返回可预测序列。
type stubUUIDGenerator struct {
	values []string
	cursor int
}

func (s *stubUUIDGenerator) NewString() (string, error) {
	if s.cursor >= len(s.values) {
		s.cursor = 0
	}
	v := s.values[s.cursor]
	s.cursor++
	return v, nil
}

func setup(t *testing.T) (Service, *fakeDNSResolver, *fakeRepo) {
	t.Helper()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := newFakeRepo()
	dns := newFakeDNS()
	uuidGen := &stubUUIDGenerator{values: []string{
		"domain-id-001",
		"token-aaa-bbb-ccc",
		"domain-id-002",
		"token-xxx-yyy-zzz",
	}}

	uow.EXPECT().B2BDomains().AnyTimes().Return(repo)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)

	svc := NewServiceWithDNS(clock.SystemClock{}, uuidGen, uow, dns)
	return svc, dns, repo
}

//go:generate 声明（占位 —— domain/b2b 尚无 mockgen）
var _ appuow.UnitOfWork = (*uowmock.MockUnitOfWork)(nil)
var _ domainb2b.Repository = (*fakeRepo)(nil)

// ── ClaimDomain 测试 ────────────────────────────────────────────────────────

func TestClaimDomain_Success(t *testing.T) {
	svc, _, repo := setup(t)

	dto, err := svc.ClaimDomain(context.Background(), ClaimDomainCommand{
		OrganizationID: "org-001",
		Domain:         "corp.example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "domain-id-001", dto.ID)
	assert.Equal(t, "GuliangIAM-Verification=v1:token-aaa-bbb-ccc", dto.VerificationToken)
	assert.Equal(t, domainb2b.StatusPending, dto.Status)

	// fakeRepo 内部状态验证
	stored := repo.items[dto.ID]
	require.NotNil(t, stored)
	assert.Equal(t, "corp.example.com", stored.Domain)
	assert.Equal(t, "org-001", stored.OrganizationID)
}

func TestClaimDomain_NormalizesDNSName(t *testing.T) {
	svc, _, repo := setup(t)

	dto, err := svc.ClaimDomain(context.Background(), ClaimDomainCommand{
		OrganizationID: "org-001", Domain: " Corp.Example.COM. ",
	})
	require.NoError(t, err)
	assert.Equal(t, "corp.example.com", dto.Domain)
	assert.Equal(t, "corp.example.com", repo.items[dto.ID].Domain)
}

func TestClaimDomain_RejectsIP(t *testing.T) {
	svc, _, _ := setup(t)

	_, err := svc.ClaimDomain(context.Background(), ClaimDomainCommand{
		OrganizationID: "org-001",
		Domain:         "192.168.1.1",
	})
	assert.Error(t, err)
}

func TestClaimDomain_RejectsBareHostname(t *testing.T) {
	svc, _, _ := setup(t)

	_, err := svc.ClaimDomain(context.Background(), ClaimDomainCommand{
		OrganizationID: "org-001",
		Domain:         "localhost",
	})
	assert.Error(t, err)
}

func TestClaimDomain_RejectsEmpty(t *testing.T) {
	svc, _, _ := setup(t)

	_, err := svc.ClaimDomain(context.Background(), ClaimDomainCommand{
		OrganizationID: "org-001",
		Domain:         "",
	})
	assert.Error(t, err)
}

func TestClaimDomain_DuplicateClaim(t *testing.T) {
	svc, _, repo := setup(t)

	_, err := svc.ClaimDomain(context.Background(), ClaimDomainCommand{
		OrganizationID: "org-001",
		Domain:         "corp.example.com",
	})
	require.NoError(t, err)

	// 同一组织认领同一域名 → fakeRepo 应返回 AlreadyExists
	repo.createErr = apperrors.NewAlreadyExists("duplicate")
	_, err = svc.ClaimDomain(context.Background(), ClaimDomainCommand{
		OrganizationID: "org-001",
		Domain:         "corp.example.com",
	})
	assert.Error(t, err)
}

// ── VerifySingle 测试 ──────────────────────────────────────────────────────

func TestVerifySingle_MatchesToken(t *testing.T) {
	svc, dns, repo := setup(t)

	// 先认领，再预注入 pending 状态
	pending := &domainb2b.OrganizationDomain{
		ID:                "pending-001",
		OrganizationID:    "org-001",
		Domain:            "corp.example.com",
		VerificationToken: "GuliangIAM-Verification=v1:mytoken",
		Status:            domainb2b.StatusPending,
		CreatedAt:         time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
		UpdatedAt:         time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
	}
	repo.items[pending.ID] = pending
	repo.byDomain[pending.OrganizationID+"|"+pending.Domain] = pending
	repo.listPending = append(repo.listPending, pending)

	// fake DNS 返回正确 token
	dns.with("_guliangiam-verification.corp.example.com", "GuliangIAM-Verification=v1:mytoken")

	dto, err := svc.VerifySingle(context.Background(), pending, DefaultVerificationPolicy())
	require.NoError(t, err)
	assert.Equal(t, domainb2b.StatusVerified, dto.Status)
	assert.False(t, dto.VerifiedAt.IsZero())

	// 仓库状态同步更新
	stored := repo.items["pending-001"]
	assert.Equal(t, domainb2b.StatusVerified, stored.Status)
}

func TestVerifySingle_NoMatch_SchedulesRetry(t *testing.T) {
	svc, dns, repo := setup(t)

	pending := &domainb2b.OrganizationDomain{
		ID:                "pending-002",
		OrganizationID:    "org-001",
		Domain:            "corp.example.com",
		VerificationToken: "GuliangIAM-Verification=v1:expected",
		Status:            domainb2b.StatusPending,
	}
	repo.items[pending.ID] = pending

	// DNS 返回错误 token
	dns.with("_guliangiam-verification.corp.example.com", "wrong-token")

	dto, err := svc.VerifySingle(context.Background(), pending, DefaultVerificationPolicy())
	require.NoError(t, err)
	assert.Equal(t, domainb2b.StatusPending, dto.Status)
	assert.Equal(t, 1, dto.Attempts)
	assert.Contains(t, dto.LastError, "token_mismatch")
	assert.False(t, dto.NextRetryAt.IsZero())
	assert.False(t, dto.LastCheckedAt.IsZero())
}

func TestVerifySingle_NoMatch_ExhaustsBudget(t *testing.T) {
	svc, dns, repo := setup(t)

	pending := &domainb2b.OrganizationDomain{
		ID:                "pending-budget",
		OrganizationID:    "org-001",
		Domain:            "corp.example.com",
		VerificationToken: "GuliangIAM-Verification=v1:expected",
		Status:            domainb2b.StatusPending,
	}
	repo.items[pending.ID] = pending
	dns.with("_guliangiam-verification.corp.example.com", "wrong-token")

	policy := VerificationPolicy{MaxAttempts: 2, Backoff: VerificationBackoff}
	for i := 0; i < 2; i++ {
		out, err := svc.VerifySingle(context.Background(), pending, policy)
		require.NoError(t, err)
		if i == 0 {
			assert.Equal(t, domainb2b.StatusPending, out.Status)
			assert.Equal(t, 1, out.Attempts)
		} else {
			assert.Equal(t, domainb2b.StatusFailed, out.Status)
			assert.Equal(t, 2, out.Attempts)
		}
	}
	if repo.items[pending.ID].Status != domainb2b.StatusFailed {
		t.Fatalf("stored status = %q, want failed", repo.items[pending.ID].Status)
	}
}

func TestVerifySingle_DNSLookupError_SchedulesRetry(t *testing.T) {
	svc, dns, repo := setup(t)

	pending := &domainb2b.OrganizationDomain{
		ID:                "pending-003",
		OrganizationID:    "org-001",
		Domain:            "corp.example.com",
		VerificationToken: "any",
		Status:            domainb2b.StatusPending,
	}
	repo.items[pending.ID] = pending

	dns.withLookupError(stderrors.New("no such host"))

	dto, err := svc.VerifySingle(context.Background(), pending, DefaultVerificationPolicy())
	require.NoError(t, err)
	assert.Equal(t, domainb2b.StatusPending, dto.Status)
	assert.Equal(t, 1, dto.Attempts)
	assert.Contains(t, dto.LastError, "dns_error")
	// 不写原始错误文本（避免解析器地址等信息进入审计/排障字段）。
	assert.NotContains(t, dto.LastError, "no such host")
}

func TestVerifySingle_ConflictNotCountedAsBusinessFailure(t *testing.T) {
	svc, dns, repo := setup(t)
	pending := &domainb2b.OrganizationDomain{
		ID: "pending-conflict", OrganizationID: "org-001", Domain: "corp.example.com",
		VerificationToken: "tok", Status: domainb2b.StatusPending,
	}
	repo.items[pending.ID] = pending
	dns.with("_guliangiam-verification.corp.example.com", "tok")
	repo.updateErr = apperrors.NewConflict("already terminalized")

	_, err := svc.VerifySingle(context.Background(), pending, DefaultVerificationPolicy())
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
}

// ── FindForSSO 测试 ────────────────────────────────────────────────────────

func TestFindForSSO_VerifiedDomain(t *testing.T) {
	svc, _, repo := setup(t)

	verified := &domainb2b.OrganizationDomain{
		ID:             "verified-001",
		OrganizationID: "org-001",
		Domain:         "corp.example.com",
		Status:         domainb2b.StatusVerified,
	}
	repo.items[verified.ID] = verified
	repo.byDomain["org-001|corp.example.com"] = verified
	repo.domainIndex["corp.example.com"] = verified

	dto, err := svc.FindForSSO(context.Background(), "alice@corp.example.com")
	require.NoError(t, err)
	require.NotNil(t, dto)
	assert.Equal(t, "org-001", dto.OrganizationID)
	assert.Equal(t, "corp.example.com", dto.Domain)
}

func TestFindForSSO_NormalizesEmailDomain(t *testing.T) {
	svc, _, repo := setup(t)
	verified := &domainb2b.OrganizationDomain{
		ID: "verified-001", OrganizationID: "org-001", Domain: "corp.example.com", Status: domainb2b.StatusVerified,
	}
	repo.domainIndex[verified.Domain] = verified

	dto, err := svc.FindForSSO(context.Background(), "alice@CORP.EXAMPLE.COM.")
	require.NoError(t, err)
	require.NotNil(t, dto)
	assert.Equal(t, "org-001", dto.OrganizationID)
}

func TestFindForSSO_UnclaimedDomain(t *testing.T) {
	svc, _, _ := setup(t)

	dto, err := svc.FindForSSO(context.Background(), "bob@unknown.com")
	require.NoError(t, err)
	assert.Nil(t, dto)
}

func TestFindForSSO_InvalidEmail_NoAt(t *testing.T) {
	svc, _, _ := setup(t)

	dto, err := svc.FindForSSO(context.Background(), "bob-no-at")
	require.NoError(t, err)
	assert.Nil(t, dto)
}

func TestFindForSSO_EmailEndsWithAt(t *testing.T) {
	svc, _, _ := setup(t)

	dto, err := svc.FindForSSO(context.Background(), "bob@")
	require.NoError(t, err)
	assert.Nil(t, dto)
}

// ── ListDomains 测试 ────────────────────────────────────────────────────────

func TestListDomains_Success(t *testing.T) {
	svc, _, repo := setup(t)

	d1 := &domainb2b.OrganizationDomain{
		ID: "d1", OrganizationID: "org-001", Domain: "a.example.com", Status: domainb2b.StatusPending,
	}
	d2 := &domainb2b.OrganizationDomain{
		ID: "d2", OrganizationID: "org-001", Domain: "b.example.com", Status: domainb2b.StatusVerified,
	}
	repo.items[d1.ID] = d1
	repo.items[d2.ID] = d2

	items, total, err := svc.ListDomains(context.Background(), ListDomainsQuery{
		OrganizationID: "org-001", Page: 1, PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}

func TestListDomains_Error(t *testing.T) {
	svc, _, repo := setup(t)
	repo.listErr = stderrors.New("db down")

	_, _, err := svc.ListDomains(context.Background(), ListDomainsQuery{
		OrganizationID: "org-001", Page: 1, PageSize: 10,
	})
	assert.Error(t, err)
}

// ── ClaimPending (Cron 入口) ────────────────────────────────────────────────

func TestClaimPending_ReturnsOnlyDuePending(t *testing.T) {
	svc, _, repo := setup(t)

	p1 := &domainb2b.OrganizationDomain{ID: "p1", Status: domainb2b.StatusPending}
	p2 := &domainb2b.OrganizationDomain{
		ID: "p2", Status: domainb2b.StatusPending, NextRetryAt: time.Now().Add(time.Hour),
	}
	v1 := &domainb2b.OrganizationDomain{ID: "v1", Status: domainb2b.StatusVerified}
	f1 := &domainb2b.OrganizationDomain{ID: "f1", Status: domainb2b.StatusFailed}
	repo.listPending = append(repo.listPending, p1, p2)
	repo.items[p1.ID] = p1
	repo.items[p2.ID] = p2
	repo.items[v1.ID] = v1
	repo.items[f1.ID] = f1

	claimed, err := svc.ClaimPending(context.Background(), 50, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, "p1", claimed[0].ID)
	assert.Equal(t, 50, repo.claimLimit)
	assert.Equal(t, 5*time.Minute, repo.claimLease)
	assert.True(t, claimed[0].NextRetryAt.After(time.Now()))
}

func TestClaimPending_RejectsInvalidBudget(t *testing.T) {
	svc, _, _ := setup(t)

	_, err := svc.ClaimPending(context.Background(), 0, time.Minute)
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "limit", appErr.Meta["field"])
	assert.Equal(t, 0, appErr.Meta["value"])
	assert.Equal(t, 1, appErr.Meta["min"])

	_, err = svc.ClaimPending(context.Background(), 10, 0)
	require.Error(t, err)
	appErr = apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "lease", appErr.Meta["field"])
	assert.Equal(t, "0s", appErr.Meta["value"])
	assert.Equal(t, "1ns", appErr.Meta["min"])
}

func TestClaimPending_PropagatesRepositoryError(t *testing.T) {
	svc, _, repo := setup(t)
	repo.claimErr = stderrors.New("db down")
	if _, err := svc.ClaimPending(context.Background(), 10, time.Minute); err == nil {
		t.Fatal("expected error")
	}
}

func TestDomainMutations_EnforceOrganizationOwnership(t *testing.T) {
	for _, operation := range []string{"delete", "verify"} {
		t.Run(operation, func(t *testing.T) {
			svc, _, repo := setup(t)
			claim, err := svc.ClaimDomain(context.Background(), ClaimDomainCommand{
				OrganizationID: "org-acme", Domain: "corp.example.com",
			})
			require.NoError(t, err)

			if operation == "delete" {
				err = svc.DeleteDomain(context.Background(), claim.ID, "org-other")
			} else {
				_, _, err = svc.VerifyDomainByID(context.Background(), claim.ID, "org-other")
			}
			require.Error(t, err)
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.CodeNotFound, appErr.Code, "cross-tenant must be indistinguishable from missing")
			assert.Equal(t, domainb2b.StatusPending, repo.items[claim.ID].Status)
		})
	}
}

func TestDomainWritesEmbedAuditAndOutbox(t *testing.T) {
	svc, dns, repo, uow := setupAudit(t)
	ctx := context.Background()

	claim, err := svc.ClaimDomain(ctx, ClaimDomainCommand{
		OrganizationID: "org-acme", Domain: "corp.example.com",
	})
	require.NoError(t, err)
	require.Len(t, uow.records.created, 1)
	require.Len(t, uow.outbox.events, 1)
	assert.Equal(t, "b2b.domain.claimed", uow.records.created[0].Action)
	assert.NotContains(t, string(uow.outbox.events[0].Payload), claim.VerificationToken)

	dns.with("_guliangiam-verification.corp.example.com", claim.VerificationToken)
	uow.records.created = nil
	uow.outbox.events = nil
	out, verified, err := svc.VerifyDomainByID(ctx, claim.ID, "org-acme")
	require.NoError(t, err)
	assert.True(t, verified)
	assert.Equal(t, domainb2b.StatusVerified, out.Status)
	require.Len(t, uow.records.created, 1)
	require.Len(t, uow.outbox.events, 1)
	assert.Equal(t, "b2b.domain.verified", uow.records.created[0].Action)
	assert.NotContains(t, string(uow.outbox.events[0].Payload), claim.VerificationToken)

	uow.records.created = nil
	uow.outbox.events = nil
	require.NoError(t, svc.DeleteDomain(ctx, claim.ID, "org-acme"))
	require.Len(t, uow.records.created, 1)
	require.Len(t, uow.outbox.events, 1)
	assert.Equal(t, "b2b.domain.deleted", uow.records.created[0].Action)
	assert.NotContains(t, string(uow.outbox.events[0].Payload), claim.VerificationToken)
	assert.Nil(t, repo.items[claim.ID])
	assert.Equal(t, 3, uow.transactions)
}

func TestVerifyFailureWithinBudgetPersistsWithoutAudit(t *testing.T) {
	svc, dns, repo, uow := setupAudit(t)
	ctx := context.Background()

	claim, err := svc.ClaimDomain(ctx, ClaimDomainCommand{
		OrganizationID: "org-acme", Domain: "retry.example.com",
	})
	require.NoError(t, err)
	uow.records.created = nil
	uow.outbox.events = nil

	dns.with("_guliangiam-verification.retry.example.com", "wrong-token")
	out, err := svc.VerifySingle(ctx, repo.items[claim.ID], DefaultVerificationPolicy())
	require.NoError(t, err)
	assert.Equal(t, domainb2b.StatusPending, out.Status)
	assert.Equal(t, 1, out.Attempts)
	require.Empty(t, uow.records.created)
	require.Empty(t, uow.outbox.events)
}

// ── 构造函数注入测试 ────────────────────────────────────────────────────────

func TestNewService_UsesDefaultDNSResolver(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := uowmock.NewMockUnitOfWork(ctrl)
	svc := NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uow)
	assert.NotNil(t, svc.DNSResolver)
}

// ── 端到端集成测试（§六 交互简化 + 数据流闭环）────────────────────────────
//
// 完整业务闭环：
//   ClaimDomain (pending) → DNS TXT 配置 → VerifySingle (verified) → FindForSSO (路由)
//
// 这是 §六 "数据流闭环无歧义" 的直接证据。

func TestE2E_Claim_Verify_SSO(t *testing.T) {
	svc, dns, repo := setup(t)
	ctx := context.Background()

	// ── Step 1: 组织 A 认领 corp.example.com ──────────────────────────────
	claim, err := svc.ClaimDomain(ctx, ClaimDomainCommand{
		OrganizationID: "org-acme",
		Domain:         "corp.example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, domainb2b.StatusPending, claim.Status)
	assert.NotEmpty(t, claim.VerificationToken, "认领后必须生成 token")

	// fakeRepo 内部状态：pending 且在 listPending 队列中
	require.Len(t, repo.listPending, 1, "认领后应进入 pending 队列")

	// ── Step 2: 管理员配置 DNS TXT record ─────────────────────────────────
	dns.with("_guliangiam-verification.corp.example.com", claim.VerificationToken)

	// ── Step 3: Cron Worker 原子认领 → 自动验证 ───────────────────────────
	pendingList, err := svc.ClaimPending(ctx, 50, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, pendingList, 1)

	// VerifyDomainByID 走完整 app 层链路：GetByID → DNS lookup → Update
	out, verified, err := svc.VerifyDomainByID(ctx, pendingList[0].ID, "org-acme")
	require.NoError(t, err)
	assert.True(t, verified, "token 匹配时 verified 标志必须为 true")
	assert.Equal(t, domainb2b.StatusVerified, out.Status)
	assert.False(t, out.VerifiedAt.IsZero(), "verified_at 必须被设置")

	// fakeRepo 状态同步：pending 队列清空 + domainIndex 建立 SS0 索引
	assert.Empty(t, repo.listPending, "验证成功后 pending 队列应清空")
	assert.NotNil(t, repo.domainIndex["corp.example.com"],
		"验证成功后应建立跨租户 SSO domainIndex")

	// ── Step 4: SSO 路由 —— 根据邮箱域发现归属组织 ────────────────────────
	ssoRoute, err := svc.FindForSSO(ctx, "alice@corp.example.com")
	require.NoError(t, err)
	require.NotNil(t, ssoRoute, "verified domain 必须能路由")
	assert.Equal(t, "org-acme", ssoRoute.OrganizationID)
	assert.Equal(t, "corp.example.com", ssoRoute.Domain)

	// ── Step 5: 同一组织重复认领同一域名 → 拒绝（SQL UNIQUE (org_id, domain)）─
	_, err = svc.ClaimDomain(ctx, ClaimDomainCommand{
		OrganizationID: "org-acme",
		Domain:         "corp.example.com",
	})
	require.Error(t, err, "同一组织不得重复认领同一域名（uq_org_domain 约束）")
}

func TestE2E_Claim_DNSFail_Flow(t *testing.T) {
	svc, dns, repo := setup(t)
	ctx := context.Background()

	// Step 1: Claim
	claim, err := svc.ClaimDomain(ctx, ClaimDomainCommand{
		OrganizationID: "org-acme",
		Domain:         "mail.example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, domainb2b.StatusPending, claim.Status)

	// Step 2: DNS TXT 配错（admin 粘错了 token）
	dns.with("_guliangiam-verification.mail.example.com", "GuliangIAM-Verification=v1:WRONG")

	// Step 3: 认领一次 → 预算内保持 pending；第 5 次才进入 failed 终态
	claimed, err := svc.ClaimPending(ctx, 50, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	var out DomainDTO
	var verified bool
	for attempt := 1; attempt <= DefaultMaxVerificationAttempts; attempt++ {
		out, verified, err = svc.VerifyDomainByID(ctx, claimed[0].ID, "org-acme")
		require.NoError(t, err)
		assert.False(t, verified, "token 不匹配时 verified 必须为 false")
		if attempt < DefaultMaxVerificationAttempts {
			assert.Equal(t, domainb2b.StatusPending, out.Status,
				"预算未耗尽时必须保持 pending 等待续验")
			assert.Equal(t, attempt, out.Attempts)
		}
	}
	assert.Equal(t, domainb2b.StatusFailed, out.Status,
		"重试预算耗尽后才进入 failed 终态")
	assert.Equal(t, DefaultMaxVerificationAttempts, out.Attempts)
	assert.NotEmpty(t, out.LastError)

	// SSO 找不到（verified 状态才索引）
	sso, _ := svc.FindForSSO(ctx, "bob@mail.example.com")
	assert.Nil(t, sso, "failed 域名不得用于 SSO 路由")
	_ = repo // 保留
}

func TestE2E_Claim_ManualVerify_ViaVerifyDomainByID(t *testing.T) {
	// 模拟 transport 层 VerifyDomain 端点（手动触发）
	svc, dns, _ := setup(t)
	ctx := context.Background()

	claim, err := svc.ClaimDomain(ctx, ClaimDomainCommand{
		OrganizationID: "org-acme",
		Domain:         "api.example.com",
	})
	require.NoError(t, err)

	dns.with("_guliangiam-verification.api.example.com", claim.VerificationToken)

	out, verified, err := svc.VerifyDomainByID(ctx, claim.ID, "org-acme")
	require.NoError(t, err)
	assert.True(t, verified, "token 匹配时 verified 标志必须为 true")
	assert.Equal(t, domainb2b.StatusVerified, out.Status)

	sso, _ := svc.FindForSSO(ctx, "carol@api.example.com")
	require.NotNil(t, sso)
	assert.Equal(t, "org-acme", sso.OrganizationID)
}

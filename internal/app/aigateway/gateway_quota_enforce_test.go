package aigateway

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// ── BE-C-03: scope/scopeRef/status 匹配 ────────────────────────────────

func strPtr(s string) *string { return &s }

func TestPolicyApplies(t *testing.T) {
	key := &domain.LlmApiKey{ID: "k-1", OrganizationID: "org-1"}
	cases := []struct {
		name  string
		p     *domain.LlmQuotaPolicy
		alias string
		want  bool
	}{
		{"nil", nil, "gpt-4", false},
		{"disabled", &domain.LlmQuotaPolicy{OrganizationID: "org-1", Scope: "organization", Status: domain.QuotaStatusDisabled}, "gpt-4", false},
		{"foreign org", &domain.LlmQuotaPolicy{OrganizationID: "org-2", Scope: "organization", Status: domain.QuotaStatusActive}, "gpt-4", false},
		{"org scope", &domain.LlmQuotaPolicy{OrganizationID: "org-1", Scope: "organization", Status: domain.QuotaStatusActive}, "gpt-4", true},
		{"api_key ref match", &domain.LlmQuotaPolicy{OrganizationID: "org-1", Scope: "api_key", ScopeRef: strPtr("k-1"), Status: domain.QuotaStatusActive}, "gpt-4", true},
		{"api_key ref mismatch", &domain.LlmQuotaPolicy{OrganizationID: "org-1", Scope: "api_key", ScopeRef: strPtr("k-other"), Status: domain.QuotaStatusActive}, "gpt-4", false},
		{"api_key empty ref = org-wide", &domain.LlmQuotaPolicy{OrganizationID: "org-1", Scope: "api_key", Status: domain.QuotaStatusActive}, "gpt-4", true},
		{"alias ref match", &domain.LlmQuotaPolicy{OrganizationID: "org-1", Scope: "alias", ScopeRef: strPtr("gpt-4"), Status: domain.QuotaStatusActive}, "gpt-4", true},
		{"alias ref mismatch", &domain.LlmQuotaPolicy{OrganizationID: "org-1", Scope: "alias", ScopeRef: strPtr("claude"), Status: domain.QuotaStatusActive}, "gpt-4", false},
		{"unknown scope", &domain.LlmQuotaPolicy{OrganizationID: "org-1", Scope: "galaxy", Status: domain.QuotaStatusActive}, "gpt-4", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := policyApplies(tc.p, key, tc.alias); got != tc.want {
				t.Fatalf("policyApplies = %v, want %v", got, tc.want)
			}
		})
	}
}

// fakeQuotaListRepo 返回预置策略列表（List 路径）。
type fakeQuotaListRepo struct {
	stubQuotaRepo
	policies []*domain.LlmQuotaPolicy
	listErr  error
}

func (f *fakeQuotaListRepo) List(_ context.Context, _ string) ([]*domain.LlmQuotaPolicy, error) {
	return f.policies, f.listErr
}

// fakeActiveQuotaListRepo 同时实现 ListActive（数据面优先路径）。
type fakeActiveQuotaListRepo struct {
	fakeQuotaListRepo
	active []*domain.LlmQuotaPolicy
}

func (f *fakeActiveQuotaListRepo) ListActive(_ context.Context, _ string) ([]*domain.LlmQuotaPolicy, error) {
	return f.active, nil
}

func quotaTestService(meter domain.QuotaMeter, quotas domain.QuotaPoliciesRepo) *GatewayService {
	return &GatewayService{
		Clock:  mockClock{t: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)},
		Meter:  meter,
		Quotas: quotas,
	}
}

func quotaTestKey() *domain.LlmApiKey {
	return &domain.LlmApiKey{ID: "k-1", OrganizationID: "org-1", Status: domain.APIKeyStatusActive}
}

func TestEnforceQuota_DisabledRPMPolicyNotEnforced(t *testing.T) {
	meter := newMockQuotaMeter()
	p := &domain.LlmQuotaPolicy{
		ID: "q-1", OrganizationID: "org-1", Scope: "organization",
		LimitType: domain.LimitTypeRPM, LimitValue: 1, Status: domain.QuotaStatusDisabled,
	}
	svc := quotaTestService(meter, &fakeQuotaListRepo{policies: []*domain.LlmQuotaPolicy{p}})
	// 预置计数器已达上限：disabled 策略不得 TryIncr 也不得拒绝
	scopeKey := buildQuotaKey(p, svc.Clock.Now())
	meter.counters[scopeKey] = 5

	if err := svc.enforceQuota(context.Background(), quotaTestKey(), "gpt-4", makeValidReq()); err != nil {
		t.Fatalf("disabled policy must not enforce, got %v", err)
	}
	if meter.counters[scopeKey] != 5 {
		t.Fatalf("disabled policy must not consume counter, got %d", meter.counters[scopeKey])
	}
}

func TestEnforceQuota_ScopeRefMismatchSkipped(t *testing.T) {
	meter := newMockQuotaMeter()
	p := &domain.LlmQuotaPolicy{
		ID: "q-1", OrganizationID: "org-1", Scope: "api_key", ScopeRef: strPtr("k-other"),
		LimitType: domain.LimitTypeRPM, LimitValue: 1, Status: domain.QuotaStatusActive,
	}
	svc := quotaTestService(meter, &fakeQuotaListRepo{policies: []*domain.LlmQuotaPolicy{p}})
	meter.counters[buildQuotaKey(p, svc.Clock.Now())] = 5

	if err := svc.enforceQuota(context.Background(), quotaTestKey(), "gpt-4", makeValidReq()); err != nil {
		t.Fatalf("policy scoped to another key must not enforce, got %v", err)
	}
}

func TestEnforceQuota_MatchingPolicyEnforced(t *testing.T) {
	meter := newMockQuotaMeter()
	p := &domain.LlmQuotaPolicy{
		ID: "q-1", OrganizationID: "org-1", Scope: "api_key", ScopeRef: strPtr("k-1"),
		LimitType: domain.LimitTypeRPM, LimitValue: 1, Status: domain.QuotaStatusActive,
	}
	svc := quotaTestService(meter, &fakeQuotaListRepo{policies: []*domain.LlmQuotaPolicy{p}})
	meter.counters[buildQuotaKey(p, svc.Clock.Now())] = 1

	err := svc.enforceQuota(context.Background(), quotaTestKey(), "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("matching policy at limit must reject")
	}
	if e, ok := err.(*apperrors.Error); !ok || e.Code != apperrors.CodeTooManyRequests {
		t.Fatalf("expected 429, got %v", err)
	}
}

func TestEnforceQuota_ListActivePreferred(t *testing.T) {
	meter := newMockQuotaMeter()
	active := &domain.LlmQuotaPolicy{
		ID: "q-1", OrganizationID: "org-1", Scope: "organization",
		LimitType: domain.LimitTypeRPM, LimitValue: 1, Status: domain.QuotaStatusActive,
	}
	repo := &fakeActiveQuotaListRepo{
		fakeQuotaListRepo: fakeQuotaListRepo{listErr: apperrors.NewInternal("List must not be used")},
		active:            []*domain.LlmQuotaPolicy{active},
	}
	svc := quotaTestService(meter, repo)
	meter.counters[buildQuotaKey(active, svc.Clock.Now())] = 1

	err := svc.enforceQuota(context.Background(), quotaTestKey(), "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("ListActive result must be enforced even when List would fail")
	}
}

// ── BE-C-04: MonthlyBudget 请求前比对 ──────────────────────────────────

func TestEnforceQuota_MonthlyBudgetExceeded(t *testing.T) {
	meter := newMockQuotaMeter()
	svc := quotaTestService(meter, nil)
	key := quotaTestKey()
	budget := 1.0 // 1 USD = 100 cents
	key.MonthlyBudget = &budget
	meter.counters["cost:api_key:k-1:2026-10"] = 100

	err := svc.enforceQuota(context.Background(), key, "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("budget exhausted must reject before upstream call")
	}
	if e, ok := err.(*apperrors.Error); !ok || e.Code != apperrors.CodeTooManyRequests {
		t.Fatalf("expected 429, got %v", err)
	}
}

func TestEnforceQuota_MonthlyBudgetUnderLimit(t *testing.T) {
	meter := newMockQuotaMeter()
	svc := quotaTestService(meter, nil)
	key := quotaTestKey()
	budget := 1.0
	key.MonthlyBudget = &budget
	meter.counters["cost:api_key:k-1:2026-10"] = 99

	if err := svc.enforceQuota(context.Background(), key, "gpt-4", makeValidReq()); err != nil {
		t.Fatalf("under-budget request must pass, got %v", err)
	}
}

// ── BE-C-05: 数据面租户上下文 ──────────────────────────────────────────

type scopeCapturingUsageLogs struct {
	mockUsageLogsRepo
	scope tenantctx.TenantScope
}

func (m *scopeCapturingUsageLogs) Create(ctx context.Context, u *domain.UsageLog) error {
	m.scope = tenantctx.TenantScopeFromContext(ctx)
	return m.mockUsageLogsRepo.Create(ctx, u)
}

type scopeCapturingApiKeys struct {
	stubRepo
	scope tenantctx.TenantScope
}

func (f *scopeCapturingApiKeys) GetByHash(ctx context.Context, hash string) (*domain.LlmApiKey, error) {
	f.scope = tenantctx.TenantScopeFromContext(ctx)
	return f.stubRepo.GetByHash(ctx, hash)
}

func TestGatewayService_Chat_BindsKeyTenant(t *testing.T) {
	svc, _, _, _ := makeService(t)
	usage := &scopeCapturingUsageLogs{}
	keys := &scopeCapturingApiKeys{}
	svc.UsageLogs = usage
	svc.ApiKeys = keys

	if _, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", makeValidReq()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// key 查询：认证前无可信租户 → System 作用域
	if keys.scope.Mode != tenantctx.TenantFilterSystem {
		t.Fatalf("GetByHash must run under system scope, got %q", keys.scope.Mode)
	}
	// 业务落库：必须绑定 key.OrganizationID（Strict）
	if usage.scope.Mode != tenantctx.TenantFilterStrict || usage.scope.OrganizationID != "org-1" {
		t.Fatalf("usage write must run under strict key tenant, got mode=%q org=%q", usage.scope.Mode, usage.scope.OrganizationID)
	}
}

func TestGatewayService_Embedding_BindsKeyTenant(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	upstream.embedResp = &domain.CanonicalResponse{InputTokens: 1, TotalTokens: 1}
	usage := &scopeCapturingUsageLogs{}
	svc.UsageLogs = usage

	if _, err := svc.Embedding(context.Background(), "hash-xyz", "emb", makeValidReq()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if usage.scope.Mode != tenantctx.TenantFilterStrict || usage.scope.OrganizationID != "org-1" {
		t.Fatalf("embedding usage write must run under strict key tenant, got mode=%q org=%q", usage.scope.Mode, usage.scope.OrganizationID)
	}
}

type orglessKeyRepo struct{ stubRepo }

func (f *orglessKeyRepo) GetByHash(_ context.Context, hash string) (*domain.LlmApiKey, error) {
	return &domain.LlmApiKey{ID: "k-x", KeyHash: hash, Status: domain.APIKeyStatusActive}, nil
}

func TestGatewayService_Authenticate_RejectsKeyWithoutOrg(t *testing.T) {
	svc, _, _, _ := makeService(t)
	svc.ApiKeys = &orglessKeyRepo{}

	_, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("key without organization must be rejected (fail-closed)")
	}
	if e, ok := err.(*apperrors.Error); !ok || e.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

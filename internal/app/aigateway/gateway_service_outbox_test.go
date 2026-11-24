package aigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
)

// fixedNow is used for deterministic CreatedAt/UpdatedAt timestamps in fixtures
var fixedNow = time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

// ── Mock QuotaMeter (in-memory) ──────────────────────────────────────

type mockQuotaMeter struct {
	counters map[string]int64
	incrs    map[string]int64 // last delta used for Incr
}

func newMockQuotaMeter() *mockQuotaMeter {
	return &mockQuotaMeter{counters: map[string]int64{}, incrs: map[string]int64{}}
}

func (m *mockQuotaMeter) Incr(_ context.Context, key string, delta int64, _ time.Duration) (int64, error) {
	m.counters[key] += delta
	m.incrs[key] = delta
	return m.counters[key], nil
}
func (m *mockQuotaMeter) Get(_ context.Context, key string) (int64, error) {
	return m.counters[key], nil
}
func (m *mockQuotaMeter) TryIncr(_ context.Context, key string, delta, limit int64, _ time.Duration) (allowed bool, current int64, err error) {
	cur := m.counters[key]
	if cur+delta > limit {
		return false, cur, nil
	}
	m.counters[key] = cur + delta
	m.incrs[key] = delta
	return true, cur + delta, nil
}

// ── Test helpers (scans mockOutboxRepo.events from health_probe_worker_test.go) ──

func findOutboxEvent(repo *mockOutboxRepo, aggType, eventType string) *domainoutbox.OutboxEvent {
	for _, e := range repo.events {
		if e.AggregateType == aggType && e.EventType == eventType {
			return e
		}
	}
	return nil
}

// ── Fixture: service with quota + outbox + API key with RPM limit ───

type fakeApiKeysWithLimits struct {
	rpm *int
	tpm *int
}

func (f *fakeApiKeysWithLimits) GetByID(_ context.Context, id string) (*domain.LlmApiKey, error) {
	return nil, nil
}
func (f *fakeApiKeysWithLimits) GetByHash(_ context.Context, hash string) (*domain.LlmApiKey, error) {
	return &domain.LlmApiKey{
		ID: "k-1", OrganizationID: "org-1", KeyHash: hash,
		Status: domain.APIKeyStatusActive, Currency: "USD",
		RPMLimit: f.rpm, TPMLimit: f.tpm,
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}, nil
}
func (f *fakeApiKeysWithLimits) List(_ context.Context, org string) ([]*domain.LlmApiKey, error) {
	return nil, nil
}
func (f *fakeApiKeysWithLimits) Create(_ context.Context, k *domain.LlmApiKey) error { return nil }
func (f *fakeApiKeysWithLimits) Update(_ context.Context, k *domain.LlmApiKey) error { return nil }
func (f *fakeApiKeysWithLimits) Delete(_ context.Context, id string) error           { return nil }

// alwaysXxxMeter always returns hardcoded values — used to force quota exceeded
type alwaysXxxMeter struct {
	returnForGet  int64
	returnForIncr int64
}

func (m *alwaysXxxMeter) Incr(_ context.Context, _ string, _ int64, _ time.Duration) (int64, error) {
	return m.returnForIncr, nil
}
func (m *alwaysXxxMeter) Get(_ context.Context, _ string) (int64, error) {
	return m.returnForGet, nil
}
func (m *alwaysXxxMeter) TryIncr(_ context.Context, _ string, delta, limit int64, _ time.Duration) (allowed bool, current int64, err error) {
	// 模拟 returnForGet 是当前值：若 cur+delta > limit 则拒绝
	if m.returnForGet+delta > limit {
		return false, m.returnForGet, nil
	}
	return true, m.returnForGet + delta, nil
}

// ── Test cases ───────────────────────────────────────────────────────

func TestGatewayService_Chat_SuccessEmitsUsageRecorded(t *testing.T) {
	rpm := 1000 // 高限额，不触发配额超限
	tpm := 50000
	meter := newMockQuotaMeter()
	outbox := &mockOutboxRepo{}

	svc := buildServiceWithDeps(t, meter, outbox, &fakeApiKeysWithLimits{rpm: &rpm, tpm: &tpm})

	_, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", makeValidReq())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// 找到 ai.usage.recorded 事件
	evt := findOutboxEvent(outbox, "llm_usage", "ai.usage.recorded")
	if evt == nil {
		t.Fatalf("expected ai.usage.recorded outbox event, got %d events", len(outbox.events))
	}

	assertOutboxEventShape(t, evt, "llm_usage", "ai.usage.recorded", "org-1")

	var payload map[string]any
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("invalid payload JSON: %v", err)
	}
	for _, k := range []string{"organization_id", "provider_id", "model_id", "input_tokens", "output_tokens", "total_tokens", "cost_amount"} {
		if _, ok := payload[k]; !ok {
			t.Errorf("usage payload missing key %q: %v", k, payload)
		}
	}
	if payload["provider_id"] != "p-1" {
		t.Errorf("expected provider_id=p-1, got %v", payload["provider_id"])
	}
	if payload["model_id"] != "m-1" {
		t.Errorf("expected model_id=m-1, got %v", payload["model_id"])
	}

	t.Logf("Usage recorded payload: %s", string(evt.Payload))
}

func TestGatewayService_Chat_RPMExceeded_EmitsQuotaExceeded(t *testing.T) {
	// RPM limit = 5, meter always returns 6 → 触发 RPM 超限
	rpm := 5
	tpm := 50000
	meter := &alwaysXxxMeter{returnForGet: 6, returnForIncr: 6}
	outbox := &mockOutboxRepo{}

	svc := buildServiceWithDeps(t, meter, outbox, &fakeApiKeysWithLimits{rpm: &rpm, tpm: &tpm})

	_, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected TooManyRequests error")
	}

	evt := findOutboxEvent(outbox, "llm_quota", "ai.quota.exceeded")
	if evt == nil {
		t.Fatalf("expected ai.quota.exceeded event. got %d events", len(outbox.events))
	}

	assertOutboxEventShape(t, evt, "llm_quota", "ai.quota.exceeded", "org-1")

	var payload map[string]any
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("invalid payload JSON: %v", err)
	}

	for _, k := range []string{"organization_id", "api_key_id", "quota_scope", "quota_limit_type", "current_value", "limit_value"} {
		if _, ok := payload[k]; !ok {
			t.Errorf("quota payload missing key %q: %v", k, payload)
		}
	}
	if payload["quota_limit_type"] != "rpm" {
		t.Errorf("expected quota_limit_type=rpm, got %v", payload["quota_limit_type"])
	}
	if payload["quota_scope"] != "api_key" {
		t.Errorf("expected quota_scope=api_key, got %v", payload["quota_scope"])
	}
	if payload["api_key_id"] != "k-1" {
		t.Errorf("expected api_key_id=k-1, got %v", payload["api_key_id"])
	}

	t.Logf("Quota exceeded payload: %s", string(evt.Payload))
}

func TestGatewayService_Chat_TPMExceeded_EmitsQuotaExceeded(t *testing.T) {
	// RPM 放行；TPM 路径通过 cur=50 + estimate=0 >= limit=5 触发
	rpm := 1000
	tpm := 5
	meter := &alwaysXxxMeter{returnForGet: 50, returnForIncr: 50}
	outbox := &mockOutboxRepo{}

	svc := buildServiceWithDeps(t, meter, outbox, &fakeApiKeysWithLimits{rpm: &rpm, tpm: &tpm})

	_, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected TooManyRequests error (TPM exceeded)")
	}

	evt := findOutboxEvent(outbox, "llm_quota", "ai.quota.exceeded")
	if evt == nil {
		t.Fatalf("expected ai.quota.exceeded event, got %d events", len(outbox.events))
	}

	var payload map[string]any
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		t.Fatalf("invalid payload JSON: %v", err)
	}
	if payload["quota_limit_type"] != "tpm" {
		t.Errorf("expected quota_limit_type=tpm, got %v", payload["quota_limit_type"])
	}

	t.Logf("TPM quota exceeded payload: %s", string(evt.Payload))
}

func TestGatewayService_OutboxNil_NoPanic(t *testing.T) {
	rpm := 1000
	tpm := 50000
	meter := newMockQuotaMeter()

	svc := buildServiceWithDeps(t, meter, nil, &fakeApiKeysWithLimits{rpm: &rpm, tpm: &tpm})

	_, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", makeValidReq())
	if err != nil {
		t.Fatalf("unexpected err with outbox=nil: %v", err)
	}
	// 不崩溃 = 通过
}

// ── helpers ───────────────────────────────────────────────────────────

func assertOutboxEventShape(t *testing.T, evt *domainoutbox.OutboxEvent, aggType, eventType, orgID string) {
	t.Helper()
	if evt.AggregateType != aggType {
		t.Errorf("AggregateType: want %q, got %q", aggType, evt.AggregateType)
	}
	if evt.EventType != eventType {
		t.Errorf("EventType: want %q, got %q", eventType, evt.EventType)
	}
	if evt.OrganizationID == nil || *evt.OrganizationID != orgID {
		t.Errorf("OrganizationID: want %q, got %v", orgID, evt.OrganizationID)
	}
	if evt.EventVersion != 1 {
		t.Errorf("EventVersion: want 1, got %d", evt.EventVersion)
	}
	if evt.MaxAttempts != 3 {
		t.Errorf("MaxAttempts: want 3, got %d", evt.MaxAttempts)
	}
	if len(evt.Payload) == 0 {
		t.Error("Payload should be non-empty")
	}
}

// buildServiceWithDeps wires up the same dependencies as makeService in
// gateway_service_test.go but accepts custom meter/outbox/apiKeys so we
// can inject quota and audit behavior for outbox integration tests.
func buildServiceWithDeps(
	t *testing.T,
	meter domain.QuotaMeter,
	outbox domainoutbox.Repository,
	apiKeys domain.ApiKeysRepo,
) *GatewayService {
	t.Helper()

	router := &mockRouter{
		res: &domain.RouteResolution{
			Provider: &domain.LlmProvider{
				ID: "p-1", OrganizationID: "org-1", Name: "openai",
			},
			Model:        &domain.LlmModel{ID: "m-1", ProviderModelName: "gpt-4o"},
			Route:        &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1},
			ProtocolFace: "openai",
			Health:       &domain.ProviderHealth{}, // empty state = healthy
		},
	}

	upstream := &mockUpstream{
		name: "openai_compat", proto: "openai",
		chatResp: &domain.CanonicalResponse{
			Content:     []domain.CanonicalContentBlock{{Type: "text", Text: "hello"}},
			InputTokens: 10, OutputTokens: 5, TotalTokens: 15,
		},
	}
	registry := newMockRegistry()
	registry.Register("openai", upstream)

	prices := &mockPriceRepo{
		pv: &domain.LlmPriceVersion{
			ID: "pv-1", OrganizationID: "org-1", ModelID: "m-1",
			InputPricePer1M: 5.0, OutputPricePer1M: 15.0, Currency: "USD",
		},
	}

	usageLogs := &mockUsageLogsRepo{}

	return NewGatewayService(
		mockClock{t: fixedNow},
		mockUUID{s: "uuid-fixed", e: nil},
		router, registry,
		&stubQuotaRepo{}, prices, usageLogs,
		apiKeys, nil, &stubProviderRepo{}, &stubHealthRepo{},
		meter, outbox,
	)
}

// silence "imported and not used" warning
var _ = fmt.Sprintf

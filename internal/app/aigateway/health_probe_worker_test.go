package aigateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
)

// ============================================================================
// Mocks — in-file, minimal, sufficient for HealthProbeWorker tests
// ============================================================================

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }

type mockHealthRepo struct {
	listOpen  []*domain.ProviderHealth
	listErr   error
	upserts   []*domain.ProviderHealth
	upsertErr error
}

func (m *mockHealthRepo) ListOpen(ctx context.Context) ([]*domain.ProviderHealth, error) {
	return m.listOpen, m.listErr
}
func (m *mockHealthRepo) Upsert(ctx context.Context, h *domain.ProviderHealth) error {
	m.upserts = append(m.upserts, h)
	return m.upsertErr
}
func (m *mockHealthRepo) GetByID(ctx context.Context, id string) (*domain.ProviderHealth, error) {
	return nil, nil
}
func (m *mockHealthRepo) GetByIDForOrganization(ctx context.Context, orgID, id string) (*domain.ProviderHealth, error) {
	return nil, nil
}
func (m *mockHealthRepo) GetByProvider(ctx context.Context, pid string) (*domain.ProviderHealth, error) {
	return nil, nil
}
func (m *mockHealthRepo) ListAll(ctx context.Context) ([]*domain.ProviderHealth, error) {
	return nil, nil
}
func (m *mockHealthRepo) ListByOrganization(ctx context.Context, orgID string) ([]*domain.ProviderHealth, error) {
	return nil, nil
}

type mockProvidersRepo struct {
	providers map[string]*domain.LlmProvider
}

func newMockProvidersRepo() *mockProvidersRepo {
	return &mockProvidersRepo{providers: map[string]*domain.LlmProvider{}}
}
func (m *mockProvidersRepo) GetByID(ctx context.Context, id string) (*domain.LlmProvider, error) {
	return m.providers[id], nil
}
func (m *mockProvidersRepo) GetByName(ctx context.Context, oid, name string) (*domain.LlmProvider, error) {
	return nil, nil
}
func (m *mockProvidersRepo) List(ctx context.Context, oid string) ([]*domain.LlmProvider, error) {
	return nil, nil
}
func (m *mockProvidersRepo) Create(ctx context.Context, p *domain.LlmProvider) error { return nil }
func (m *mockProvidersRepo) Update(ctx context.Context, p *domain.LlmProvider) error { return nil }
func (m *mockProvidersRepo) Delete(ctx context.Context, id string) error             { return nil }

type mockOutboxRepo struct {
	events []*domainoutbox.OutboxEvent
	err    error
}

func (m *mockOutboxRepo) Enqueue(ctx context.Context, e *domainoutbox.OutboxEvent) error {
	m.events = append(m.events, e)
	return m.err
}
func (m *mockOutboxRepo) EnqueueBatch(ctx context.Context, items []*domainoutbox.OutboxEvent) error {
	m.events = append(m.events, items...)
	return m.err
}

// ============================================================================
// Helpers
// ============================================================================

func makeOpenHealth(pid string, openUntil time.Time) *domain.ProviderHealth {
	return &domain.ProviderHealth{
		ID:                  "h-" + pid,
		ProviderID:          pid,
		State:               domain.HealthStateOpen,
		ConsecutiveFailures: 5,
		OpenUntil:           &openUntil,
	}
}

// ============================================================================
// Tests
// ============================================================================

func TestHealthProbeWorker_Tick_EmptyList(t *testing.T) {
	healthRepo := &mockHealthRepo{listOpen: nil} // nil → 无 open health
	providers := newMockProvidersRepo()
	w := NewHealthProbeWorker(healthRepo, providers, fakeClock{now: time.Now()},
		http.DefaultClient, nil, 30*time.Second, nil)

	err := w.tick(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(healthRepo.upserts) != 0 {
		t.Errorf("expected 0 upserts, got %d", len(healthRepo.upserts))
	}
}

func TestHealthProbeWorker_Tick_SuccessRecovery(t *testing.T) {
	// HTTP server returns 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	now := time.Now()
	expired := now.Add(-1 * time.Second) // already expired
	pid := "provider-1"
	orgID := "org-1"

	healthRepo := &mockHealthRepo{
		listOpen: []*domain.ProviderHealth{makeOpenHealth(pid, expired)},
	}
	providers := newMockProvidersRepo()
	providers.providers[pid] = &domain.LlmProvider{
		ID:             pid,
		OrganizationID: orgID,
		Name:           "Test Provider",
		BaseURL:        srv.URL,
	}
	outbox := &mockOutboxRepo{}

	w := NewHealthProbeWorker(healthRepo, providers, fakeClock{now: now},
		srv.Client(), outbox, 30*time.Second, nil)

	err := w.tick(context.Background())
	if err != nil {
		t.Fatalf("tick err: %v", err)
	}

	// 期望: 2 次 upsert — 1) half_open 进入 2) RecordSuccess → healthy
	if len(healthRepo.upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(healthRepo.upserts))
	}
	final := healthRepo.upserts[len(healthRepo.upserts)-1]
	if final.State != domain.HealthStateHealthy {
		t.Errorf("final state = %q, want healthy", final.State)
	}
	if final.OpenUntil != nil {
		t.Error("OpenUntil should be nil after healthy")
	}
	if final.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0", final.ConsecutiveFailures)
	}

	// Outbox: ai.provider.circuit_closed
	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outbox.events))
	}
	ev := outbox.events[0]
	if ev.EventType != "ai.provider.circuit_closed" {
		t.Errorf("event_type = %q", ev.EventType)
	}
	if ev.AggregateType != "llm_provider" {
		t.Errorf("aggregate_type = %q", ev.AggregateType)
	}
	if ev.OrganizationID == nil || *ev.OrganizationID != orgID {
		t.Errorf("org_id = %v", ev.OrganizationID)
	}
	// payload 验证
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload["trigger"] != "probe_success" {
		t.Errorf("trigger = %v", payload["trigger"])
	}
	if payload["provider_id"] != pid {
		t.Errorf("provider_id = %v", payload["provider_id"])
	}
}

func TestHealthProbeWorker_Tick_FailureReopen(t *testing.T) {
	// HTTP server returns 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	now := time.Now()
	expired := now.Add(-1 * time.Second)
	pid := "provider-2"
	orgID := "org-1"

	healthRepo := &mockHealthRepo{
		listOpen: []*domain.ProviderHealth{makeOpenHealth(pid, expired)},
	}
	providers := newMockProvidersRepo()
	providers.providers[pid] = &domain.LlmProvider{
		ID:             pid,
		OrganizationID: orgID,
		BaseURL:        srv.URL,
	}
	outbox := &mockOutboxRepo{}

	w := NewHealthProbeWorker(healthRepo, providers, fakeClock{now: now},
		srv.Client(), outbox, 30*time.Second, nil)

	err := w.tick(context.Background())
	if err != nil {
		t.Fatalf("tick err: %v", err)
	}

	// 期望: 2 次 upsert — 1) half_open 2) RecordFailure → open (新 OpenUntil)
	if len(healthRepo.upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(healthRepo.upserts))
	}
	final := healthRepo.upserts[len(healthRepo.upserts)-1]
	if final.State != domain.HealthStateOpen {
		t.Errorf("final state = %q, want open", final.State)
	}
	if final.OpenUntil == nil {
		t.Error("OpenUntil should be set after reopen")
	}

	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outbox.events))
	}
	if outbox.events[0].EventType != "ai.provider.circuit_opened" {
		t.Errorf("event_type = %q", outbox.events[0].EventType)
	}
	var payload map[string]any
	_ = json.Unmarshal(outbox.events[0].Payload, &payload)
	if payload["trigger"] != "probe_failure" {
		t.Errorf("trigger = %v", payload["trigger"])
	}
}

func TestHealthProbeWorker_Tick_NotYetExpired(t *testing.T) {
	now := time.Now()
	notExpired := now.Add(30 * time.Second) // 还没到期
	pid := "provider-3"

	healthRepo := &mockHealthRepo{
		listOpen: []*domain.ProviderHealth{makeOpenHealth(pid, notExpired)},
	}
	w := NewHealthProbeWorker(healthRepo, newMockProvidersRepo(), fakeClock{now: now},
		http.DefaultClient, nil, 30*time.Second, nil)

	err := w.tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(healthRepo.upserts) != 0 {
		t.Errorf("expected 0 upserts (not yet expired), got %d", len(healthRepo.upserts))
	}
}

func TestHealthProbeWorker_Tick_OutboxNilSafe(t *testing.T) {
	// outbox = nil 时，成功/失败都不应该 panic
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	now := time.Now()
	expired := now.Add(-1 * time.Second)
	pid := "provider-4"

	healthRepo := &mockHealthRepo{
		listOpen: []*domain.ProviderHealth{makeOpenHealth(pid, expired)},
	}
	providers := newMockProvidersRepo()
	providers.providers[pid] = &domain.LlmProvider{ID: pid, BaseURL: srv.URL}

	w := NewHealthProbeWorker(healthRepo, providers, fakeClock{now: now},
		srv.Client(), nil, 30*time.Second, nil)

	err := w.tick(context.Background())
	if err != nil {
		t.Fatalf("tick err: %v", err)
	}
	// 验证成功 — 没有 panic，upsert 发生了
	if len(healthRepo.upserts) != 2 {
		t.Errorf("expected 2 upserts, got %d", len(healthRepo.upserts))
	}
}

func TestHealthProbeWorker_Tick_ProviderNotFound(t *testing.T) {
	// ProviderRepo.GetByID 返回 nil — 直接跳过（没有 valid provider + BaseURL
	// 就不做 EnterHalfOpen 状态转换）
	now := time.Now()
	expired := now.Add(-1 * time.Second)
	pid := "provider-5"

	healthRepo := &mockHealthRepo{
		listOpen: []*domain.ProviderHealth{makeOpenHealth(pid, expired)},
	}
	providers := newMockProvidersRepo() // 空 map → GetByID 返回 nil

	w := NewHealthProbeWorker(healthRepo, providers, fakeClock{now: now},
		http.DefaultClient, nil, 30*time.Second, nil)

	err := w.tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// provider 缺失 → 在 EnterHalfOpen 之前就 continue → 0 upserts
	if len(healthRepo.upserts) != 0 {
		t.Errorf("expected 0 upserts (skipped before EnterHalfOpen), got %d", len(healthRepo.upserts))
	}
}

func TestHealthProbeWorker_Tick_ProviderNoBaseURL(t *testing.T) {
	// provider 存在但 BaseURL 为空 — 同样在 EnterHalfOpen 之前跳过
	now := time.Now()
	expired := now.Add(-1 * time.Second)
	pid := "provider-6"

	healthRepo := &mockHealthRepo{
		listOpen: []*domain.ProviderHealth{makeOpenHealth(pid, expired)},
	}
	providers := newMockProvidersRepo()
	providers.providers[pid] = &domain.LlmProvider{ID: pid, BaseURL: ""} // 空 URL

	w := NewHealthProbeWorker(healthRepo, providers, fakeClock{now: now},
		http.DefaultClient, nil, 30*time.Second, nil)

	err := w.tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(healthRepo.upserts) != 0 {
		t.Errorf("expected 0 upserts (BaseURL empty → skipped before EnterHalfOpen), got %d", len(healthRepo.upserts))
	}
}

func TestHealthProbeWorker_Tick_MultipleProviders(t *testing.T) {
	// 两个 provider 都 open 且过期 — 各自独立处理
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	now := time.Now()
	expired := now.Add(-1 * time.Second)
	pid1 := "p-multi-1"
	pid2 := "p-multi-2"

	h1 := makeOpenHealth(pid1, expired)
	h2 := makeOpenHealth(pid2, expired)
	healthRepo := &mockHealthRepo{listOpen: []*domain.ProviderHealth{h1, h2}}

	providers := newMockProvidersRepo()
	providers.providers[pid1] = &domain.LlmProvider{ID: pid1, BaseURL: srv.URL}
	providers.providers[pid2] = &domain.LlmProvider{ID: pid2, BaseURL: srv.URL}

	outbox := &mockOutboxRepo{}
	w := NewHealthProbeWorker(healthRepo, providers, fakeClock{now: now},
		srv.Client(), outbox, 30*time.Second, nil)

	err := w.tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 每个 provider 2 upserts (half_open + healthy) → 4
	if len(healthRepo.upserts) != 4 {
		t.Fatalf("expected 4 upserts, got %d", len(healthRepo.upserts))
	}
	// 每个 provider 1 outbox event → 2
	if len(outbox.events) != 2 {
		t.Fatalf("expected 2 outbox events, got %d", len(outbox.events))
	}
}

func TestHealthProbeWorker_StartStop_Idempotent(t *testing.T) {
	healthRepo := &mockHealthRepo{}
	providers := newMockProvidersRepo()
	w := NewHealthProbeWorker(healthRepo, providers, fakeClock{now: time.Now()},
		http.DefaultClient, nil, 30*time.Second, nil)

	// Start twice — should not panic
	w.Start(context.Background())
	w.Start(context.Background())

	// Stop twice — should not panic
	w.Stop()
	w.Stop()

	// Start → Stop → Start 也应该可以
	w.Start(context.Background())
	w.Stop()
}

// ============================================================================
// enqueueCircuitEvent 单元级测试（通过 tick 间接覆盖，但直接调一下验证 nil-safe）
// ============================================================================

func TestHealthProbeWorker_EnqueueCircuitEvent_NilOutbox(t *testing.T) {
	w := NewHealthProbeWorker(&mockHealthRepo{}, newMockProvidersRepo(), fakeClock{now: time.Now()},
		http.DefaultClient, nil, 30*time.Second, nil)

	// nil outbox — 不 panic
	w.enqueueCircuitEvent(context.Background(), "org", "provider", "ai.provider.circuit_opened", 100, "test")
}

func TestHealthProbeWorker_EnqueueCircuitEvent_WithOutbox(t *testing.T) {
	outbox := &mockOutboxRepo{}
	w := NewHealthProbeWorker(&mockHealthRepo{}, newMockProvidersRepo(), fakeClock{now: time.Now()},
		http.DefaultClient, outbox, 30*time.Second, nil)

	w.enqueueCircuitEvent(context.Background(), "org-abc", "provider-xyz",
		"ai.provider.circuit_opened", 250, "manual_test")

	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(outbox.events))
	}
	ev := outbox.events[0]
	if ev.EventType != "ai.provider.circuit_opened" {
		t.Errorf("EventType = %q", ev.EventType)
	}
	if ev.AggregateType != "llm_provider" {
		t.Errorf("AggregateType = %q", ev.AggregateType)
	}
	if ev.AggregateID != "provider-xyz" {
		t.Errorf("AggregateID = %q", ev.AggregateID)
	}
	if ev.OrganizationID == nil || *ev.OrganizationID != "org-abc" {
		t.Errorf("OrgID = %v", ev.OrganizationID)
	}
	var payload map[string]any
	_ = json.Unmarshal(ev.Payload, &payload)
	if payload["latency_ms"].(float64) != 250 {
		t.Errorf("latency_ms = %v", payload["latency_ms"])
	}
	if payload["trigger"] != "manual_test" {
		t.Errorf("trigger = %v", payload["trigger"])
	}
}

// ============================================================================
// probe 单元测试
// ============================================================================

func TestHealthProbeWorker_Probe_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w := NewHealthProbeWorker(&mockHealthRepo{}, newMockProvidersRepo(), fakeClock{now: time.Now()},
		srv.Client(), nil, 30*time.Second, nil)

	latency, ok := w.probe(context.Background(), srv.URL)
	if !ok {
		t.Errorf("probe should succeed (200), ok=false")
	}
	if latency < 0 {
		t.Errorf("latency = %d, should be >= 0", latency)
	}
}

func TestHealthProbeWorker_Probe_400_StillOk(t *testing.T) {
	// 4xx 也算成功 — 只有 5xx 才算失败
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	w := NewHealthProbeWorker(&mockHealthRepo{}, newMockProvidersRepo(), fakeClock{now: time.Now()},
		srv.Client(), nil, 30*time.Second, nil)

	_, ok := w.probe(context.Background(), srv.URL)
	if !ok {
		t.Error("probe should succeed (401 < 500), ok=false")
	}
}

func TestHealthProbeWorker_Probe_500_Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	w := NewHealthProbeWorker(&mockHealthRepo{}, newMockProvidersRepo(), fakeClock{now: time.Now()},
		srv.Client(), nil, 30*time.Second, nil)

	_, ok := w.probe(context.Background(), srv.URL)
	if ok {
		t.Error("probe should fail on 500, got ok=true")
	}
}

func TestHealthProbeWorker_Probe_InvalidURL_Fails(t *testing.T) {
	w := NewHealthProbeWorker(&mockHealthRepo{}, newMockProvidersRepo(), fakeClock{now: time.Now()},
		http.DefaultClient, nil, 30*time.Second, nil)

	_, ok := w.probe(context.Background(), "://bad-url")
	if ok {
		t.Error("probe should fail on invalid URL")
	}
}

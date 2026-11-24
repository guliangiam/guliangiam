package aigateway

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ─── mocks ──────────────────────────────────────────────────────

type mockClock struct {
	t time.Time
}

func (m mockClock) Now() time.Time { return m.t }

type mockUUID struct {
	s string
	e error
}

func (m mockUUID) NewString() (string, error) { return m.s, m.e }

type mockRouter struct {
	res *domain.RouteResolution
	err error
}

func (m *mockRouter) Resolve(ctx context.Context, orgID, alias string, req *domain.CanonicalRequest) (*domain.RouteResolution, error) {
	return m.res, m.err
}

type mockUpstream struct {
	name      string
	proto     string
	chatResp  *domain.CanonicalResponse
	chatErr   error
	embedResp *domain.CanonicalResponse
	embedErr  error
	streamCh  chan *domain.CanonicalStreamEvent
	streamErr error
}

func (m *mockUpstream) Name() string              { return m.name }
func (m *mockUpstream) SupportedProtocol() string { return m.proto }
func (m *mockUpstream) Chat(ctx context.Context, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	return m.chatResp, m.chatErr
}
func (m *mockUpstream) ChatStream(ctx context.Context, req *domain.CanonicalRequest) (<-chan *domain.CanonicalStreamEvent, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	if m.streamCh != nil {
		return m.streamCh, nil
	}
	// 默认一个空 channel，立即关闭
	ch := make(chan *domain.CanonicalStreamEvent)
	close(ch)
	return ch, nil
}
func (m *mockUpstream) Embedding(ctx context.Context, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	return m.embedResp, m.embedErr
}

type mockRegistry struct {
	m map[string]domain.UpstreamAdapter
}

func newMockRegistry() *mockRegistry                                { return &mockRegistry{m: map[string]domain.UpstreamAdapter{}} }
func (r *mockRegistry) Register(p string, a domain.UpstreamAdapter) { r.m[p] = a }
func (r *mockRegistry) Get(p string) (domain.UpstreamAdapter, bool) { a, ok := r.m[p]; return a, ok }
func (r *mockRegistry) List() []string {
	out := make([]string, 0, len(r.m))
	for k := range r.m {
		out = append(out, k)
	}
	return out
}

// BuildAdapter 测试 stub —— 忽略 provider 参数（测试不需要真实 baseURL/apiKey），
// 直接按 protocolFace 从预注册的 adapter 里返回。没注册就报错。
func (r *mockRegistry) BuildAdapter(provider *domain.LlmProvider, protocolFace string) (domain.UpstreamAdapter, error) {
	if a, ok := r.m[protocolFace]; ok {
		return a, nil
	}
	return nil, domain.ErrUnsupportedProtocol
}

type mockUsageLogsRepo struct {
	last  *domain.UsageLog
	calls int
	err   error
}

func (m *mockUsageLogsRepo) Create(ctx context.Context, u *domain.UsageLog) error {
	m.last = u
	m.calls++
	return m.err
}
func (m *mockUsageLogsRepo) List(ctx context.Context, org string, limit, offset int) ([]*domain.UsageLog, int64, error) {
	return nil, 0, nil
}
func (m *mockUsageLogsRepo) QueryStats(ctx context.Context, org string, days int) (*domain.UsageStats, error) {
	return &domain.UsageStats{}, nil
}

type mockPriceRepo struct {
	pv  *domain.LlmPriceVersion
	err error
}

func (m *mockPriceRepo) GetByModel(ctx context.Context, id string) (*domain.LlmPriceVersion, error) {
	return m.pv, m.err
}
func (m *mockPriceRepo) GetByID(ctx context.Context, id string) (*domain.LlmPriceVersion, error) {
	return nil, nil
}
func (m *mockPriceRepo) List(ctx context.Context, orgID string) ([]*domain.LlmPriceVersion, error) {
	return nil, nil
}
func (m *mockPriceRepo) ListByModel(ctx context.Context, id string) ([]*domain.LlmPriceVersion, error) {
	return nil, nil
}
func (m *mockPriceRepo) Create(ctx context.Context, v *domain.LlmPriceVersion) error { return nil }
func (m *mockPriceRepo) Update(ctx context.Context, v *domain.LlmPriceVersion) error { return nil }
func (m *mockPriceRepo) Delete(ctx context.Context, id string) error                 { return nil }

// 其他 Repo 空实现，GatewayService 目前不直接用
type stubRepo struct{}

func (s *stubRepo) GetByID(ctx context.Context, id string) (*domain.LlmApiKey, error) {
	return nil, nil
}
func (s *stubRepo) GetByHash(ctx context.Context, hash string) (*domain.LlmApiKey, error) {
	// 返回 active key，让 authenticate 通过
	return &domain.LlmApiKey{
		ID: "k-1", OrganizationID: "org-1", Name: "test",
		KeyPrefix: "sk-xxx", KeyHash: hash, Status: domain.APIKeyStatusActive,
		Currency: "USD", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil
}
func (s *stubRepo) List(ctx context.Context, org string) ([]*domain.LlmApiKey, error) {
	return nil, nil
}
func (s *stubRepo) Create(ctx context.Context, k *domain.LlmApiKey) error { return nil }
func (s *stubRepo) Update(ctx context.Context, k *domain.LlmApiKey) error { return nil }
func (s *stubRepo) Delete(ctx context.Context, id string) error           { return nil }

type stubProviderRepo struct{}

func (s *stubProviderRepo) GetByID(ctx context.Context, id string) (*domain.LlmProvider, error) {
	return nil, nil
}
func (s *stubProviderRepo) List(ctx context.Context, org string) ([]*domain.LlmProvider, error) {
	return nil, nil
}
func (s *stubProviderRepo) Create(ctx context.Context, p *domain.LlmProvider) error { return nil }
func (s *stubProviderRepo) Update(ctx context.Context, p *domain.LlmProvider) error { return nil }
func (s *stubProviderRepo) Delete(ctx context.Context, id string) error             { return nil }
func (s *stubProviderRepo) GetByName(ctx context.Context, org, name string) (*domain.LlmProvider, error) {
	return nil, nil
}

type stubHealthRepo struct{}

func (s *stubHealthRepo) GetByID(ctx context.Context, id string) (*domain.ProviderHealth, error) {
	return nil, nil
}
func (s *stubHealthRepo) GetByIDForOrganization(ctx context.Context, orgID, id string) (*domain.ProviderHealth, error) {
	return nil, nil
}
func (s *stubHealthRepo) GetByProvider(ctx context.Context, id string) (*domain.ProviderHealth, error) {
	return nil, nil
}
func (s *stubHealthRepo) Upsert(ctx context.Context, h *domain.ProviderHealth) error { return nil }
func (s *stubHealthRepo) ListOpen(ctx context.Context) ([]*domain.ProviderHealth, error) {
	return nil, nil
}
func (s *stubHealthRepo) ListAll(ctx context.Context) ([]*domain.ProviderHealth, error) {
	return nil, nil
}
func (s *stubHealthRepo) ListByOrganization(ctx context.Context, orgID string) ([]*domain.ProviderHealth, error) {
	return nil, nil
}

type stubQuotaRepo struct{}

func (s *stubQuotaRepo) GetByID(ctx context.Context, id string) (*domain.LlmQuotaPolicy, error) {
	return nil, nil
}
func (s *stubQuotaRepo) List(ctx context.Context, org string) ([]*domain.LlmQuotaPolicy, error) {
	return nil, nil
}
func (s *stubQuotaRepo) Create(ctx context.Context, q *domain.LlmQuotaPolicy) error { return nil }
func (s *stubQuotaRepo) Update(ctx context.Context, q *domain.LlmQuotaPolicy) error { return nil }
func (s *stubQuotaRepo) Delete(ctx context.Context, id string) error                { return nil }

// ─── fixtures ────────────────────────────────────────────────────

func makeValidReq() *domain.CanonicalRequest {
	return &domain.CanonicalRequest{
		ModelAlias: "gpt-4",
		Messages: []domain.CanonicalMessage{
			{Role: "user", Content: []domain.CanonicalContentBlock{{Type: "text", Text: "hi"}}},
		},
	}
}

func makeService(t *testing.T) (*GatewayService, *mockRouter, *mockUpstream, *mockUsageLogsRepo) {
	t.Helper()

	upstream := &mockUpstream{
		name:  "openai_compat",
		proto: "openai",
		chatResp: &domain.CanonicalResponse{
			Content:      []domain.CanonicalContentBlock{{Type: "text", Text: "hello"}},
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}
	registry := newMockRegistry()
	registry.Register("openai", upstream)

	provider := &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: "openai"}
	model := &domain.LlmModel{ID: "m-1", ProviderModelName: "gpt-4o"}
	route := &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1}
	resolution := &domain.RouteResolution{
		Provider:     provider,
		Model:        model,
		Route:        route,
		ProtocolFace: "openai",
	}
	router := &mockRouter{res: resolution}

	usageLogs := &mockUsageLogsRepo{}
	prices := &mockPriceRepo{
		pv: &domain.LlmPriceVersion{
			ID:               "pv-1",
			OrganizationID:   "org-1",
			ModelID:          "m-1",
			InputPricePer1M:  5.0,
			OutputPricePer1M: 15.0,
			Currency:         "USD",
		},
	}

	svc := NewGatewayService(
		mockClock{t: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)},
		mockUUID{s: "uuid-fixed", e: nil},
		router, registry,
		&stubQuotaRepo{}, prices, usageLogs,
		&stubRepo{}, nil, &stubProviderRepo{}, &stubHealthRepo{},
		nil, // meter: disabled (Fail-Open)
		nil, // outbox: disabled (Fail-Open)
	)
	return svc, router, upstream, usageLogs
}

// ─── tests ───────────────────────────────────────────────────────

func TestGatewayService_Chat_Success(t *testing.T) {
	svc, _, _, usageLogs := makeService(t)

	resp, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", makeValidReq())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Content[0].Text != "hello" {
		t.Errorf("expected hello, got %q", resp.Content[0].Text)
	}
	// UsageLog 应被写
	if usageLogs.calls != 1 {
		t.Fatalf("expected 1 usage log call, got %d", usageLogs.calls)
	}
	log := usageLogs.last
	if log.Status != domain.UsageStatusSucceeded {
		t.Errorf("expected status %q, got %q", domain.UsageStatusSucceeded, log.Status)
	}
	if log.OrganizationID != "org-1" {
		t.Errorf("expected org-1, got %s", log.OrganizationID)
	}
	if log.ProviderID != "p-1" || log.ModelID != "m-1" {
		t.Errorf("wrong provider/model IDs: %s/%s", log.ProviderID, log.ModelID)
	}
	// 成本计算：10 input * 5/1M + 5 output * 15/1M = 50/1M + 75/1M = 125/1M = 0.000125
	expectedCost := float64(10)/1_000_000*5.0 + float64(5)/1_000_000*15.0
	if log.CostAmount != expectedCost {
		t.Errorf("expected cost %.8f, got %.8f", expectedCost, log.CostAmount)
	}
}

func TestGatewayService_Chat_AuthEmptyHash(t *testing.T) {
	svc, _, _, _ := makeService(t)
	_, err := svc.Chat(context.Background(), "", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected error")
	}
	if e, ok := err.(*apperrors.Error); !ok || e.Code != apperrors.CodeUnauthorized {
		t.Errorf("expected unauthorized, got %v (type %T)", err, err)
	}
}

func TestGatewayService_Chat_ValidateRequestFails(t *testing.T) {
	svc, _, _, _ := makeService(t)
	bad := &domain.CanonicalRequest{
		ModelAlias: "", // 为空触发 Validate 失败
	}
	_, err := svc.Chat(context.Background(), "h", "gpt-4", bad)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGatewayService_Chat_NoRouterConfigured(t *testing.T) {
	// 不注入 Routers（保持 nil），验证无路由配置时返回错误
	upstream := &mockUpstream{proto: "openai"}
	registry := newMockRegistry()
	registry.Register("openai", upstream)
	svc := NewGatewayService(mockClock{t: time.Now()}, mockUUID{s: "u"}, nil, registry,
		&stubQuotaRepo{}, &mockPriceRepo{}, &mockUsageLogsRepo{},
		&stubRepo{}, nil, &stubProviderRepo{}, &stubHealthRepo{},
		nil, // meter disabled
		nil) // outbox disabled

	_, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected error when no router")
	}
}

func TestGatewayService_Chat_RouterResolveFails(t *testing.T) {
	svc, router, _, _ := makeService(t)
	router.err = fmt.Errorf("alias not found")

	_, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGatewayService_Chat_NoUpstreamAdapter(t *testing.T) {
	// resolution 指向 openai，但 registry 里没注册
	provider := &domain.LlmProvider{ID: "p-2", OrganizationID: "org-1"}
	model := &domain.LlmModel{ID: "m-2"}
	route := &domain.LlmRoute{ID: "r-2", AliasID: "a-2"}
	resolution := &domain.RouteResolution{Provider: provider, Model: model, Route: route, ProtocolFace: "unknown_proto"}

	svc := NewGatewayService(mockClock{t: time.Now()}, mockUUID{s: "u"},
		&mockRouter{res: resolution}, newMockRegistry(),
		&stubQuotaRepo{}, &mockPriceRepo{}, &mockUsageLogsRepo{},
		&stubRepo{}, nil, &stubProviderRepo{}, &stubHealthRepo{},
		nil, // meter disabled
		nil) // outbox disabled

	_, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGatewayService_Chat_CircuitOpen(t *testing.T) {
	svc, router, _, usageLogs := makeService(t)
	router.res.Health = &domain.ProviderHealth{
		ID:         "ph-1",
		ProviderID: "p-1",
		State:      domain.HealthStateOpen,
	}

	_, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected error when circuit open")
	}
	// 即使熔断，UsageLog 也写失败行
	if usageLogs.calls != 1 {
		t.Errorf("expected usage log written even when circuit open, got %d", usageLogs.calls)
	}
	if usageLogs.last == nil || usageLogs.last.Status != domain.UsageStatusFailed {
		t.Errorf("expected failed status usage log, got %+v", usageLogs.last)
	}
}

func TestGatewayService_Chat_UpstreamFails(t *testing.T) {
	svc, _, upstream, usageLogs := makeService(t)
	upstream.chatErr = fmt.Errorf("provider timeout")

	_, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected error")
	}
	// 失败也写 UsageLog
	if usageLogs.calls != 1 || usageLogs.last.Status != domain.UsageStatusFailed {
		t.Errorf("expected failed usage log, got calls=%d status=%q", usageLogs.calls, usageLogs.last.Status)
	}
}

func TestGatewayService_Chat_NilResponseFromUpstream(t *testing.T) {
	svc, _, upstream, usageLogs := makeService(t)
	upstream.chatResp = nil
	upstream.chatErr = nil

	_, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected error for nil response")
	}
	if usageLogs.last == nil || usageLogs.last.Status != domain.UsageStatusFailed {
		t.Errorf("expected failed usage log, got %+v", usageLogs.last)
	}
}

func TestGatewayService_ChatStream_Success(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	ch := make(chan *domain.CanonicalStreamEvent, 2)
	ch <- &domain.CanonicalStreamEvent{Type: "message_start"}
	ch <- &domain.CanonicalStreamEvent{Type: "message_stop"}
	close(ch)
	upstream.streamCh = ch

	evCh, err := svc.ChatStream(context.Background(), "h", "gpt-4", makeValidReq())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	count := 0
	for range evCh {
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 events, got %d", count)
	}
}

func TestGatewayService_ChatStream_UpstreamFails(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	upstream.streamErr = fmt.Errorf("stream dial failed")

	_, err := svc.ChatStream(context.Background(), "h", "gpt-4", makeValidReq())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGatewayService_Embedding_Success(t *testing.T) {
	svc, _, upstream, usageLogs := makeService(t)
	upstream.embedResp = &domain.CanonicalResponse{
		InputTokens:  8,
		OutputTokens: 0,
		TotalTokens:  8,
	}

	resp, err := svc.Embedding(context.Background(), "h", "emb-v1", makeValidReq())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.InputTokens != 8 {
		t.Errorf("expected 8 input tokens, got %d", resp.InputTokens)
	}
	if usageLogs.calls != 1 {
		t.Errorf("expected 1 usage log for embedding, got %d", usageLogs.calls)
	}
}

func TestGatewayService_UsageLogCreateFails_DoesNotBlockResponse(t *testing.T) {
	svc, _, _, usageLogs := makeService(t)
	usageLogs.err = fmt.Errorf("db down")

	resp, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	if err != nil {
		t.Fatalf("usage log create failure should not block: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response despite usage log failure")
	}
	if usageLogs.calls != 1 {
		t.Errorf("expected create attempted even with error, got calls=%d", usageLogs.calls)
	}
}

func TestGatewayService_UsageLogsRepoNil_SkipsRecording(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	// 替换 UsageLogs 为 nil
	svc.UsageLogs = nil
	_ = upstream

	resp, err := svc.Chat(context.Background(), "h", "gpt-4", makeValidReq())
	if err != nil {
		t.Fatalf("unexpected err with nil UsageLogs: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response even with nil UsageLogs repo")
	}
}

// 确保 clock/uuid 接口可用
var _ clock.Clock = mockClock{}
var _ uuid.Generator = mockUUID{}

// ─── provider health row identity / persistence errors ──────────

// 固定返回值便于断言：health 行必须直接使用 UUID 生成器的输出，而不是拼接前缀。
const testHealthUUID = "018f0f0f-0000-7000-8000-000000000001"

func TestGatewayService_Chat_NewHealthRowUsesUUID(t *testing.T) {
	svc, _, _, _ := makeService(t)
	health := &mockHealthRepo{}
	svc.Health = health
	svc.UUID = mockUUID{s: testHealthUUID}

	resp, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", makeValidReq())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if len(health.upserts) != 1 {
		t.Fatalf("expected 1 health upsert, got %d", len(health.upserts))
	}

	h := health.upserts[0]
	if !uuid.IsValid(h.ID) {
		t.Fatalf("health row id %q is not a valid UUID", h.ID)
	}
	if h.ID != testHealthUUID {
		t.Errorf("health row id = %q, want generator output %q", h.ID, testHealthUUID)
	}
	if h.ProviderID != "p-1" {
		t.Errorf("health row provider_id = %q, want p-1", h.ProviderID)
	}
}

func TestGatewayService_UpdateHealthOnFailure_NewRowUsesUUID(t *testing.T) {
	svc, router, _, _ := makeService(t)
	health := &mockHealthRepo{}
	svc.Health = health
	svc.UUID = mockUUID{s: testHealthUUID}

	if err := svc.updateHealthOnFailure(context.Background(), router.res); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(health.upserts) != 1 {
		t.Fatalf("expected 1 health upsert, got %d", len(health.upserts))
	}
	if id := health.upserts[0].ID; !uuid.IsValid(id) {
		t.Fatalf("health row id %q is not a valid UUID", id)
	}
}

func TestGatewayService_UpdateHealthOnSuccess_PropagatesUpsertError(t *testing.T) {
	svc, router, _, _ := makeService(t)
	upsertErr := errors.New("db down")
	health := &mockHealthRepo{upsertErr: upsertErr}
	svc.Health = health
	svc.UUID = mockUUID{s: testHealthUUID}

	err := svc.updateHealthOnSuccess(context.Background(), router.res)
	if err == nil {
		t.Fatal("health upsert failure must not be reported as success")
	}
	e, ok := err.(*apperrors.Error)
	if !ok || e.Code != apperrors.CodeInternal {
		t.Fatalf("expected wrapped internal error, got %T: %v", err, err)
	}
	if !errors.Is(err, upsertErr) {
		t.Error("wrapped error must retain the underlying cause")
	}
}

func TestGatewayService_UpdateHealthOnSuccess_UUIDFailureSkipsWrite(t *testing.T) {
	svc, router, _, _ := makeService(t)
	health := &mockHealthRepo{}
	svc.Health = health
	svc.UUID = mockUUID{e: errors.New("uuid source unavailable")}

	err := svc.updateHealthOnSuccess(context.Background(), router.res)
	if err == nil {
		t.Fatal("uuid generation failure must surface")
	}
	if len(health.upserts) != 0 {
		t.Fatalf("no row must be written on id failure, got %d", len(health.upserts))
	}
}

// Health 是旁路状态：持久化失败要记录，但不能把上游已成功的调用变成 5xx。
func TestGatewayService_Chat_HealthUpsertFailureDoesNotFailRequest(t *testing.T) {
	svc, _, _, _ := makeService(t)
	svc.Health = &mockHealthRepo{upsertErr: errors.New("db down")}
	svc.UUID = mockUUID{s: testHealthUUID}

	resp, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", makeValidReq())
	if err != nil {
		t.Fatalf("health persistence failure must not fail the data-plane response: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

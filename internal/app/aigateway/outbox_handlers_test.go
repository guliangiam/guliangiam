package aigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	metering "github.com/guliangiam/guliangiam/internal/domain/metering"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ── MeteringEventRepo stub ──────────────────────────────────────────

type fakeMeteringEventRepo struct {
	last  *domain.MeteringEvent
	calls int
	err   error
}

func (f *fakeMeteringEventRepo) Save(ctx context.Context, e *domain.MeteringEvent) error {
	f.last = e
	f.calls++
	return f.err
}

// ── Billing metering.Repository stub ───────────────────────────────

type fakeBillingRepo struct {
	inserted []*metering.MeteringEvent
	calls    int
	err      error // all InsertEvent calls return this
}

func (f *fakeBillingRepo) InsertEvent(ctx context.Context, e *metering.MeteringEvent) (bool, error) {
	f.inserted = append(f.inserted, e)
	f.calls++
	return f.err == nil, f.err
}
func (f *fakeBillingRepo) IngestBatch(ctx context.Context, events []*metering.MeteringEvent) (inserted, duplicates int, err error) {
	f.inserted = append(f.inserted, events...)
	return len(events), 0, f.err
}
func (f *fakeBillingRepo) ListEvents(ctx context.Context, q metering.ListEventsQuery) ([]*metering.MeteringEvent, int64, error) {
	return nil, 0, nil
}
func (f *fakeBillingRepo) ListAggregates(ctx context.Context, q metering.ListAggregatesQuery) ([]*metering.MeteringAggregate, int64, error) {
	return nil, 0, nil
}
func (f *fakeBillingRepo) UpsertAggregate(ctx context.Context, agg *metering.MeteringAggregate) error {
	return nil
}
func (f *fakeBillingRepo) RefreshAggregate(ctx context.Context, cmd metering.RefreshAggregateCommand) (*metering.MeteringAggregate, error) {
	return nil, nil
}
func (f *fakeBillingRepo) ScanDistinctCombos(ctx context.Context, since time.Time) ([]metering.Combo, error) {
	return nil, nil
}

// ── NewOutboxHandlers ────────────────────────────────────────────────

func TestNewOutboxHandlers_ReturnsAllThreeAggregates(t *testing.T) {
	handlers := NewOutboxHandlers(nil, slog.Default(), nil, platformuuid.V7Generator{})

	for _, at := range []string{"llm_provider", "llm_quota", "llm_usage"} {
		h, ok := handlers[at]
		if !ok {
			t.Errorf("missing handler for aggregate_type %q", at)
			continue
		}
		if h == nil {
			t.Errorf("nil handler for aggregate_type %q", at)
		}
	}
	if len(handlers) != 3 {
		t.Errorf("expected 3 handlers, got %d", len(handlers))
	}
}

func TestNewOutboxHandlers_NilLogger_DefaultsOK(t *testing.T) {
	handlers := NewOutboxHandlers(nil, nil, nil, platformuuid.V7Generator{})
	if len(handlers) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(handlers))
	}
}

func TestNewOutboxHandlers_NilRepos_StillConstructs(t *testing.T) {
	handlers := NewOutboxHandlers(nil, slog.Default(), nil, platformuuid.V7Generator{})
	if len(handlers) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(handlers))
	}
}

// ── meteringHandler.Handle: audit sink (llm_metering_events) ──────────

func TestMeteringHandler_Handle_AuditSink_FullPayloadRoundTrip(t *testing.T) {
	repo := &fakeMeteringEventRepo{}
	handlers := NewOutboxHandlers(repo, slog.Default(), nil, platformuuid.V7Generator{})
	h := handlers["llm_usage"]

	orgID := "org-xyz"
	payload := map[string]any{
		"provider_id":  "p-1",
		"model_id":     "m-1",
		"input_tokens": 100,
	}
	payloadBytes, _ := json.Marshal(payload)

	item := &platformoutbox.OutboxItem{
		ID:             "outbox-uuid",
		OrganizationID: &orgID,
		AggregateType:  "llm_usage",
		AggregateID:    "k-1",
		EventType:      "ai.usage.recorded",
		Payload:        payloadBytes,
		Attempts:       1,
	}

	err := h.Handle(context.Background(), item)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if repo.calls != 1 {
		t.Fatalf("expected 1 audit Save call, got %d", repo.calls)
	}
	e := repo.last
	if e == nil {
		t.Fatal("repo.last nil")
	}
	if e.ID != "outbox-uuid" || e.OutboxID != "outbox-uuid" {
		t.Errorf("ID/OutboxID mismatch: %s / %s", e.ID, e.OutboxID)
	}
	if e.AggregateType != "llm_usage" {
		t.Errorf("AggregateType mismatch: %s", e.AggregateType)
	}
	if e.EventType != "ai.usage.recorded" {
		t.Errorf("EventType mismatch: %s", e.EventType)
	}
	if e.AggregateID == nil || *e.AggregateID != "k-1" {
		t.Errorf("AggregateID mismatch: %+v", e.AggregateID)
	}
	if e.OrganizationID == nil || *e.OrganizationID != "org-xyz" {
		t.Errorf("OrganizationID mismatch: %+v", e.OrganizationID)
	}
	if e.ProviderID == nil || *e.ProviderID != "p-1" {
		t.Errorf("ProviderID mismatch: %+v", e.ProviderID)
	}
	if e.ModelID == nil || *e.ModelID != "m-1" {
		t.Errorf("ModelID mismatch: %+v", e.ModelID)
	}
	if !bytes.Equal(e.PayloadJSON, payloadBytes) {
		t.Errorf("PayloadJSON mismatch")
	}
}

func TestMeteringHandler_Handle_EmptyAggregateID_SetsNil(t *testing.T) {
	repo := &fakeMeteringEventRepo{}
	handlers := NewOutboxHandlers(repo, slog.Default(), nil, platformuuid.V7Generator{})
	h := handlers["llm_provider"]

	payload := map[string]any{}
	payloadBytes, _ := json.Marshal(payload)

	item := &platformoutbox.OutboxItem{
		ID:            "outbox-uuid",
		AggregateType: "llm_provider",
		AggregateID:   "",
		EventType:     "ai.provider.circuit_opened",
		Payload:       payloadBytes,
	}

	_ = h.Handle(context.Background(), item)
	if repo.last.AggregateID != nil {
		t.Errorf("expected nil AggregateID for empty string, got %+v", repo.last.AggregateID)
	}
}

func TestMeteringHandler_Handle_AuditRepoNil_BillingNil_LogOnly(t *testing.T) {
	handlers := NewOutboxHandlers(nil, slog.Default(), nil, platformuuid.V7Generator{})
	h := handlers["llm_usage"]

	item := &platformoutbox.OutboxItem{
		ID:            "outbox-uuid",
		AggregateType: "llm_usage",
		EventType:     "ai.usage.recorded",
	}

	err := h.Handle(context.Background(), item)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestMeteringHandler_Handle_AuditSaveError_ReturnsError(t *testing.T) {
	wantErr := errors.New("audit db down")
	auditRepo := &fakeMeteringEventRepo{err: wantErr}
	handlers := NewOutboxHandlers(auditRepo, slog.Default(), nil, platformuuid.V7Generator{})
	h := handlers["llm_provider"]
	item := &platformoutbox.OutboxItem{
		ID:            "evt-audit-err",
		AggregateType: "llm_provider",
		EventType:     "ai.provider.circuit_opened",
	}

	err := h.Handle(context.Background(), item)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Handle() error = %v, want wrapped audit error", err)
	}
	if auditRepo.calls != 1 {
		t.Errorf("audit calls = %d, want 1", auditRepo.calls)
	}
}

// ── meteringHandler.Handle: billing sink (metering_events) ────────────

func TestMeteringHandler_BillingSink_UsageRecorded_TokensAndCost(t *testing.T) {
	// Both audit + billing repo present → both sinks fire for usage.recorded
	auditRepo := &fakeMeteringEventRepo{}
	billingRepo := &fakeBillingRepo{}
	handlers := NewOutboxHandlers(auditRepo, slog.Default(), billingRepo, platformuuid.V7Generator{})
	h := handlers["llm_usage"]

	orgID := "org-billing-1"
	payload := map[string]any{
		"organization_id": orgID,
		"provider_id":     "p-1",
		"model_id":        "m-1",
		"api_key_id":      "key-abc",
		"input_tokens":    150,
		"output_tokens":   50,
		"total_tokens":    200,
		"cost_amount":     0.015, // yuan → 15 milli-yuan
		"currency":        "CNY",
	}
	payloadBytes, _ := json.Marshal(payload)

	item := &platformoutbox.OutboxItem{
		ID:             "evt-billing-001",
		OrganizationID: &orgID,
		AggregateType:  "llm_usage",
		EventType:      "ai.usage.recorded",
		Payload:        payloadBytes,
	}

	err := h.Handle(context.Background(), item)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// ── Audit sink: 1 call ──
	if auditRepo.calls != 1 {
		t.Errorf("audit Save calls = %d, want 1", auditRepo.calls)
	}

	// ── Billing sink: 2 calls (tokens + cost) ──
	if billingRepo.calls != 2 {
		t.Fatalf("billing InsertEvent calls = %d, want 2", billingRepo.calls)
	}

	// First: tokens fact
	tokensEvt := billingRepo.inserted[0]
	if tokensEvt.EventType != "ai.usage.tokens" {
		t.Errorf("tokens event_type = %s, want ai.usage.tokens", tokensEvt.EventType)
	}
	if tokensEvt.Quantity != 200 {
		t.Errorf("tokens quantity = %d, want 200", tokensEvt.Quantity)
	}
	if tokensEvt.Unit != "tokens" {
		t.Errorf("tokens unit = %s, want tokens", tokensEvt.Unit)
	}
	if tokensEvt.Subject != "key-abc" {
		t.Errorf("tokens subject = %s, want key-abc", tokensEvt.Subject)
	}
	if tokensEvt.IdempotencyKey != "evt-billing-001:tokens" {
		t.Errorf("tokens idempotency = %s", tokensEvt.IdempotencyKey)
	}

	// Second: cost fact
	costEvt := billingRepo.inserted[1]
	if costEvt.EventType != "ai.usage.cost" {
		t.Errorf("cost event_type = %s, want ai.usage.cost", costEvt.EventType)
	}
	if costEvt.Quantity != 15 { // 0.015 * 1000 = 15 milli-yuan
		t.Errorf("cost quantity = %d, want 15", costEvt.Quantity)
	}
	if costEvt.Unit != "milliyuan" {
		t.Errorf("cost unit = %s, want milliyuan", costEvt.Unit)
	}
	if costEvt.Subject != "key-abc" {
		t.Errorf("cost subject = %s, want key-abc", costEvt.Subject)
	}
}

func TestMeteringHandler_BillingSink_NoApiKey_FallsBackToOrg(t *testing.T) {
	billingRepo := &fakeBillingRepo{}
	handlers := NewOutboxHandlers(nil, slog.Default(), billingRepo, platformuuid.V7Generator{})
	h := handlers["llm_usage"]

	orgID := "org-fallback"
	payload := map[string]any{
		"input_tokens":  50,
		"output_tokens": 25,
		// no api_key_id → subject should fallback to org_id
	}
	payloadBytes, _ := json.Marshal(payload)

	item := &platformoutbox.OutboxItem{
		ID:             "evt-fallback-002",
		OrganizationID: &orgID,
		AggregateType:  "llm_usage",
		EventType:      "ai.usage.recorded",
		Payload:        payloadBytes,
	}

	_ = h.Handle(context.Background(), item)

	// 1 billing call (tokens only; no cost_amount → no cost fact)
	if billingRepo.calls != 1 {
		t.Fatalf("billing calls = %d, want 1", billingRepo.calls)
	}
	evt := billingRepo.inserted[0]
	if evt.Subject != orgID {
		t.Errorf("subject = %s, want %s (org fallback)", evt.Subject, orgID)
	}
	if evt.Quantity != 75 {
		t.Errorf("tokens quantity = %d, want 75 (50+25)", evt.Quantity)
	}
}

func TestMeteringHandler_BillingSink_NonUsageEventType_SkipsBilling(t *testing.T) {
	// llm_provider events → audit sink writes, billing sink skips
	auditRepo := &fakeMeteringEventRepo{}
	billingRepo := &fakeBillingRepo{}
	handlers := NewOutboxHandlers(auditRepo, slog.Default(), billingRepo, platformuuid.V7Generator{})
	h := handlers["llm_provider"]

	item := &platformoutbox.OutboxItem{
		ID:            "evt-provider-003",
		AggregateType: "llm_provider",
		EventType:     "ai.provider.circuit_opened",
	}

	_ = h.Handle(context.Background(), item)

	if auditRepo.calls != 1 {
		t.Errorf("audit calls = %d, want 1", auditRepo.calls)
	}
	if billingRepo.calls != 0 {
		t.Errorf("billing calls = %d, want 0 (non-usage event)", billingRepo.calls)
	}
}

func TestMeteringHandler_BillingSink_NilBillingRepo_SkipsBilling(t *testing.T) {
	auditRepo := &fakeMeteringEventRepo{}
	handlers := NewOutboxHandlers(auditRepo, slog.Default(), nil, platformuuid.V7Generator{})
	h := handlers["llm_usage"]

	orgID := "org-no-billing"
	payload := map[string]any{"input_tokens": 100}
	payloadBytes, _ := json.Marshal(payload)
	item := &platformoutbox.OutboxItem{
		ID:             "evt-no-billing-004",
		OrganizationID: &orgID,
		AggregateType:  "llm_usage",
		EventType:      "ai.usage.recorded",
		Payload:        payloadBytes,
	}

	err := h.Handle(context.Background(), item)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if auditRepo.calls != 1 {
		t.Errorf("audit calls = %d, want 1", auditRepo.calls)
	}
}

func TestMeteringHandler_BillingSink_BillingInsertError_ReturnsError(t *testing.T) {
	auditRepo := &fakeMeteringEventRepo{}
	wantErr := errors.New("billing db down")
	billingRepo := &fakeBillingRepo{err: wantErr}
	handlers := NewOutboxHandlers(auditRepo, slog.Default(), billingRepo, platformuuid.V7Generator{})
	h := handlers["llm_usage"]

	orgID := "org-err"
	payload := map[string]any{
		"input_tokens": 100,
		"cost_amount":  1.0,
	}
	payloadBytes, _ := json.Marshal(payload)
	item := &platformoutbox.OutboxItem{
		ID:             "evt-billing-err",
		OrganizationID: &orgID,
		AggregateType:  "llm_usage",
		EventType:      "ai.usage.recorded",
		Payload:        payloadBytes,
	}

	err := h.Handle(context.Background(), item)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Handle() error = %v, want wrapped billing error", err)
	}
	if auditRepo.calls != 1 {
		t.Errorf("audit calls = %d, want 1 (audit should succeed even if billing fails)", auditRepo.calls)
	}
	if billingRepo.calls != 1 {
		t.Errorf("billing calls = %d, want 1 (failed tokens fact must stop delivery)", billingRepo.calls)
	}
}

// ── Edge cases ────────────────────────────────────────────────────────

func TestMeteringHandler_Handle_InvalidJSONPayload_NoPanic(t *testing.T) {
	auditRepo := &fakeMeteringEventRepo{}
	billingRepo := &fakeBillingRepo{}
	handlers := NewOutboxHandlers(auditRepo, slog.Default(), billingRepo, platformuuid.V7Generator{})
	h := handlers["llm_usage"]

	orgID := "org-invalid"
	item := &platformoutbox.OutboxItem{
		ID:             "evt-invalid-005",
		OrganizationID: &orgID,
		AggregateType:  "llm_usage",
		EventType:      "ai.usage.recorded",
		Payload:        []byte("not valid json {{{"),
	}

	err := h.Handle(context.Background(), item)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if auditRepo.last.ProviderID != nil {
		t.Errorf("expected nil ProviderID for invalid JSON, got %+v", auditRepo.last.ProviderID)
	}
	if string(auditRepo.last.PayloadJSON) != "not valid json {{{" {
		t.Errorf("PayloadJSON mismatch")
	}

	// billing: no tokens/cost extracted from invalid JSON → 0 calls
	if billingRepo.calls != 0 {
		t.Errorf("billing calls = %d, want 0 (invalid JSON → no tokens/cost extracted)", billingRepo.calls)
	}
}

func TestMeteringHandler_Handle_BothReposNil_LogOnly(t *testing.T) {
	handlers := NewOutboxHandlers(nil, slog.Default(), nil, platformuuid.V7Generator{})
	h := handlers["llm_usage"]
	orgID := "org-empty"
	item := &platformoutbox.OutboxItem{
		ID:             "evt-empty",
		OrganizationID: &orgID,
		AggregateType:  "llm_usage",
		EventType:      "ai.usage.recorded",
	}

	err := h.Handle(context.Background(), item)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestMeteringHandler_BillingSink_NoOrgID_SkipsBilling(t *testing.T) {
	billingRepo := &fakeBillingRepo{}
	handlers := NewOutboxHandlers(nil, slog.Default(), billingRepo, platformuuid.V7Generator{})
	h := handlers["llm_usage"]

	// OrganizationID is nil on outbox item → billing skipped
	payload := map[string]any{"input_tokens": 100}
	payloadBytes, _ := json.Marshal(payload)
	item := &platformoutbox.OutboxItem{
		ID:            "evt-no-org",
		AggregateType: "llm_usage",
		EventType:     "ai.usage.recorded",
		// OrganizationID 保持 nil（无租户）
		Payload: payloadBytes,
	}

	_ = h.Handle(context.Background(), item)
	if billingRepo.calls != 0 {
		t.Errorf("billing calls = %d, want 0 (no org id on outbox item)", billingRepo.calls)
	}
}

// time package referenced by firstTime helper test if needed
var _ = time.Now

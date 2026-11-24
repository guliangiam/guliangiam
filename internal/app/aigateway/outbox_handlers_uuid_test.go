package aigateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	metering "github.com/guliangiam/guliangiam/internal/domain/metering"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// dedupeBillingRepo 模拟 metering_events 的
// ON CONFLICT (organization_id, idempotency_key) DO NOTHING 语义：
// 同一 (organization_id, idempotency_key) 第二次插入不落新行，
// 返回 (false, nil) 与 Postgres 实际行为一致。
type dedupeBillingRepo struct {
	mu    sync.Mutex
	rows  map[string]*metering.MeteringEvent
	calls int
}

func newDedupeBillingRepo() *dedupeBillingRepo {
	return &dedupeBillingRepo{rows: make(map[string]*metering.MeteringEvent)}
}

func (r *dedupeBillingRepo) InsertEvent(_ context.Context, e *metering.MeteringEvent) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	key := e.OrganizationID + "\x00" + e.IdempotencyKey
	if _, ok := r.rows[key]; ok {
		return false, nil
	}
	r.rows[key] = e
	return true, nil
}

func (r *dedupeBillingRepo) IngestBatch(_ context.Context, events []*metering.MeteringEvent) (int, int, error) {
	inserted, duplicates := 0, 0
	for _, e := range events {
		ok, err := r.InsertEvent(context.Background(), e)
		if err != nil {
			return inserted, duplicates, err
		}
		if ok {
			inserted++
		} else {
			duplicates++
		}
	}
	return inserted, duplicates, nil
}

func (r *dedupeBillingRepo) ListEvents(context.Context, metering.ListEventsQuery) ([]*metering.MeteringEvent, int64, error) {
	return nil, 0, nil
}

func (r *dedupeBillingRepo) ScanDistinctCombos(context.Context, time.Time) ([]metering.Combo, error) {
	return nil, nil
}

func (r *dedupeBillingRepo) ListAggregates(context.Context, metering.ListAggregatesQuery) ([]*metering.MeteringAggregate, int64, error) {
	return nil, 0, nil
}

func (r *dedupeBillingRepo) UpsertAggregate(context.Context, *metering.MeteringAggregate) error {
	return nil
}

func (r *dedupeBillingRepo) RefreshAggregate(context.Context, metering.RefreshAggregateCommand) (*metering.MeteringAggregate, error) {
	return nil, nil
}

func (r *dedupeBillingRepo) Rows() []*metering.MeteringEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*metering.MeteringEvent, 0, len(r.rows))
	for _, e := range r.rows {
		out = append(out, e)
	}
	return out
}

func (r *dedupeBillingRepo) rowByEventType(t *testing.T, eventType string) *metering.MeteringEvent {
	t.Helper()
	for _, e := range r.Rows() {
		if e.EventType == eventType {
			return e
		}
	}
	t.Fatalf("no metering row with event_type %q", eventType)
	return nil
}

func uuidUsageOutboxItem(t *testing.T) *platformoutbox.OutboxItem {
	t.Helper()
	orgID := "org-uuid-1"
	payload := map[string]any{
		"api_key_id":    "key-uuid-1",
		"input_tokens":  120,
		"output_tokens": 80,
		"total_tokens":  200,
		"cost_amount":   0.021, // → 21 milli-yuan
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return &platformoutbox.OutboxItem{
		ID:             "9f0f5b60-4a6e-7c31-9d6a-6b0f5a7c1e21",
		OrganizationID: &orgID,
		AggregateType:  "llm_usage",
		EventType:      "ai.usage.recorded",
		Payload:        payloadBytes,
	}
}

// TestMeteringHandler_BillingSink_EventIDsAreUUIDs 验证写入 metering_events
// 的两条 fact 使用 UUID 主键（列类型 UUID），而 IdempotencyKey 仍是原去重键。
func TestMeteringHandler_BillingSink_EventIDsAreUUIDs(t *testing.T) {
	billingRepo := newDedupeBillingRepo()
	handlers := NewOutboxHandlers(nil, slog.Default(), billingRepo, platformuuid.V7Generator{})
	h := handlers["llm_usage"]
	item := uuidUsageOutboxItem(t)

	if err := h.Handle(context.Background(), item); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	tokensEvt := billingRepo.rowByEventType(t, "ai.usage.tokens")
	costEvt := billingRepo.rowByEventType(t, "ai.usage.cost")

	for _, evt := range []*metering.MeteringEvent{tokensEvt, costEvt} {
		if !platformuuid.IsValid(evt.ID) {
			t.Errorf("metering_events.id = %q is not a UUID (column type is UUID; Postgres rejects it)", evt.ID)
		}
	}
	if tokensEvt.ID == costEvt.ID {
		t.Errorf("tokens and cost facts share id %q, want distinct UUIDs", tokensEvt.ID)
	}
	if tokensEvt.IdempotencyKey != item.ID+":tokens" {
		t.Errorf("tokens idempotency_key = %q, want %q", tokensEvt.IdempotencyKey, item.ID+":tokens")
	}
	if costEvt.IdempotencyKey != item.ID+":cost" {
		t.Errorf("cost idempotency_key = %q, want %q", costEvt.IdempotencyKey, item.ID+":cost")
	}
}

// TestMeteringHandler_BillingSink_ReplayLandsOneRowPerFact 验证同一 outbox
// 事件被 Worker 重试多次时，tokens / cost 各只落一条（ON CONFLICT DO NOTHING
// 语义），且每次尝试重新生成的 ID 都是合法 UUID。
func TestMeteringHandler_BillingSink_ReplayLandsOneRowPerFact(t *testing.T) {
	billingRepo := newDedupeBillingRepo()
	handlers := NewOutboxHandlers(nil, slog.Default(), billingRepo, platformuuid.V7Generator{})
	h := handlers["llm_usage"]
	item := uuidUsageOutboxItem(t)

	const attempts = 3
	for i := 0; i < attempts; i++ {
		if err := h.Handle(context.Background(), item); err != nil {
			t.Fatalf("attempt %d: Handle() error = %v", i+1, err)
		}
	}

	if billingRepo.calls != attempts*2 {
		t.Errorf("InsertEvent calls = %d, want %d (2 facts per attempt)", billingRepo.calls, attempts*2)
	}
	rows := billingRepo.Rows()
	if len(rows) != 2 {
		t.Fatalf("metering_events rows = %d, want 2 (replay must not duplicate facts)", len(rows))
	}
	for _, evt := range rows {
		if !platformuuid.IsValid(evt.ID) {
			t.Errorf("metering_events.id = %q is not a UUID on replay", evt.ID)
		}
	}
}

// errorUUIDGenerator 用于验证 ID 生成失败时整条 fact 不被写入。
type errorUUIDGenerator struct{ err error }

func (g errorUUIDGenerator) NewString() (string, error) { return "", g.err }

func TestMeteringHandler_BillingSink_IDGeneratorErrorReturnsError(t *testing.T) {
	billingRepo := newDedupeBillingRepo()
	wantErr := errors.New("uuid source unavailable")
	handlers := NewOutboxHandlers(nil, slog.Default(), billingRepo, errorUUIDGenerator{err: wantErr})
	h := handlers["llm_usage"]
	item := uuidUsageOutboxItem(t)

	err := h.Handle(context.Background(), item)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Handle() error = %v, want wrapped generator error", err)
	}
	if billingRepo.calls != 0 {
		t.Errorf("InsertEvent calls = %d, want 0 (no id generated → no insert)", billingRepo.calls)
	}
}

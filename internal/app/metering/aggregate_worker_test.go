package metering

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
)

// ── fake MeteringRepository ────────────────────────────────────────────

type fakeRepo struct {
	mu           sync.Mutex
	combos       []domain.Combo
	refreshCalls []domain.RefreshAggregateCommand
	refreshFn    func() (*domain.MeteringAggregate, error) // injectable error
}

func (f *fakeRepo) ScanDistinctCombos(ctx context.Context, since time.Time) ([]domain.Combo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.combos, nil
}
func (f *fakeRepo) ListEvents(ctx context.Context, q domain.ListEventsQuery) ([]*domain.MeteringEvent, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) InsertEvent(ctx context.Context, e *domain.MeteringEvent) (bool, error) {
	return true, nil
}
func (f *fakeRepo) IngestBatch(ctx context.Context, events []*domain.MeteringEvent) (inserted, duplicates int, err error) {
	return len(events), 0, nil
}
func (f *fakeRepo) ListAggregates(ctx context.Context, q domain.ListAggregatesQuery) ([]*domain.MeteringAggregate, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) UpsertAggregate(ctx context.Context, agg *domain.MeteringAggregate) error {
	return nil
}
func (f *fakeRepo) RefreshAggregate(ctx context.Context, cmd domain.RefreshAggregateCommand) (*domain.MeteringAggregate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshCalls = append(f.refreshCalls, cmd)
	if f.refreshFn != nil {
		return f.refreshFn()
	}
	return &domain.MeteringAggregate{}, nil
}

// fake MeteringUoW 已移除 —— NewAggregateWorker 直接吃 domain.Repository

func newTestWorker(combos []domain.Combo) (*AggregateWorker, *fakeRepo) {
	r := &fakeRepo{combos: combos}
	w := NewAggregateWorker(r, slog.Default(), 5*time.Minute)
	w.Clock = func() time.Time {
		return time.Date(2026, 10, 15, 11, 30, 0, 0, time.UTC)
	}
	w.PeriodTypes = []PeriodKind{PeriodHour, PeriodDay}
	return w, r
}

func TestAggregateWorker_WindowsInRange(t *testing.T) {
	// now = 2026-10-15 11:30 UTC
	// lookback = 3h → from = 08:30
	// PeriodHour → 08-09, 09-10, 10-11, 11-12  → 4 windows
	// PeriodDay  → 2026-10-15 全天            → 1 window
	// 总共 5 次调用
	combos := []domain.Combo{{OrganizationID: "org1", Subject: "user1", EventType: "ai.usage.tokens"}}
	w2, r := newTestWorker(combos)
	refreshed, errs := w2.RefreshAllPeriods(context.Background())
	if errs != 0 {
		t.Fatalf("expected 0 errors, got %d", errs)
	}
	// 1 combo × (4 hour windows + 1 day window) = 5
	if refreshed != 5 {
		t.Fatalf("expected 5 refreshes (1 combo × 4h + 1d), got %d", refreshed)
	}
	if len(r.refreshCalls) != 5 {
		t.Fatalf("expected 5 RefreshAggregate calls, got %d", len(r.refreshCalls))
	}
}

func TestAggregateWorker_MultipleCombos(t *testing.T) {
	// 2 combos × (4h + 1d) = 10 calls
	combos := []domain.Combo{
		{OrganizationID: "org1", Subject: "user1", EventType: "ai.usage.tokens"},
		{OrganizationID: "org1", Subject: "user1", EventType: "ai.usage.cost"},
	}
	w, _ := newTestWorker(combos)
	refreshed, errs := w.RefreshAllPeriods(context.Background())
	if errs != 0 {
		t.Fatalf("expected 0 errors, got %d", errs)
	}
	if refreshed != 10 {
		t.Fatalf("expected 10 refreshes, got %d", refreshed)
	}
}

func TestAggregateWorker_IdempotentReRun(t *testing.T) {
	combos := []domain.Combo{{OrganizationID: "org1", Subject: "user1", EventType: "ai.usage.tokens"}}
	w, r := newTestWorker(combos)
	// 跑两次
	r1, e1 := w.RefreshAllPeriods(context.Background())
	r2, e2 := w.RefreshAllPeriods(context.Background())
	if e1 != 0 || e2 != 0 {
		t.Fatalf("errors: %d, %d", e1, e2)
	}
	if r1 != r2 {
		t.Fatalf("not idempotent: first=%d, second=%d", r1, r2)
	}
	// RefreshAggregate 被调用 r1 + r2 = 10 次（幂等 SUM，没问题）
	if len(r.refreshCalls) != r1+r2 {
		t.Fatalf("expected %d total calls, got %d", r1+r2, len(r.refreshCalls))
	}
}

func TestAggregateWorker_EmptyCombos_Noop(t *testing.T) {
	w, r := newTestWorker(nil)
	refreshed, errs := w.RefreshAllPeriods(context.Background())
	if refreshed != 0 || errs != 0 {
		t.Fatalf("expected 0/0, got %d/%d", refreshed, errs)
	}
	if len(r.refreshCalls) != 0 {
		t.Fatalf("expected 0 RefreshAggregate calls, got %d", len(r.refreshCalls))
	}
}

func TestAggregateWorker_RepoError_FailOpen(t *testing.T) {
	combos := []domain.Combo{{OrganizationID: "org1", Subject: "user1", EventType: "ai.usage.tokens"}}
	w, r := newTestWorker(combos)
	// 让 RefreshAggregate 返回 error
	callCount := 0
	r.refreshFn = func() (*domain.MeteringAggregate, error) {
		callCount++
		if callCount == 2 { // 第二次调用失败（任意一个窗口）
			return nil, errors.New("db down")
		}
		return &domain.MeteringAggregate{}, nil
	}
	refreshed, errs := w.RefreshAllPeriods(context.Background())
	// 总共 5 个窗口，1 个失败 → errs=1, refreshed=4
	if errs != 1 {
		t.Fatalf("expected 1 error, got %d", errs)
	}
	if refreshed != 4 {
		t.Fatalf("expected 4 refreshed, got %d", refreshed)
	}
	// ScanDistinctCombos 没报错 → worker 继续处理
	// 验证没 panic，后续窗口仍继续
}

func TestAggregateWorker_PeriodLabelFormats(t *testing.T) {
	// 验证 periodLabelFor 正确生成 hour/day/month 格式
	cases := []struct {
		kind PeriodKind
		t    time.Time
		want string
	}{
		{PeriodHour, time.Date(2026, 10, 15, 11, 30, 0, 0, time.UTC), "2026-10-15T11"},
		{PeriodDay, time.Date(2026, 10, 15, 11, 30, 0, 0, time.UTC), "2026-10-15"},
		{PeriodMonth, time.Date(2026, 10, 15, 11, 30, 0, 0, time.UTC), "2026-10"},
	}
	for _, c := range cases {
		got := periodLabelFor(c.kind, c.t)
		if got != c.want {
			t.Errorf("periodLabelFor(%v, %v) = %q, want %q", c.kind, c.t, got, c.want)
		}
	}
}

func TestAggregateWorker_PeriodStartTruncation(t *testing.T) {
	tm := time.Date(2026, 10, 15, 11, 30, 45, 123456789, time.UTC)
	if got := periodStartFor(PeriodHour, tm); got != time.Date(2026, 10, 15, 11, 0, 0, 0, time.UTC) {
		t.Errorf("PeriodHour truncation wrong: %v", got)
	}
	if got := periodStartFor(PeriodDay, tm); got != time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) {
		t.Errorf("PeriodDay truncation wrong: %v", got)
	}
	if got := periodStartFor(PeriodMonth, tm); got != time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) {
		t.Errorf("PeriodMonth truncation wrong: %v", got)
	}
}

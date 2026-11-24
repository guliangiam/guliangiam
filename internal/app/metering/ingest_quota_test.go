package metering

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// quotaDedupRepo 模拟 metering_events 的 (organization_id, idempotency_key)
// 唯一约束，并记录每次 ConsumeQuota 的扣减量。
type quotaDedupRepo struct {
	domain.Repository

	seen     map[string]bool
	consumed []int64

	allowed     bool
	used, limit int64

	lastEvent *domain.MeteringEvent
}

func (r *quotaDedupRepo) IngestBatch(ctx context.Context, events []*domain.MeteringEvent) (inserted, duplicates int, err error) {
	for _, ev := range events {
		r.lastEvent = ev
		if r.seen[ev.IdempotencyKey] {
			duplicates++
			continue
		}
		r.seen[ev.IdempotencyKey] = true
		inserted++
	}
	return inserted, duplicates, nil
}

func (r *quotaDedupRepo) ConsumeQuota(ctx context.Context, organizationID, quotaKey string, delta int64) (domain.QuotaStatus, error) {
	r.consumed = append(r.consumed, delta)
	return domain.QuotaStatus{Allowed: r.allowed, Used: r.used, Limit: r.limit}, nil
}

// quotaDedupUoW 只实现 Ingest 用到的 tx 能力；其余方法由嵌入的接口提供
// （测试中不会被调用，调用即 panic，正好暴露越界依赖）。
// Records()/Outbox() 返回 nil，使审计嵌入按「无仓储」跳过。
type quotaDedupUoW struct {
	appuow.UnitOfWork
	repo domain.Repository
}

func (u *quotaDedupUoW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *quotaDedupUoW) Metering() domain.Repository { return u.repo }

func (u *quotaDedupUoW) Records() domainrecords.Repository { return nil }

func (u *quotaDedupUoW) Outbox() domainoutbox.Repository { return nil }

func newQuotaDedupService(repo *quotaDedupRepo) Service {
	return Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: &quotaDedupUoW{repo: repo}}
}

func ingestCmds(keys ...string) []IngestEventCommand {
	cmds := make([]IngestEventCommand, 0, len(keys))
	for _, k := range keys {
		cmds = append(cmds, IngestEventCommand{
			EventType:      "ai.usage.tokens",
			Subject:        "user-1",
			Quantity:       10,
			Unit:           "token",
			IdempotencyKey: k,
			OccurredAt:     time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC),
		})
	}
	return cmds
}

// 同一批次重复投递：第二次全部被去重，配额不得再次扣减。
func TestIngest_ReplayDoesNotConsumeQuotaAgain(t *testing.T) {
	repo := &quotaDedupRepo{seen: map[string]bool{}, allowed: true, limit: 1000}
	svc := newQuotaDedupService(repo)
	ctx := context.Background()

	inserted, duplicates, err := svc.Ingest(ctx, "org-1", ingestCmds("k1", "k2", "k3"))
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if inserted != 3 || duplicates != 0 {
		t.Fatalf("first ingest = (%d inserted, %d duplicates), want (3, 0)", inserted, duplicates)
	}
	if len(repo.consumed) != 1 || repo.consumed[0] != 3 {
		t.Fatalf("first consume = %v, want [3]", repo.consumed)
	}

	inserted, duplicates, err = svc.Ingest(ctx, "org-1", ingestCmds("k1", "k2", "k3"))
	if err != nil {
		t.Fatalf("replay ingest: %v", err)
	}
	if inserted != 0 || duplicates != 3 {
		t.Fatalf("replay ingest = (%d inserted, %d duplicates), want (0, 3)", inserted, duplicates)
	}
	if len(repo.consumed) != 1 {
		t.Fatalf("replay must not consume quota again, consumed = %v, want [3]", repo.consumed)
	}
}

// 部分重复批次：只为新插入的行扣减。
func TestIngest_MixedBatchConsumesOnlyNewEvents(t *testing.T) {
	repo := &quotaDedupRepo{seen: map[string]bool{}, allowed: true, limit: 1000}
	svc := newQuotaDedupService(repo)
	ctx := context.Background()

	if _, _, err := svc.Ingest(ctx, "org-1", ingestCmds("k1", "k2")); err != nil {
		t.Fatalf("seed ingest: %v", err)
	}

	inserted, duplicates, err := svc.Ingest(ctx, "org-1", ingestCmds("k1", "k2", "k3"))
	if err != nil {
		t.Fatalf("mixed ingest: %v", err)
	}
	if inserted != 1 || duplicates != 2 {
		t.Fatalf("mixed ingest = (%d inserted, %d duplicates), want (1, 2)", inserted, duplicates)
	}
	want := []int64{2, 1}
	if len(repo.consumed) != len(want) {
		t.Fatalf("consume calls = %v, want %v", repo.consumed, want)
	}
	for i := range want {
		if repo.consumed[i] != want[i] {
			t.Fatalf("consume calls = %v, want %v", repo.consumed, want)
		}
	}
}

// 硬上限拒绝时，扣减请求量按实际插入行数计算（重复行不计入）。
func TestIngest_QuotaRejectionCountsInsertedOnly(t *testing.T) {
	repo := &quotaDedupRepo{seen: map[string]bool{}, allowed: true, used: 100, limit: 100}
	svc := newQuotaDedupService(repo)
	ctx := context.Background()

	if _, _, err := svc.Ingest(ctx, "org-1", ingestCmds("k1", "k2")); err != nil {
		t.Fatalf("unexpected error when quota allows: %v", err)
	}

	// 只改 allowed，模拟配额在第二批前被占满。
	repo.allowed = false

	_, _, err := svc.Ingest(ctx, "org-1", ingestCmds("k1", "k2", "k3", "k4"))
	if err == nil {
		t.Fatal("expected quota rejection, got nil error")
	}
	if len(repo.consumed) != 2 || repo.consumed[1] != 2 {
		t.Fatalf("consume calls = %v, want second delta 2 (only k3/k4 are new)", repo.consumed)
	}
}

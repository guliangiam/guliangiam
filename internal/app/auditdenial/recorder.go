// Package auditdenial implements the best-effort denial audit recorder:
// in-process dedup (Redis-pluggable), an independent token bucket, and a
// write path that never fails the request.
package auditdenial

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/time/rate"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	platformaudit "github.com/guliangiam/guliangiam/internal/platform/auditdenial"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const defaultDedupWindow = 60 * time.Second

var denialDropped = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "guliangiam",
	Name:      "audit_denial_dropped_total",
	Help:      "Denial audit details dropped by rate limiting or write failure.",
}, []string{"reason"})

// Deduper reports whether a key was already seen inside the window.
// firstCount is the count after this hit (1 for the first occurrence).
type Deduper interface {
	Hit(ctx context.Context, key string, window time.Duration) (firstCount int, duplicate bool)
}

type memoryDeduper struct {
	mu      sync.Mutex
	entries map[string]dedupEntry
}

type dedupEntry struct {
	count   int
	expires time.Time
}

func NewMemoryDeduper() Deduper { return &memoryDeduper{entries: map[string]dedupEntry{}} }

func (d *memoryDeduper) Hit(_ context.Context, key string, window time.Duration) (int, bool) {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, entry := range d.entries {
		if now.After(entry.expires) {
			delete(d.entries, k)
		}
	}
	if entry, ok := d.entries[key]; ok {
		entry.count++
		d.entries[key] = entry
		return entry.count, true
	}
	d.entries[key] = dedupEntry{count: 1, expires: now.Add(window)}
	return 1, false
}

type Recorder struct {
	UOW      appuow.AuditUoW
	Clock    clock.Clock
	UUID     platformuuid.Generator
	Deduper  Deduper
	Limiter  *rate.Limiter
	Log      *slog.Logger
	DedupWin time.Duration

	retryCh   chan *domainrecords.Record
	retryOnce sync.Once
}

// NewRecorder constructs the L3/L4 recorder. RATE_LIMIT_AUDIT_DENIAL_RATE
// defaults to 10 events/second with a 20-event burst.
func NewRecorder(uow appuow.AuditUoW, clock clock.Clock, uuid platformuuid.Generator) *Recorder {
	perSecond := platformconfig.RateLimitAuditDenialRate()
	burst := perSecond * 2
	if burst < 10 {
		burst = 10
	}
	return &Recorder{
		UOW:      uow,
		Clock:    clock,
		UUID:     uuid,
		Deduper:  NewMemoryDeduper(),
		Limiter:  rate.NewLimiter(rate.Limit(perSecond), burst),
		Log:      slog.Default(),
		DedupWin: defaultDedupWindow,
		retryCh:  make(chan *domainrecords.Record, 256),
	}
}

// Record writes at most one detail row per dedup key/window and never returns
// an error to the request path.
func (r *Recorder) Record(ctx context.Context, event platformaudit.Event) {
	if r == nil || r.UOW == nil || strings.TrimSpace(event.OrganizationID) == "" {
		return
	}
	action := event.Action
	if action == "" {
		action = "authz.deny"
	}
	key := strings.Join([]string{event.ActorID, event.TargetTenant, event.ResourceType, event.Reason}, "|")
	count := 1
	if r.Deduper != nil {
		var duplicate bool
		count, duplicate = r.Deduper.Hit(ctx, key, r.DedupWin)
		if duplicate {
			return
		}
	}
	if r.Limiter != nil && !r.Limiter.Allow() {
		denialDropped.WithLabelValues("rate_limited").Inc()
		r.log().WarnContext(ctx, "audit denial rate limited", "action", action, "reason", event.Reason)
		return
	}
	generator := r.UUID
	if generator == nil {
		generator = platformuuid.V7Generator{}
	}
	id, err := generator.NewString()
	if err != nil {
		denialDropped.WithLabelValues("write_failed").Inc()
		return
	}
	now := time.Now().UTC()
	if r.Clock != nil {
		now = r.Clock.Now().UTC()
	}
	metadata := make(map[string]any, len(event.Metadata)+3)
	for k, v := range event.Metadata {
		metadata[k] = v
	}
	metadata["reason"] = event.Reason
	metadata["count"] = count
	if event.TargetTenant != "" {
		metadata["targetTenant"] = event.TargetTenant
	}
	record := &domainrecords.Record{
		ID:             id,
		OrganizationID: event.OrganizationID,
		ActorID:        event.ActorID,
		Action:         action,
		ResourceType:   event.ResourceType,
		ResourceID:     event.ResourceID,
		IP:             sanitizeIP(event.IP),
		UserAgent:      event.UserAgent,
		Surface:        domainrecords.NormalizeSurface(event.Surface),
		RequestID:      event.RequestID,
		TraceID:        event.TraceID,
		Metadata:       metadata,
		CreatedAt:      now,
	}
	systemCtx := tenantctx.System(ctx)
	err = r.UOW.WithinTransaction(systemCtx, func(txCtx context.Context, tx appuow.UnitOfWork) error {
		return tx.Records().Create(txCtx, record)
	})
	if err != nil {
		denialDropped.WithLabelValues("write_failed").Inc()
		r.log().WarnContext(ctx, "audit denial write failed", "error", err)
		r.enqueueRetry(record)
	}
}

// enqueueRetry hands the failed record to the bounded L5 compensation worker.
// The queue is best-effort and process-local: dropping is counted, never blocks
// the request.
func (r *Recorder) enqueueRetry(record *domainrecords.Record) {
	if r == nil || record == nil || r.retryCh == nil {
		return
	}
	r.retryOnce.Do(func() { recover.SafeGo("audit-denial-retry", r.retryLoop) })
	select {
	case r.retryCh <- record:
	default:
		denialDropped.WithLabelValues("retry_queue_full").Inc()
	}
}

func (r *Recorder) retryLoop() {
	for record := range r.retryCh {
		attempts := 3
		written := false
		for attempt := 0; attempt < attempts; attempt++ {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			systemCtx := tenantctx.System(context.Background())
			err := r.UOW.WithinTransaction(systemCtx, func(txCtx context.Context, tx appuow.UnitOfWork) error {
				return tx.Records().Create(txCtx, record)
			})
			if err == nil {
				written = true
				break
			}
		}
		if !written {
			denialDropped.WithLabelValues("retry_exhausted").Inc()
		}
	}
}

func (r *Recorder) log() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

// sanitizeIP 把 http.Request.RemoteAddr（host:port）规范化为 INET 可接受的地址；
// 非法值返回空串（records.ip 可空）。历史上直接把 host:port 写入 INET 列会触发
// SQLSTATE 22P02，使拒绝审计整体写失败（DEF-18）。
func sanitizeIP(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	s = strings.TrimPrefix(strings.TrimSuffix(s, "]"), "[")
	if net.ParseIP(s) == nil {
		return ""
	}
	return s
}

// SafeGo 兼容包装：重试循环通过 recover.SafeGo 启动（DEF-01）。

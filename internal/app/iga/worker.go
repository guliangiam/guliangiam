package iga

import (
	"context"
	"log/slog"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// ExpireWorker scans lifecycle rows and reclaims every JIT/PAM artifact in the
// same transaction. An empty org list means one global scan.
type ExpireWorker struct {
	UOW       UnitOfWork
	Interval  time.Duration
	BatchSize int
	Now       func() time.Time
	orgIDs    []string
}

func NewExpireWorker(uow UnitOfWork, interval time.Duration, batchSize int, now func() time.Time, orgIDs []string) ExpireWorker {
	if interval <= 0 {
		interval = time.Minute
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return ExpireWorker{UOW: uow, Interval: interval, BatchSize: batchSize, Now: now, orgIDs: append([]string(nil), orgIDs...)}
}

func (w ExpireWorker) Run(ctx context.Context) error {
	var lastErr error
	if err := w.tick(ctx); err != nil {
		lastErr = err
	}
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "iga-worker shutting down", "reason", ctx.Err())
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				lastErr = err
			}
		}
	}
}

func (w ExpireWorker) tick(ctx context.Context) error {
	ctx = tenantctx.System(ctx)
	start := w.Now()
	scopes := w.orgIDs
	if len(scopes) == 0 {
		scopes = []string{""}
	}
	var totalExpired int64
	var orgsProcessed int
	var lastErr error
	for _, orgID := range scopes {
		if err := ctx.Err(); err != nil {
			return err
		}
		var items []*domain.AccessLifecycle
		err := w.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
			var err error
			items, err = tx.IGA().ListExpiringLifecycle(ctx, orgID, start, w.BatchSize)
			return err
		})
		if err != nil {
			slog.ErrorContext(ctx, "iga-worker list expiring grants failed", "org_id", orgID, "error", err)
			lastErr = err
			continue
		}
		orgsProcessed++
		for _, item := range items {
			completed, err := w.reclaimOne(ctx, item, start)
			if err != nil {
				slog.ErrorContext(ctx, "iga-worker reclaim failed", "lifecycle_id", item.ID, "error", err)
				lastErr = err
				if ctx.Err() == nil {
					if failureErr := w.recordFailure(ctx, item, start, err.Error()); failureErr != nil {
						slog.ErrorContext(ctx, "iga-worker record reclaim failure failed", "lifecycle_id", item.ID, "error", failureErr)
					}
				}
				continue
			}
			if completed {
				totalExpired++
			}
		}
	}
	slog.InfoContext(ctx, "iga-worker run summary", "total_expired", totalExpired, "orgs_processed", orgsProcessed, "elapsed_ms", w.Now().Sub(start).Milliseconds())
	return lastErr
}

func (w ExpireWorker) reclaimOne(ctx context.Context, candidate *domain.AccessLifecycle, now time.Time) (bool, error) {
	completed := false
	err := w.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		lifecycle, err := tx.IGA().GetLifecycleForUpdate(ctx, candidate.OrganizationID, candidate.ID)
		if err != nil {
			return err
		}
		if lifecycle.Status != domain.LifecycleActive || lifecycle.ExpiresAt.After(now) {
			return nil
		}
		var reclaimErr error
		completed, reclaimErr = reclaimJITGrant(ctx, tx, lifecycle, domain.LifecycleExpired, now)
		if reclaimErr != nil || !completed {
			return reclaimErr
		}
		service := Service{Clock: staticClock{at: now}, UOW: w.UOW}
		return service.embedAuditEvent(tx, ctx, lifecycle.OrganizationID, "",
			"iga.jit.expired", "lifecycle", lifecycle.ID, "lifecycle", "iga.jit.expired",
			map[string]any{keyLifecycleID: lifecycle.ID, keyPolicyID: lifecycle.PolicyID, "expiredAt": now})
	})
	return completed, err
}

func (w ExpireWorker) recordFailure(ctx context.Context, lifecycle *domain.AccessLifecycle, now time.Time, message string) error {
	return w.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.IGA().RecordLifecycleFailure(ctx, lifecycle.OrganizationID, lifecycle.ID, message); err != nil {
			return err
		}
		service := Service{Clock: staticClock{at: now}, UOW: w.UOW}
		return service.embedAuditEvent(tx, ctx, lifecycle.OrganizationID, "",
			"iga.jit.revoke_failed", "lifecycle", lifecycle.ID, "lifecycle", "iga.jit.revoke_failed",
			map[string]any{keyLifecycleID: lifecycle.ID, "error": message})
	})
}

type staticClock struct{ at time.Time }

func (c staticClock) Now() time.Time { return c.at }

package metering

import (
	"context"
	"log/slog"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
)

// AggregateWorker 周期性扫 metering_events → 幂等 RefreshAggregate 入 metering_aggregates。
// 管道位置：docs/design/commerce/metering.md — step 2 「Outbox Worker 聚合」
//
// 设计要点：
//  1. 全量 refresh 而非逐事件消费：RefreshAggregate SQL 是 SUM + ON CONFLICT DO UPDATE
//     → 幂等、跨窗口安全、容忍重启/重复/乱序事件
//  2. 扫描边界：period_start 回溯 now - lookback（默认 3h），覆盖 T+1 迟到容忍
//  3. Fail-open：单窗口 error 不阻塞其他窗口，下周期重跑自愈
//  4. 无状态：不记录上次扫描位置，纯事件驱动（metering_events append-only）
type AggregateWorker struct {
	Repo        domain.Repository
	Logger      *slog.Logger
	Interval    time.Duration // 默认 5min
	Lookback    time.Duration // 默认 3h — 回溯扫描窗口
	PeriodTypes []PeriodKind  // 默认 [hour, day]
	// Clock 方便测试注入
	Clock func() time.Time
}

// PeriodKind 是聚合窗口粒度。与 domain.Period 字符串 label 对应。
type PeriodKind int

const (
	PeriodHour PeriodKind = iota
	PeriodDay
	PeriodMonth
)

func (k PeriodKind) String() string {
	switch k {
	case PeriodHour:
		return "hour"
	case PeriodDay:
		return "day"
	case PeriodMonth:
		return "month"
	}
	return "unknown"
}

// NewAggregateWorker 构造一个带合理默认值的 worker。
func NewAggregateWorker(repo domain.Repository, logger *slog.Logger, interval time.Duration) *AggregateWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &AggregateWorker{
		Repo:        repo,
		Logger:      logger,
		Interval:    interval,
		Lookback:    3 * time.Hour,
		PeriodTypes: []PeriodKind{PeriodHour, PeriodDay},
		Clock:       time.Now,
	}
}

// Start 阻塞直到 ctx cancel。每个 Interval 周期执行 RefreshAllPeriods。
func (w *AggregateWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	// 启动先跑一次（避免首次等待一个完整周期才聚合）
	w.runOnce(ctx, "startup")

	for {
		select {
		case <-ctx.Done():
			w.Logger.Info("metering aggregate worker stopped")
			return nil
		case now := <-ticker.C:
			w.runOnce(ctx, now.Format("15:04:05"))
		}
	}
}

func (w *AggregateWorker) runOnce(ctx context.Context, trigger string) {
	start := time.Now()
	refreshed, errs := w.RefreshAllPeriods(ctx)
	elapsed := time.Since(start)
	w.Logger.Info("metering aggregate refresh done",
		"trigger", trigger,
		"windows_refreshed", refreshed,
		"errors", errs,
		"elapsed_ms", elapsed.Milliseconds(),
	)
}

// RefreshAllPeriods 扫描最近 lookback 时间窗内 metering_events 涉及的所有
// (org, subject, metric, period, period_start) 组合 → 对每个调 repo.RefreshAggregate()。
// 返回刷新窗口数 + 失败数。Fail-open：单窗口 error 不阻塞其他。
func (w *AggregateWorker) RefreshAllPeriods(ctx context.Context) (refreshed, errs int) {
	// 1. 扫最近 lookback 时间窗内所有 distinct (org, subject, event_type)
	combos, err := w.scanCombos(ctx)
	if err != nil {
		w.Logger.Error("aggregate worker: scan combos failed", "error", err)
		return 0, 0
	}
	if len(combos) == 0 {
		return 0, 0
	}

	now := w.Clock().UTC()

	// 2. 对每个 combo × 每个 period kind × 每个窗口 调 RefreshAggregate
	for _, combo := range combos {
		for _, pk := range w.PeriodTypes {
			windows := w.windowsInRange(pk, now.Add(-w.Lookback), now)
			for _, win := range windows {
				periodLabel := periodLabelFor(pk, win.start)
				cmd := domain.RefreshAggregateCommand{
					OrganizationID: combo.orgID,
					Subject:        combo.subject,
					Metric:         combo.eventType, // event_type 即 metric
					Period:         domain.Period(periodLabel),
					PeriodStart:    &win.start,
					PeriodEnd:      &win.end,
				}
				if _, err := w.Repo.RefreshAggregate(ctx, cmd); err != nil {
					w.Logger.Error("aggregate refresh failed",
						"org", combo.orgID,
						"subject", combo.subject,
						"metric", combo.eventType,
						"period", periodLabel,
						"period_kind", pk.String(),
						"error", err,
					)
					errs++
					continue // fail-open
				}
				refreshed++
			}
		}
	}
	return refreshed, errs
}

// scanCombos 取最近 lookback 时间窗内所有 distinct (org, subject, event_type) 组合。
// 直接走 repo.ScanDistinctCombos（底层 SQL: SELECT DISTINCT ... WHERE occurred_at >= $1）。
func (w *AggregateWorker) scanCombos(ctx context.Context) ([]combo, error) {
	since := w.Clock().UTC().Add(-w.Lookback)
	rows, err := w.Repo.ScanDistinctCombos(ctx, since)
	if err != nil {
		return nil, err
	}
	out := make([]combo, 0, len(rows))
	for _, r := range rows {
		out = append(out, combo{
			orgID:     r.OrganizationID,
			subject:   r.Subject,
			eventType: r.EventType,
		})
	}
	return dedupCombos(out), nil
}

func (w *AggregateWorker) windowsInRange(pk PeriodKind, from, to time.Time) []periodWindow {
	var out []periodWindow
	cur := periodStartFor(pk, from)
	for cur.Before(to) {
		out = append(out, periodWindow{
			start: cur,
			end:   periodEndFor(pk, cur),
		})
		cur = periodEndFor(pk, cur) // 下一个窗口从当前窗口结束开始
		// 安全上限：最多 100 个窗口（防止 lookback 配太大时无限循环）
		if len(out) >= 100 {
			break
		}
	}
	return out
}

// ─── helpers ──────────────────────────────────────────────────────────────

type combo struct {
	orgID     string
	subject   string
	eventType string
}

type periodWindow struct {
	start time.Time
	end   time.Time
}

func dedupCombos(in []combo) []combo {
	seen := make(map[string]struct{}, len(in))
	out := make([]combo, 0, len(in))
	for _, c := range in {
		k := c.orgID + "|" + c.subject + "|" + c.eventType
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, c)
	}
	return out
}

func periodStartFor(pk PeriodKind, t time.Time) time.Time {
	t = t.UTC()
	switch pk {
	case PeriodHour:
		return t.Truncate(time.Hour)
	case PeriodDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	case PeriodMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return t.Truncate(time.Hour)
}

func periodEndFor(pk PeriodKind, start time.Time) time.Time {
	switch pk {
	case PeriodHour:
		return start.Add(time.Hour)
	case PeriodDay:
		return start.AddDate(0, 0, 1)
	case PeriodMonth:
		return start.AddDate(0, 1, 0)
	}
	return start.Add(time.Hour)
}

func periodLabelFor(pk PeriodKind, start time.Time) string {
	start = start.UTC()
	switch pk {
	case PeriodHour:
		return start.Format("2006-01-02T15")
	case PeriodDay:
		return start.Format("2006-01-02")
	case PeriodMonth:
		return start.Format("2006-01")
	}
	return start.Format("2006-01")
}

package metering

import (
	"github.com/guliangiam/guliangiam/internal/infra/dialect"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"

	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
)

// SQL 方言转换规则（PG → MySQL）：
//   "guliangiam".table                 → `table`
//   ON CONFLICT (cols) DO NOTHING      → INSERT IGNORE
//   ON CONFLICT DO UPDATE SET EXCLUDED.x → ON DUPLICATE KEY UPDATE col = VALUES(col)
//   RETURNING ...                      → 拆成执行后 SELECT 回读
//   now()                              → NOW()
//   to_char(occurred_at, 'YYYY-MM')   → DATE_FORMAT(occurred_at, '%Y-%m')

const btEvents = "`metering_events`"
const btAggregates = "`metering_aggregates`"

// paginationSuffix 是 MySQL 参数化分页片段（含前导空格，SQL 文本逐字不变）。
const paginationSuffix = " LIMIT ? OFFSET ?"

// queryScanDistinctCombos MySQL 版：同 PG —— 取最近时间窗内 distinct (org, subject, event_type)
const queryScanDistinctCombos = "SELECT DISTINCT organization_id, subject, event_type FROM `metering_events` WHERE occurred_at >= ?"

// ─── Events ──────────────────────────────────────────────────────────

// buildListEventsSQL 构造 ListEvents/Count 的动态 SQL。
// 返回 (selectSQL, countSQL, args)。
func buildListEventsSQL(q domain.ListEventsQuery, withPaging bool) (queryOut, whereClauseOut string, argsOut []any) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.EventType != nil && *q.EventType != "" {
		args = append(args, *q.EventType)
		extra += " AND event_type = ?"
	}
	if q.Subject != nil && *q.Subject != "" {
		args = append(args, *q.Subject)
		extra += " AND subject = ?"
	}
	if q.From != nil {
		args = append(args, *q.From)
		extra += " AND occurred_at >= ?"
	}
	if q.To != nil {
		args = append(args, *q.To)
		extra += " AND occurred_at <= ?"
	}

	selectBase := `SELECT id, organization_id, event_type, subject, quantity, unit, idempotency_key, occurred_at, metadata, created_at FROM ` + btEvents + ` WHERE organization_id = ?`
	countBase := `SELECT COUNT(*) FROM ` + btEvents + ` WHERE organization_id = ?`

	countSQL := countBase + extra
	selectSQL := selectBase + extra + " ORDER BY occurred_at DESC"
	if withPaging {
		selectSQL += paginationSuffix
	}
	return selectSQL, countSQL, args
}

// ─── Aggregates ─────────────────────────────────────────────────────

func buildListAggregatesSQL(q domain.ListAggregatesQuery, withPaging bool) (queryOut, whereClauseOut string, argsOut []any) {
	args := []any{q.OrganizationID}
	extra := ""
	if q.Period != nil && *q.Period != "" {
		args = append(args, string(*q.Period))
		extra += " AND period = ?"
	}
	if q.Subject != nil && *q.Subject != "" {
		args = append(args, *q.Subject)
		extra += " AND subject = ?"
	}
	if q.Metric != nil && *q.Metric != "" {
		args = append(args, *q.Metric)
		extra += " AND metric = ?"
	}

	selectBase := `SELECT id, organization_id, period, subject, metric, quantity, unit, calculated_at, created_at FROM ` + btAggregates + ` WHERE organization_id = ?`
	countBase := `SELECT COUNT(*) FROM ` + btAggregates + ` WHERE organization_id = ?`

	countSQL := countBase + extra
	selectSQL := selectBase + extra + " ORDER BY period DESC"
	if withPaging {
		selectSQL += paginationSuffix
	}
	return selectSQL, countSQL, args
}

var (
	// queryInsertEvent — INSERT IGNORE 替代 ON CONFLICT DO NOTHING
	queryInsertEvent = `
INSERT IGNORE INTO ` + btEvents + `
  (id, organization_id, event_type, subject, quantity, unit, idempotency_key, occurred_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpsertAggregate — ON DUPLICATE KEY UPDATE 替代 ON CONFLICT DO UPDATE
	queryUpsertAggregate = `
INSERT INTO ` + btAggregates + `
  (id, organization_id, period, subject, metric, quantity, unit)
VALUES (?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(
		dialect.DialectUpsert{
			UpdateColumns: []string{"quantity", "unit", "calculated_at"},
		}, nil) + ``

	// queryRefreshAggregate — MySQL 版（动态 SQL + ON DUPLICATE KEY UPDATE）
	// 两种模式：同 PG SQL —— PeriodStart/PeriodEnd 显式时间边界或 DATE_FORMAT fallback
	queryRefreshAggregate = `
WITH agg AS (
  SELECT COALESCE(SUM(quantity), 0) AS total, COALESCE(MAX(unit), '') AS unit
  FROM ` + btEvents + `
  WHERE organization_id = ?
    AND subject = ?
    AND event_type = ?
    AND (
      (? IS NOT NULL AND ? IS NOT NULL AND occurred_at >= ? AND occurred_at < ?)
      OR
      (? IS NULL AND ? IS NULL AND DATE_FORMAT(occurred_at, '%%Y-%%m') = ?)
    )
)
INSERT INTO ` + btAggregates + `
  (id, organization_id, period, subject, metric, quantity, unit)
SELECT ?, ?, ?, ?, ?, agg.total, agg.unit
FROM agg
` + mysqlshared.Dialect.Upsert(
		dialect.DialectUpsert{
			UpdateColumns: []string{"quantity", "unit", "calculated_at"},
		}, nil) + ``

	// queryRefreshAggregateSelect — 回读 Upsert 后的 aggregate（替代 PG RETURNING）
	queryRefreshAggregateSelect = `
SELECT id, organization_id, period, subject, metric, quantity, unit, calculated_at, created_at
FROM ` + btAggregates + `
WHERE organization_id = ? AND period = ? AND subject = ? AND metric = ?`
)

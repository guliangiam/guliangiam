package metering

const queryListEventsSelect = `
SELECT id, organization_id, event_type, subject, quantity, unit, idempotency_key, occurred_at, metadata, created_at
FROM guliangiam.metering_events
WHERE organization_id = $1
`

// queryCountEvents 是列表查询的计数投影 —— 不能把 `SELECT count(*)` 直接拼在
// queryListEventsSelect 前面（会得到 `SELECT count(*) SELECT id, ...` 的语法错误）。
const queryCountEvents = `
SELECT count(*) FROM guliangiam.metering_events WHERE organization_id = $1
`

const queryInsertEvent = `
INSERT INTO guliangiam.metering_events
  (id, organization_id, event_type, subject, quantity, unit, idempotency_key, occurred_at, metadata)
VALUES
  ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (organization_id, idempotency_key) DO NOTHING
`

// queryScanDistinctCombos 取最近时间窗内 distinct (org, subject, event_type) 组合 ——
// 供后台 worker 批量 RefreshAggregate 用。用 idx_metering_events_occurred 索引。
const queryScanDistinctCombos = `
SELECT DISTINCT organization_id, subject, event_type
FROM guliangiam.metering_events
WHERE occurred_at >= $1
`

const queryListAggregatesSelect = `
SELECT id, organization_id, period, subject, metric, quantity, unit, calculated_at, created_at
FROM guliangiam.metering_aggregates
WHERE organization_id = $1
`

// queryCountAggregates 同上。
const queryCountAggregates = `
SELECT count(*) FROM guliangiam.metering_aggregates WHERE organization_id = $1
`

const queryUpsertAggregate = `
INSERT INTO guliangiam.metering_aggregates
  (id, organization_id, period, subject, metric, quantity, unit)
VALUES
  ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (organization_id, period, subject, metric) DO UPDATE
  SET quantity = EXCLUDED.quantity,
      unit     = EXCLUDED.unit,
      calculated_at = now()
`

// refreshAggregateSQL sums events for an (org, period, subject, metric) window.
// 两种模式：
//
//	a) PeriodStart/PeriodEnd 都提供 → 显式时间边界（支持 hour/day/month 任意窗口）
//	b) 未提供 → fallback to to_char(occurred_at, 'YYYY-MM') = period_label（历史月度模式）
const queryRefreshAggregate = `
WITH agg AS (
  SELECT COALESCE(SUM(quantity), 0) AS total, COALESCE(MAX(unit), '') AS unit
  FROM guliangiam.metering_events
  WHERE organization_id = $1
    AND subject = $2
    AND event_type = $3
    AND (
      ($6::timestamptz IS NOT NULL AND $7::timestamptz IS NOT NULL AND occurred_at >= $6 AND occurred_at < $7)
      OR
      ($6::timestamptz IS NULL AND $7::timestamptz IS NULL AND to_char(occurred_at, 'YYYY-MM') = $4)
    )
)
INSERT INTO guliangiam.metering_aggregates
  (id, organization_id, period, subject, metric, quantity, unit)
SELECT $5, $1, $4, $2, $3, agg.total, agg.unit
FROM agg
ON CONFLICT (organization_id, period, subject, metric) DO UPDATE
  SET quantity     = EXCLUDED.quantity,
      unit         = EXCLUDED.unit,
      calculated_at = now()
RETURNING id, organization_id, period, subject, metric, quantity, unit, calculated_at, created_at
`

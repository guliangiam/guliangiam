package outbox

import (
	"fmt"

	"github.com/guliangiam/guliangiam/internal/infra/dialect"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// PG → MySQL 转换规则：
//   $1..$N → ?（占位符）
//   guliangiam.table → `table`（无 schema 前缀）
//   now() → NOW()（统一）
//   方言敏感片段通过 mysqlshared.Dialect 生成（Upsert / ForUpdateSkipLocked / ...）

var (
	// sqlInsert — 业务事务内写入 outbox 行（非方言敏感）
	sqlInsert = `
INSERT INTO ` + "`outbox`" + ` (
    id, organization_id, aggregate_type, aggregate_id, event_type,
    event_version, payload, headers, status, attempts, max_attempts,
    available_at, created_at, updated_at, revision
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 1)
`

	// sqlLockBatch_Pick — 先 SELECT 哪些行要锁（方言敏感：锁语法经 Dialect 生成）
	sqlLockBatch_Pick = `
SELECT id
FROM ` + "`outbox`" + `
WHERE status = 'pending' AND available_at <= NOW()
ORDER BY created_at ASC
LIMIT ?
` + mysqlshared.Dialect.ForUpdateSkipLocked() + `
`

	// sqlLockBatch_Update — 锁 picked 行（非方言敏感）
	sqlLockBatch_Update = `
UPDATE ` + "`outbox`" + `
SET
    status      = 'processing',
    locked_by   = ?,
    locked_at   = NOW(),
    updated_at  = NOW(),
    revision    = revision + 1
WHERE id IN (__PICKED_IDS__)
`

	// sqlLockBatch_Select — 取回已锁行（非方言敏感）
	sqlLockBatch_Select = `
SELECT id, organization_id, aggregate_type, aggregate_id, event_type,
       event_version, payload, headers, status, attempts, max_attempts,
       available_at, locked_by, locked_at, processed_at, last_error,
       created_at, updated_at, revision
FROM ` + "`outbox`" + `
WHERE id IN (__PICKED_IDS__)
`

	sqlMarkSuccess = `
UPDATE ` + "`outbox`" + `
SET status = 'succeeded',
    processed_at = ?,
    updated_at = NOW(),
    revision = revision + 1
WHERE id = ?
`

	sqlMarkRetry = `
UPDATE ` + "`outbox`" + `
SET
    status       = 'pending',
    attempts     = attempts + 1,
    last_error   = ?,
    available_at = ?,
    updated_at   = NOW(),
    revision     = revision + 1
WHERE id = ?
`

	// sqlMarkDead_Letter — 原子 INSERT dead letter（方言敏感：冲突静默跳过 → Upsert DoNothing）
	// 用 Sprintf 拼装（而非 + 号拼接）以显式区分「静态表名」与「运行时方言片段」，
	// 两者都不来自调用方输入。
	sqlMarkDead_Letter = fmt.Sprintf(`
INSERT INTO %s
    (id, organization_id, aggregate_type, aggregate_id, event_type, event_id, payload, headers,
     status, attempts, last_error, dead_at)
SELECT id, organization_id, aggregate_type, aggregate_id, event_type, id, payload, headers,
       'dead', attempts, ?, ?
FROM %s
WHERE id = ?
%s
`, "`outbox_dead_letters`", "`outbox`",
		mysqlshared.Dialect.Upsert(dialect.DialectUpsert{DoNothing: true}, nil))

	sqlMarkDead_Outbox = `
UPDATE ` + "`outbox`" + `
SET status = 'dead',
    last_error = ?,
    updated_at = NOW(),
    revision = revision + 1
WHERE id = ?
`
)

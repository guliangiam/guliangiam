package outbox

// INSERT — written inside the same business transaction
const sqlInsert = `
INSERT INTO guliangiam.outbox (
    id, organization_id, aggregate_type, aggregate_id, event_type,
    event_version, payload, headers, status, attempts, max_attempts,
    available_at, created_at, updated_at, revision
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11,
    $12, now(), now(), 1
)
`

// LockBatch — lease pickup with FOR UPDATE SKIP LOCKED.
// 除了 pending 且到期的行，还要回收「processing 但租约过期」的行：
// Worker 崩溃/被杀后 locked_at 不再前进，没有租约回收时这些行会永久卡死
// （DEF-32）。$3 是租约 TTL 秒数（double precision，避免 Duration→interval 编码差异）。
const sqlLockBatch = `
WITH picked AS (
    SELECT id
    FROM guliangiam.outbox
    WHERE (status = 'pending' AND available_at <= now())
       OR (status = 'processing'
           AND (locked_at IS NULL OR locked_at < now() - ($3 * interval '1 second')))
    ORDER BY created_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE guliangiam.outbox
SET
    status      = 'processing',
    locked_by   = $2,
    locked_at   = now(),
    updated_at  = now(),
    revision    = revision + 1
WHERE id IN (SELECT id FROM picked)
RETURNING
    id, organization_id, aggregate_type, aggregate_id, event_type,
    event_version, payload, headers, status, attempts, max_attempts,
    available_at, locked_by, locked_at, processed_at, last_error,
    created_at, updated_at, revision
`

const sqlMarkSuccess = `
UPDATE guliangiam.outbox
SET status = 'succeeded',
    processed_at = $2,
    updated_at = now(),
    revision = revision + 1
WHERE id = $1
`

const sqlMarkRetry = `
UPDATE guliangiam.outbox
SET
    status       = 'pending',
    attempts     = attempts + 1,
    last_error   = $2,
    available_at = $3,
    updated_at   = now(),
    revision     = revision + 1
WHERE id = $1
`

const sqlMarkDead_Letter = `
INSERT INTO guliangiam.outbox_dead_letters
    (id, organization_id, aggregate_type, aggregate_id, event_type, event_id, payload, headers,
     status, attempts, last_error, dead_at)
SELECT id, organization_id, aggregate_type, aggregate_id, event_type, id, payload, headers,
       'dead', attempts, $2, $3
FROM guliangiam.outbox
WHERE id = $1
ON CONFLICT (id) DO NOTHING
`

const sqlMarkDead_Outbox = `
UPDATE guliangiam.outbox
SET status = 'dead',
    last_error = $2,
    updated_at = now(),
    revision = revision + 1
WHERE id = $1
`

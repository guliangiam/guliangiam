package lockoutattempts

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::timestamptz / $N::int  → ?
//   "guliangiam".table                    → `table`
//   now()                                 → NOW()
//   ON CONFLICT (user_id) DO UPDATE       → INSERT ... ON DUPLICATE KEY UPDATE
//   RETURNING attempts, locked_until      → MySQL 用 UPSERT + SELECT
//   滑动观察窗及退避状态由单条 UPSERT 原子维护。

const btLockoutAttempts = "`lockout_attempts`"

const queryGetByUserID = `
SELECT id, user_id, attempts, locked_until, window_started_at, lockout_level, created_at, updated_at, revision
FROM ` + btLockoutAttempts + `
WHERE user_id = ?`

const queryCreate = `
INSERT INTO ` + btLockoutAttempts + ` (id, user_id, attempts, locked_until, window_started_at, lockout_level, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

const queryUpdate = `
UPDATE ` + btLockoutAttempts + `
SET attempts = ?, locked_until = ?, window_started_at = ?, lockout_level = ?, updated_at = ?, revision = revision + 1
WHERE user_id = ? AND revision = ?`

const queryEnsureAttempt = `
INSERT IGNORE INTO ` + btLockoutAttempts + `
    (id, user_id, attempts, window_started_at, lockout_level, created_at, updated_at, revision)
VALUES (?, ?, 0, ?, 0, ?, ?, 1)`

const queryLockByUserID = `
SELECT id, user_id, attempts, locked_until, window_started_at, lockout_level, created_at, updated_at, revision
FROM ` + btLockoutAttempts + `
WHERE user_id = ?
FOR UPDATE`

// queryDeleteByUserID: user_id(1)
const queryDeleteByUserID = `
DELETE FROM ` + btLockoutAttempts + `
WHERE user_id = ?`

// queryDeleteExpired: 无参数
const queryDeleteExpired = `
UPDATE ` + btLockoutAttempts + `
SET attempts = 0, locked_until = NULL, window_started_at = NOW(), updated_at = NOW(), revision = revision + 1
WHERE locked_until IS NOT NULL AND locked_until < NOW()`

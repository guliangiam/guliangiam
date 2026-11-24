package lockoutattempts

const (
	queryGetByUserID = `
SELECT id, user_id, attempts, locked_until, window_started_at, lockout_level, created_at, updated_at, revision
FROM "guliangiam".lockout_attempts
WHERE user_id = $1::uuid`

	queryCreate = `
INSERT INTO "guliangiam".lockout_attempts
    (id, user_id, attempts, locked_until, window_started_at, lockout_level, created_at, updated_at, revision)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9)`

	// There is exactly one row per user (uq_lockout_attempts_user_id), so the
	// update is keyed by user_id and the primary key is redundant. The previous
	// form passed the id as $2 but never referenced it, which made PostgreSQL
	// reject the statement with "could not determine data type of parameter $2".
	// revision is incremented to satisfy the manual's update rule (3.5).
	queryUpdate = `
UPDATE "guliangiam".lockout_attempts
SET attempts = $2, locked_until = $3, window_started_at = $4, lockout_level = $5, updated_at = $6, revision = revision + 1
WHERE user_id = $1::uuid AND revision = $7`

	// queryRecordFailure performs the whole failure-counting state machine in one
	// statement so concurrent failed logins cannot lose increments (the previous
	// read-modify-write flow let N parallel requests all write K+1, which is what
	// makes MaxAttempts bypassable).
	//
	// State machine:
	//   - no row                -> attempts = 1, locked when MaxAttempts <= 1
	//   - locked (lock > now)   -> unchanged (no increment, keep the lock)
	//   - lock/window expired   -> start a new window at 1, preserve backoff level
	//   - otherwise             -> increment within the window; lock at threshold
	queryRecordFailure = `
INSERT INTO "guliangiam".lockout_attempts
    (id, user_id, attempts, locked_until, window_started_at, lockout_level, created_at, updated_at)
VALUES (
    $1::uuid,
    $2::uuid,
    1,
    CASE WHEN $3::int <= 1 THEN $4::timestamptz + ($5::int * INTERVAL '1 minute') ELSE NULL END,
    $4::timestamptz,
    CASE WHEN $3::int <= 1 THEN 1 ELSE 0 END,
    $4::timestamptz,
    $4::timestamptz
)
ON CONFLICT (user_id) DO UPDATE
SET attempts = CASE
        WHEN lockout_attempts.locked_until IS NOT NULL AND lockout_attempts.locked_until > $4::timestamptz
            THEN lockout_attempts.attempts
        WHEN lockout_attempts.locked_until IS NOT NULL
          OR lockout_attempts.window_started_at <= $4::timestamptz - INTERVAL '15 minutes'
            THEN 1
        ELSE lockout_attempts.attempts + 1
    END,
    window_started_at = CASE
        WHEN lockout_attempts.locked_until IS NOT NULL AND lockout_attempts.locked_until > $4::timestamptz
            THEN lockout_attempts.window_started_at
        WHEN lockout_attempts.locked_until IS NOT NULL
          OR lockout_attempts.window_started_at <= $4::timestamptz - INTERVAL '15 minutes'
            THEN $4::timestamptz
        ELSE lockout_attempts.window_started_at
    END,
    locked_until = CASE
        WHEN lockout_attempts.locked_until IS NOT NULL AND lockout_attempts.locked_until > $4::timestamptz
            THEN lockout_attempts.locked_until
        WHEN (CASE
                WHEN lockout_attempts.locked_until IS NOT NULL
                  OR lockout_attempts.window_started_at <= $4::timestamptz - INTERVAL '15 minutes'
                    THEN 1
                ELSE lockout_attempts.attempts + 1
              END) >= $3::int
            THEN $4::timestamptz + (CASE lockout_attempts.lockout_level
                WHEN 0 THEN $5::int
                WHEN 1 THEN 360
                ELSE 1440
              END * INTERVAL '1 minute')
        ELSE NULL
    END,
    lockout_level = CASE
        WHEN lockout_attempts.locked_until IS NOT NULL AND lockout_attempts.locked_until > $4::timestamptz
            THEN lockout_attempts.lockout_level
        WHEN (CASE
                WHEN lockout_attempts.locked_until IS NOT NULL
                  OR lockout_attempts.window_started_at <= $4::timestamptz - INTERVAL '15 minutes'
                    THEN 1
                ELSE lockout_attempts.attempts + 1
              END) >= $3::int
            THEN LEAST(lockout_attempts.lockout_level + 1, 3)
        ELSE lockout_attempts.lockout_level
    END,
    updated_at = $4::timestamptz,
    revision = lockout_attempts.revision + 1
RETURNING attempts, locked_until, window_started_at, lockout_level, created_at, updated_at, revision`

	queryDeleteByUserID = `
DELETE FROM "guliangiam".lockout_attempts
WHERE user_id = $1::uuid`

	queryDeleteExpired = `
UPDATE "guliangiam".lockout_attempts
SET attempts = 0, locked_until = NULL, window_started_at = NOW(), updated_at = NOW(), revision = revision + 1
WHERE locked_until IS NOT NULL AND locked_until < NOW()`
)

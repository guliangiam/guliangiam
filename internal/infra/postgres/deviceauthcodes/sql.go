package deviceauthcodes

const (
	queryGetByDeviceCode = `
SELECT id, device_code, user_code, client_id, scope, expires_at, poll_interval_sec,
       verified, verified_at, user_id, username, organization_id,
       last_polled_at, slow_down_count, created_at, updated_at, revision
FROM "guliangiam".device_auth_codes
WHERE device_code = $1`

	queryGetByUserCode = `
SELECT id, device_code, user_code, client_id, scope, expires_at, poll_interval_sec,
       verified, verified_at, user_id, username, organization_id,
       last_polled_at, slow_down_count, created_at, updated_at, revision
FROM "guliangiam".device_auth_codes
WHERE user_code = $1`

	queryCreate = `
INSERT INTO "guliangiam".device_auth_codes
    (id, device_code, user_code, client_id, scope, expires_at, poll_interval_sec,
     verified, verified_at, user_id, username, organization_id,
     last_polled_at, slow_down_count, created_at, updated_at, revision)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`

	queryUpdateVerification = `
UPDATE "guliangiam".device_auth_codes
SET verified = $3, verified_at = $4, user_id = $5, username = $6, organization_id = $7,
    updated_at = $8, revision = revision + 1
WHERE device_code = $1 AND revision = $2`

	queryDeleteByDeviceCode = `
DELETE FROM "guliangiam".device_auth_codes
WHERE device_code = $1`

	queryDeleteByUserCode = `
DELETE FROM "guliangiam".device_auth_codes
WHERE user_code = $1`

	// queryRecordPoll implements the RFC 8628 §3.5 throttle in one statement.
	// A read-then-write would let N concurrent polls all read the same counter
	// and each write counter+1, so a flood would never reach the brute-force
	// cap. RETURNING hands back the *new* count so the caller can choose
	// slow_down versus terminate without a second round trip.
	queryRecordPoll = `
UPDATE "guliangiam".device_auth_codes
SET last_polled_at = $2,
    slow_down_count = slow_down_count + 1,
    updated_at = $2,
    revision = revision + 1
WHERE device_code = $1
RETURNING slow_down_count`

	// queryResetSlowDown records the poll and clears the counter in one write.
	queryResetSlowDown = `
UPDATE "guliangiam".device_auth_codes
SET last_polled_at = $2,
    slow_down_count = 0,
    updated_at = $2,
    revision = revision + 1
WHERE device_code = $1`

	queryDeleteExpired = `
DELETE FROM "guliangiam".device_auth_codes
WHERE expires_at < NOW()`
)

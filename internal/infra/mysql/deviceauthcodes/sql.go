package deviceauthcodes

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::timestamptz  → ?
//   "guliangiam".table          → `table`
//   now()                       → NOW()
//   UPDATE ... RETURNING col    → MySQL 用 UPDATE + SELECT 两步

const btDeviceAuthCodes = "`device_auth_codes`"

// deviceAuthCodeSelectBase: 17 列
const deviceAuthCodeSelectBase = `
SELECT id, device_code, user_code, client_id, scope, expires_at, poll_interval_sec,
       verified, verified_at, user_id, username, organization_id,
       last_polled_at, slow_down_count, created_at, updated_at, revision
FROM ` + btDeviceAuthCodes

// queryGetByDeviceCode: device_code(1)
const queryGetByDeviceCode = deviceAuthCodeSelectBase + `
WHERE device_code = ?`

// queryGetByUserCode: user_code(1)
const queryGetByUserCode = deviceAuthCodeSelectBase + `
WHERE user_code = ?`

// queryCreate: 17 个占位符
const queryCreate = `
INSERT INTO ` + btDeviceAuthCodes + `
    (id, device_code, user_code, client_id, scope, expires_at, poll_interval_sec,
     verified, verified_at, user_id, username, organization_id,
     last_polled_at, slow_down_count, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateVerification: 乐观锁更新
// 参数：verified, verified_at, user_id, username, organization_id, updated_at,
//
//	device_code, old_revision
const queryUpdateVerification = `
UPDATE ` + btDeviceAuthCodes + `
SET verified = ?, verified_at = ?, user_id = ?, username = ?, organization_id = ?,
    updated_at = ?, revision = revision + 1
WHERE device_code = ? AND revision = ?`

// queryDeleteByDeviceCode: device_code(1)
const queryDeleteByDeviceCode = `
DELETE FROM ` + btDeviceAuthCodes + ` WHERE device_code = ?`

// queryDeleteByUserCode: user_code(1)
const queryDeleteByUserCode = `
DELETE FROM ` + btDeviceAuthCodes + ` WHERE user_code = ?`

// queryRecordPoll: PG RETURNING slow_down_count 改写 — MySQL UPDATE 不支持 RETURNING
// 参数：device_code, polled_at(×3)
//
//	SET last_polled_at = ?, slow_down_count = slow_down_count + 1, updated_at = ?, revision = revision + 1
//	WHERE device_code = ?
const queryRecordPoll = `
UPDATE ` + btDeviceAuthCodes + `
SET last_polled_at = ?, slow_down_count = slow_down_count + 1,
    updated_at = ?, revision = revision + 1
WHERE device_code = ?`

// querySelectSlowDownAfterPoll: UPDATE 之后拿新 counter
const querySelectSlowDownAfterPoll = `
SELECT slow_down_count FROM ` + btDeviceAuthCodes + ` WHERE device_code = ?`

// queryResetSlowDown: 直接 EXEC
const queryResetSlowDown = `
UPDATE ` + btDeviceAuthCodes + `
SET last_polled_at = ?, slow_down_count = 0, updated_at = ?, revision = revision + 1
WHERE device_code = ?`

// queryDeleteExpired: 无参数
const queryDeleteExpired = `
DELETE FROM ` + btDeviceAuthCodes + ` WHERE expires_at < NOW()`

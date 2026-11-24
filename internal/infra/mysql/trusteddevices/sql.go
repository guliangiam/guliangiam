package trusteddevices

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text            → ?
//   "guliangiam".table             → `table`
//   pgx.Query/QueryRow/Exec        → QueryContext/QueryRowContext/ExecContext
//   nullable TIMESTAMP             → *time.Time（database/sql 支持 **time.Time 扫描）

const btTrustedDevices = "`trusted_devices`"

// queryInsertDevice 占位符：12
const queryInsertDevice = `INSERT INTO ` + btTrustedDevices + ` (id, organization_id, user_id, device_credential_id, device_public_key, ua_hash, created_at, last_seen_at, expires_at, revoked_at, revision, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryGetByID 占位符：id(1)
const queryGetByID = `SELECT id, organization_id, user_id, device_credential_id, device_public_key, ua_hash, created_at, last_seen_at, expires_at, revoked_at, revision, updated_at FROM ` + btTrustedDevices + ` WHERE id = ?`

// queryListByUser 占位符：organization_id(1), user_id(2)
const queryListByUser = `SELECT id, organization_id, user_id, device_credential_id, device_public_key, ua_hash, created_at, last_seen_at, expires_at, revoked_at, revision, updated_at FROM ` + btTrustedDevices + ` WHERE organization_id = ? AND user_id = ? ORDER BY created_at DESC`

// queryFindActiveByCredential 占位符：organization_id(1), user_id(2), device_credential_id(3), ua_hash(4), now(5)
const queryFindActiveByCredential = `SELECT id, organization_id, user_id, device_credential_id, device_public_key, ua_hash, created_at, last_seen_at, expires_at, revoked_at, revision, updated_at FROM ` + btTrustedDevices + ` WHERE organization_id = ? AND user_id = ? AND device_credential_id = ? AND ua_hash = ? AND revoked_at IS NULL AND expires_at > ? ORDER BY created_at DESC LIMIT 1`

// queryRevoke 占位符：revoked_at/updated_at(2), organization_id(3), id(4)
// 幂等撤销：只有 revoked_at IS NULL 的行会被写入；重复撤销 RowsAffected=0 但不是错误。
const queryRevoke = `UPDATE ` + btTrustedDevices + ` SET revoked_at = ?, updated_at = ?, revision = revision + 1 WHERE organization_id = ? AND id = ? AND revoked_at IS NULL`

// queryTouchLastSeen 占位符：last_seen_at/updated_at(2), id(3)
const queryTouchLastSeen = `UPDATE ` + btTrustedDevices + ` SET last_seen_at = ?, updated_at = ?, revision = revision + 1 WHERE id = ?`

// queryRevokeByCredential 占位符：revoked_at/updated_at(2), organization_id(3), user_id(4), device_credential_id(5)
const queryRevokeByCredential = `UPDATE ` + btTrustedDevices + ` SET revoked_at = ?, updated_at = ?, revision = revision + 1 WHERE organization_id = ? AND user_id = ? AND device_credential_id = ? AND revoked_at IS NULL`

// queryRevokeByUser 占位符：revoked_at/updated_at(2), organization_id(3), user_id(4)
// 凭据级联：批量吊销某用户在当前组织下所有仍有效的受信设备。
const queryRevokeByUser = `UPDATE ` + btTrustedDevices + ` SET revoked_at = ?, updated_at = ?, revision = revision + 1 WHERE organization_id = ? AND user_id = ? AND revoked_at IS NULL`

// queryDeleteExpired 占位符：now(1), limit(2)
// MySQL 不允许 DELETE 的同表子查询直接引用目标表，包一层派生表即可。
// 用反引号 SQL 片段 + 编译期表名常量拼接，便于 SQL 审计器静态识别（DEF-08）。
const queryDeleteExpired = `DELETE FROM ` + btTrustedDevices + ` WHERE id IN (SELECT id FROM (SELECT id FROM ` + btTrustedDevices + ` WHERE expires_at < ? ORDER BY expires_at LIMIT ?) AS expired_devices)`

// queryRevokeLegacyPlaintext 占位符：now(1)
const queryRevokeLegacyPlaintext = `UPDATE ` + btTrustedDevices + ` SET revoked_at = ?, updated_at = ?, revision = revision + 1` + ` WHERE revoked_at IS NULL AND device_credential_id NOT REGEXP '^[0-9a-f]{64}$'`

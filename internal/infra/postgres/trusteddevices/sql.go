package trusteddevices

const (
	queryInsertDevice = `
INSERT INTO "guliangiam".trusted_devices
  (id, organization_id, user_id, device_credential_id, device_public_key, ua_hash,
   created_at, last_seen_at, expires_at, revoked_at, revision, updated_at)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	queryGetByID = `
SELECT id, COALESCE(organization_id::text, ''), COALESCE(user_id::text, ''),
       device_credential_id, device_public_key, ua_hash, created_at, last_seen_at,
       expires_at, revoked_at, revision, updated_at
FROM guliangiam.trusted_devices
WHERE id = $1::uuid`

	queryListByUser = `
SELECT id, COALESCE(organization_id::text, ''), COALESCE(user_id::text, ''),
       device_credential_id, device_public_key, ua_hash, created_at, last_seen_at,
       expires_at, revoked_at, revision, updated_at
FROM guliangiam.trusted_devices
WHERE organization_id = $1::uuid
  AND user_id = $2::uuid
ORDER BY created_at DESC`

	queryFindActiveByCredential = `
SELECT id, COALESCE(organization_id::text, ''), COALESCE(user_id::text, ''),
       device_credential_id, device_public_key, ua_hash, created_at, last_seen_at,
       expires_at, revoked_at, revision, updated_at
FROM guliangiam.trusted_devices
WHERE organization_id = $1::uuid
  AND user_id = $2::uuid
  AND device_credential_id = $3
  AND ua_hash = $4
  AND revoked_at IS NULL
  AND expires_at > $5
ORDER BY created_at DESC
LIMIT 1`

	// 幂等撤销：只有 revoked_at IS NULL 的行会被写入；重复撤销 RowsAffected=0 但不是错误。
	queryRevoke = `
UPDATE "guliangiam".trusted_devices
SET revoked_at = $3, updated_at = $3, revision = revision + 1
WHERE organization_id = $1::uuid
  AND id = $2::uuid
  AND revoked_at IS NULL`

	queryTouchLastSeen = `
UPDATE "guliangiam".trusted_devices
SET last_seen_at = $2, updated_at = $2, revision = revision + 1
WHERE id = $1::uuid`

	// 注册新设备前吊销同一凭据的旧行，避免同一设备出现多条活跃信任记录。
	queryRevokeByCredential = `
UPDATE "guliangiam".trusted_devices
SET revoked_at = $4, updated_at = $4, revision = revision + 1
WHERE organization_id = $1::uuid
  AND user_id = $2::uuid
  AND device_credential_id = $3
  AND revoked_at IS NULL`

	// 凭据级联：批量吊销某用户在当前组织下所有仍有效的受信设备。
	queryRevokeByUser = `
UPDATE "guliangiam".trusted_devices
SET revoked_at = $3, updated_at = $3, revision = revision + 1
WHERE organization_id = $1::uuid
  AND user_id = $2::uuid
  AND revoked_at IS NULL`
)

// queryDeleteExpired 占位符：now(1), limit(2)
// 分批物理删除过期受信设备（不区分是否已撤销；过期即不可用）。
const queryDeleteExpired = "DELETE FROM guliangiam.trusted_devices " +
	"WHERE id IN (" +
	"SELECT id FROM guliangiam.trusted_devices " +
	"WHERE expires_at < $1 ORDER BY expires_at LIMIT $2)"

// queryRevokeLegacyPlaintext 占位符：now(1)
// 存量明文凭据行（非 64 位小写 hex 摘要）显式撤销，使客户端回退 MFA 并重新注册。
const queryRevokeLegacyPlaintext = "UPDATE guliangiam.trusted_devices " +
	"SET revoked_at = $1, updated_at = $1, revision = revision + 1 " +
	"WHERE revoked_at IS NULL AND device_credential_id !~ '^[0-9a-f]{64}$'"

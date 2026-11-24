package rememberme

const (
	queryInsertFamily = `
INSERT INTO "guliangiam".remember_me_families
  (id, organization_id, user_id, application_id, expires_at, revoked_at, revoke_reason,
   created_at, updated_at, revision)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8, $9, $10)`

	queryGetFamily = `
SELECT id, organization_id, user_id, application_id, expires_at, revoked_at, revoke_reason,
       created_at, updated_at, revision
FROM guliangiam.remember_me_families
WHERE id = $1::uuid`

	// 幂等撤销：只有 revoked_at IS NULL 的行会被写入；重复撤销 RowsAffected=0 但不是错误。
	queryRevokeFamily = `
UPDATE "guliangiam".remember_me_families
SET revoked_at = $2, revoke_reason = $3, updated_at = $2, revision = revision + 1
WHERE id = $1::uuid
  AND revoked_at IS NULL`

	queryRevokeFamiliesForUser = `
UPDATE "guliangiam".remember_me_families
SET revoked_at = $3, revoke_reason = $4, updated_at = $3, revision = revision + 1
WHERE organization_id = $1::uuid
  AND user_id = $2::uuid
  AND revoked_at IS NULL`

	queryInsertToken = `
INSERT INTO "guliangiam".remember_me_tokens
  (id, family_id, organization_id, token_hash, token_prefix, consumed_at, expires_at,
   created_at, updated_at, revision)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10)`

	// 匿名 exchange 按哈希唯一查询；token_hash 自带 UNIQUE 约束，无需租户过滤。
	queryGetTokenByHash = `
SELECT id, family_id, organization_id, token_hash, token_prefix, consumed_at,
       expires_at, created_at, updated_at, revision
FROM guliangiam.remember_me_tokens
WHERE token_hash = $1`

	// 单次消费：条件更新；已消费/不存在时 RowsAffected=0。
	queryConsumeToken = `
UPDATE "guliangiam".remember_me_tokens
SET consumed_at = $2, updated_at = $2, revision = revision + 1
WHERE id = $1::uuid
  AND consumed_at IS NULL`

	// 家族撤销时把尚未消费的 token 一并作废（consumed_at 充当失效标记；该表无 revoked_at）。
	queryRevokeTokensByFamily = `
UPDATE "guliangiam".remember_me_tokens
SET consumed_at = COALESCE(consumed_at, $2), updated_at = $2, revision = revision + 1
WHERE family_id = $1::uuid`
)

// queryDeleteExpiredTokens 占位符：now(1), limit(2)
// 分批物理删除过期令牌；只删除 expires_at 已过的行，不触碰仍可用的轮换令牌。
const queryDeleteExpiredTokens = "DELETE FROM guliangiam.remember_me_tokens " +
	"WHERE id IN (" +
	"SELECT id FROM guliangiam.remember_me_tokens " +
	"WHERE expires_at < $1 ORDER BY expires_at LIMIT $2)"

// queryDeleteExpiredFamilies 占位符：now(1), limit(2)
// 家族删除会级联删除其令牌（family_id ON DELETE CASCADE）。
const queryDeleteExpiredFamilies = "DELETE FROM guliangiam.remember_me_families " +
	"WHERE id IN (" +
	"SELECT id FROM guliangiam.remember_me_families " +
	"WHERE expires_at < $1 ORDER BY expires_at LIMIT $2)"

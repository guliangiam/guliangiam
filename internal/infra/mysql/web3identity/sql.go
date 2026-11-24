package web3identity

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::timestamptz → ?
//   "guliangiam".table         → `table`
//   now()                      → NOW()
//   no schema prefix (MySQL uses database)

const btWeb3IdentityMappings = "`web3_identity_mappings`"

// queryCreate: 11 个占位符（created_at, updated_at, revision 由 DB 默认值填充）
const queryCreate = `
INSERT INTO ` + btWeb3IdentityMappings + `
	(id, user_id, credential_id, wallet_address,
	 derivation_method, derivation_seed, chain_ids, ens_name, is_primary,
	 verified_at, last_used_at)
VALUES
	(?, ?, ?, ?,
	 ?, ?, ?, ?, ?,
	 ?, ?)`

// queryByCredentialID: 1 个占位符
const queryByCredentialID = `
SELECT id, user_id, credential_id, wallet_address,
       derivation_method, derivation_seed, chain_ids, ens_name, is_primary,
       verified_at, last_used_at, created_at, updated_at, revision
  FROM ` + btWeb3IdentityMappings + `
 WHERE credential_id = ?
 LIMIT 1`

// queryByWalletAddress: 1 个占位符
const queryByWalletAddress = `
SELECT id, user_id, credential_id, wallet_address,
       derivation_method, derivation_seed, chain_ids, ens_name, is_primary,
       verified_at, last_used_at, created_at, updated_at, revision
  FROM ` + btWeb3IdentityMappings + `
 WHERE wallet_address = ?
 LIMIT 1`

// queryTouchLastUsed: 2 个占位符 — MySQL 先 SET updated_at，再 WHERE
const queryTouchLastUsed = `
UPDATE ` + btWeb3IdentityMappings + `
   SET last_used_at = ?, updated_at = NOW()
 WHERE id = ?`

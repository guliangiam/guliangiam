package web3identity

// SQL statements for the PostgreSQL web3_identity_mappings repository.

const (
	queryCreate = `
INSERT INTO guliangiam.web3_identity_mappings
	(id, user_id, credential_id, wallet_address,
	 derivation_method, derivation_seed, chain_ids, ens_name, is_primary,
	 verified_at, last_used_at, created_at, updated_at, revision)
VALUES
	($1, $2, $3, $4,
	 $5, $6, $7, $8, $9,
	 $10, $11, now(), now(), 1)`

	queryByCredentialID = `
SELECT id, user_id, credential_id, wallet_address,
       derivation_method, derivation_seed, chain_ids, ens_name, is_primary,
       verified_at, last_used_at, created_at, updated_at, revision
  FROM guliangiam.web3_identity_mappings
 WHERE credential_id = $1
 LIMIT 1`

	queryByWalletAddress = `
SELECT id, user_id, credential_id, wallet_address,
       derivation_method, derivation_seed, chain_ids, ens_name, is_primary,
       verified_at, last_used_at, created_at, updated_at, revision
  FROM guliangiam.web3_identity_mappings
 WHERE wallet_address = $1
 LIMIT 1`

	queryTouchLastUsed = `
UPDATE guliangiam.web3_identity_mappings
   SET last_used_at = $2, updated_at = now()
 WHERE id = $1`
)

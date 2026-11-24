package workload

const qListPools = `SELECT id, organization_id, name, display_name, COALESCE(description, '') as description, created_at, updated_at, revision FROM guliangiam.identity_pools WHERE organization_id = $1 ORDER BY name DESC`
const qGetPool = `SELECT id, organization_id, name, display_name, COALESCE(description, ''), created_at, updated_at, revision FROM guliangiam.identity_pools WHERE id = $1 AND organization_id = $2`
const qInsertPool = `INSERT INTO guliangiam.identity_pools (id, organization_id, name, display_name, description) VALUES ($1, $2, $3, $4, $5)`

const qListTrusts = `SELECT id, organization_id, pool_id, name, issuer, COALESCE(jwks_uri, ''), audience, subjects, status, created_at, updated_at, revision FROM guliangiam.federation_trusts WHERE ($1 = '' OR organization_id = NULLIF($1, '')::uuid) AND ($2 = '' OR pool_id = NULLIF($2, '')::uuid) ORDER BY name DESC`
const qCountTrusts = `SELECT count(*) FROM guliangiam.federation_trusts WHERE ($1 = '' OR organization_id = NULLIF($1, '')::uuid) AND ($2 = '' OR pool_id = NULLIF($2, '')::uuid)`
const qGetTrust = `SELECT id, organization_id, pool_id, name, issuer, COALESCE(jwks_uri, ''), audience, subjects, status, created_at, updated_at, revision FROM guliangiam.federation_trusts WHERE id = $1 AND organization_id = $2`
const qFindActiveTrustsByIssuer = `SELECT id, organization_id, pool_id, name, issuer, COALESCE(jwks_uri, ''), audience, subjects, status, created_at, updated_at, revision FROM guliangiam.federation_trusts WHERE issuer = $1 AND status = 'active' ORDER BY name DESC`
const qInsertTrust = `INSERT INTO guliangiam.federation_trusts (id, organization_id, pool_id, name, issuer, jwks_uri, audience, subjects, status) VALUES ($1, $2, $3, $4, $5, $6, $7::text[], $8::text[], 'active')`

const qListAccounts = `SELECT id, organization_id, COALESCE(pool_id::text, ''), name, COALESCE(description, ''), client_id, COALESCE(client_secret_hash, ''), scopes, access_token_ttl_seconds, last_rotated_at, status, created_at, updated_at, revision FROM guliangiam.service_accounts WHERE organization_id = $1 AND pool_id = $2 ORDER BY name DESC`
const qCountAccounts = `SELECT count(*) FROM guliangiam.service_accounts WHERE organization_id = $1 AND pool_id = $2`
const qGetAccount = `SELECT id, organization_id, COALESCE(pool_id::text, ''), name, COALESCE(description, ''), client_id, COALESCE(client_secret_hash, ''), scopes, access_token_ttl_seconds, last_rotated_at, status, created_at, updated_at, revision FROM guliangiam.service_accounts WHERE id = $1 AND organization_id = $2`
const qGetAccountByClientID = `SELECT id, organization_id, COALESCE(pool_id::text, ''), name, COALESCE(description, ''), client_id, COALESCE(client_secret_hash, ''), scopes, access_token_ttl_seconds, last_rotated_at, status, created_at, updated_at, revision FROM guliangiam.service_accounts WHERE client_id = $1`
const qGetAccountByName = `SELECT id, organization_id, COALESCE(pool_id::text, ''), name, COALESCE(description, ''), client_id, COALESCE(client_secret_hash, ''), scopes, access_token_ttl_seconds, last_rotated_at, status, created_at, updated_at, revision FROM guliangiam.service_accounts WHERE organization_id = $1 AND name = $2`
const qInsertAccount = `INSERT INTO guliangiam.service_accounts (id, organization_id, pool_id, name, description, client_id, client_secret_hash, scopes, access_token_ttl_seconds) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::text[], $9)`

const qUpdateTrustStatus = `UPDATE guliangiam.federation_trusts
	SET status = $1, updated_at = NOW()
	WHERE id = $2 AND organization_id = $3 AND status <> 'deleted'`
const qDeleteTrust = `UPDATE guliangiam.federation_trusts
	SET status = 'deleted', updated_at = NOW()
	WHERE id = $1 AND organization_id = $2 AND status <> 'deleted'`
const qUpdateAccountStatus = `UPDATE guliangiam.service_accounts
	SET status = $1, updated_at = NOW(), revision = revision + 1
	WHERE id = $2 AND organization_id = $3 AND revision = $4 AND status <> 'deleted'`
const qRotateAccountSecret = `UPDATE guliangiam.service_accounts
	SET client_secret_hash = $1, last_rotated_at = $2, updated_at = NOW(), revision = revision + 1
	WHERE id = $3 AND organization_id = $4 AND revision = $5 AND status <> 'deleted'`
const qDeleteAccount = `UPDATE guliangiam.service_accounts
	SET status = 'deleted', updated_at = NOW(), revision = revision + 1
	WHERE id = $1 AND organization_id = $2 AND revision = $3 AND status = 'disabled'`

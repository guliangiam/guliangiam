package workload

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text         → ?
//   "guliangiam".table         → `table`（去掉 schema 前缀）
//   $N::text[]                 → ?（TEXT，存 JSON 数组字符串）
//   COALESCE(x, '')            → 保留
//   LIMIT/OFFSET               → 参数化 LIMIT ? OFFSET ?

const btPools = "`identity_pools`"
const btTrusts = "`federation_trusts`"
const btAccounts = "`service_accounts`"

const (
	qListPools = `
	SELECT id, organization_id, name, display_name, COALESCE(description, ''), created_at, updated_at, revision
	FROM ` + btPools + ` WHERE organization_id = ? ORDER BY name DESC
	LIMIT ? OFFSET ?`

	qCountPools = `SELECT COUNT(*) FROM ` + btPools + ` WHERE organization_id = ?`

	qGetPool = `
	SELECT id, organization_id, name, display_name, COALESCE(description, ''), created_at, updated_at, revision
	FROM ` + btPools + ` WHERE id = ? AND organization_id = ?`

	qInsertPool = `
	INSERT INTO ` + btPools + ` (id, organization_id, name, display_name, description)
	VALUES (?, ?, ?, ?, ?)`
)

const (
	qListTrusts = `
	SELECT id, organization_id, pool_id, name, issuer, COALESCE(jwks_uri, ''), audience, subjects, status, created_at, updated_at, revision
	FROM ` + btTrusts + ` WHERE (? = '' OR organization_id = NULLIF(?, '')) AND (? = '' OR pool_id = NULLIF(?, '')) ORDER BY name DESC
	LIMIT ? OFFSET ?`
	qCountTrusts = `SELECT COUNT(*) FROM ` + btTrusts + ` WHERE (? = '' OR organization_id = NULLIF(?, '')) AND (? = '' OR pool_id = NULLIF(?, ''))`

	qGetTrust = `
	SELECT id, organization_id, pool_id, name, issuer, COALESCE(jwks_uri, ''), audience, subjects, status, created_at, updated_at, revision
	FROM ` + btTrusts + ` WHERE id = ? AND organization_id = ?`

	qFindActiveTrustsByIssuer = `
	SELECT id, organization_id, pool_id, name, issuer, COALESCE(jwks_uri, ''), audience, subjects, status, created_at, updated_at, revision
	FROM ` + btTrusts + ` WHERE issuer = ? AND status = 'active' ORDER BY name DESC`

	qInsertTrust = `
	INSERT INTO ` + btTrusts + ` (id, organization_id, pool_id, name, issuer, jwks_uri, audience, subjects, status)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active')`
)

const (
	qListAccounts = `
	SELECT id, organization_id, COALESCE(pool_id, ''), name, COALESCE(description, ''), client_id, COALESCE(client_secret_hash, ''), scopes, access_token_ttl_seconds, last_rotated_at, status, created_at, updated_at, revision
	FROM ` + btAccounts + ` WHERE organization_id = ? AND pool_id = ? ORDER BY name DESC
	LIMIT ? OFFSET ?`
	qCountAccounts = `SELECT COUNT(*) FROM ` + btAccounts + ` WHERE organization_id = ? AND pool_id = ?`

	qGetAccount = `
	SELECT id, organization_id, COALESCE(pool_id, ''), name, COALESCE(description, ''), client_id, COALESCE(client_secret_hash, ''), scopes, access_token_ttl_seconds, last_rotated_at, status, created_at, updated_at, revision
	FROM ` + btAccounts + ` WHERE id = ? AND organization_id = ?`
	qGetAccountByClientID = `
	SELECT id, organization_id, COALESCE(pool_id, ''), name, COALESCE(description, ''), client_id, COALESCE(client_secret_hash, ''), scopes, access_token_ttl_seconds, last_rotated_at, status, created_at, updated_at, revision
	FROM ` + btAccounts + ` WHERE client_id = ?`
	qGetAccountByName = `
	SELECT id, organization_id, COALESCE(pool_id, ''), name, COALESCE(description, ''), client_id, COALESCE(client_secret_hash, ''), scopes, access_token_ttl_seconds, last_rotated_at, status, created_at, updated_at, revision
	FROM ` + btAccounts + ` WHERE organization_id = ? AND name = ?`

	qInsertAccount = `
	INSERT INTO ` + btAccounts + ` (id, organization_id, pool_id, name, description, client_id, client_secret_hash, scopes, access_token_ttl_seconds)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

const qUpdateTrustStatus = `UPDATE ` + btTrusts + `
	SET status = ?, updated_at = NOW()
	WHERE id = ? AND organization_id = ? AND status <> 'deleted'`
const qDeleteTrust = `UPDATE ` + btTrusts + `
	SET status = 'deleted', updated_at = NOW()
	WHERE id = ? AND organization_id = ? AND status <> 'deleted'`
const qUpdateAccountStatus = `UPDATE ` + btAccounts + `
	SET status = ?, updated_at = NOW(), revision = revision + 1
	WHERE id = ? AND organization_id = ? AND revision = ? AND status <> 'deleted'`
const qRotateAccountSecret = `UPDATE ` + btAccounts + `
	SET client_secret_hash = ?, last_rotated_at = ?, updated_at = NOW(), revision = revision + 1
	WHERE id = ? AND organization_id = ? AND revision = ? AND status <> 'deleted'`
const qDeleteAccount = `UPDATE ` + btAccounts + `
	SET status = 'deleted', updated_at = NOW(), revision = revision + 1
	WHERE id = ? AND organization_id = ? AND revision = ? AND status = 'disabled'`

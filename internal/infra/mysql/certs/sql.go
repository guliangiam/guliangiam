package certs

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::jsonb              → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   RETURNING revision                 → MySQL 用 UPDATE + SELECT revision 两步
//   pgx.Query/QueryRow/Exec            → QueryContext/QueryRowContext/ExecContext

const btCerts = "`certs`"

// queryGetCertByID 占位符：id(1)、organization_id(2)
const queryGetCertByID = `
SELECT id, organization_id, name, display_name, scope, type, crypto_algorithm, bit_size, expire_in_years,
       expire_time, certificate, private_key,
       COALESCE(authority_public_key, '') AS authority_public_key,
       COALESCE(authority_root_public_key, '') AS authority_root_public_key,
       COALESCE(acme_issuer, '') AS acme_issuer,
       COALESCE(acme_endpoint, '') AS acme_endpoint,
       provider, account, access_key, access_secret, domain,
       domain_expire_time,
       created_at, updated_at, revision
FROM ` + btCerts + `
WHERE id = ? AND organization_id = ?`

// queryListCerts 占位符计数（PG $1=$org, $2=$kw, $3=$scope, $4=$type → MySQL 参数重复展开）：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	keyword: ?='' OR name LIKE CONCAT('%',?, '%')          → 3
//	         OR display_name LIKE CONCAT('%',?, '%')       → （已包含在上面的第二个 CONCAT 里）
//	scope: ?='' OR scope=?                                  → 2
//	type: ?='' OR type=?                                    → 2
//	limit, offset                                           → 2
//	总计：11
const queryListCerts = `
SELECT id, organization_id, name, display_name, scope, type, crypto_algorithm, bit_size, expire_in_years,
       expire_time, certificate, private_key,
       COALESCE(authority_public_key, '') AS authority_public_key,
       COALESCE(authority_root_public_key, '') AS authority_root_public_key,
       COALESCE(acme_issuer, '') AS acme_issuer,
       COALESCE(acme_endpoint, '') AS acme_endpoint,
       provider, account, access_key, access_secret, domain,
       domain_expire_time,
       created_at, updated_at, revision
FROM ` + btCerts + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR scope = ?)
  AND (? = '' OR type = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountCerts: org_id×2 + keyword×3 + scope×2 + type×2 = 9
const queryCountCerts = `
SELECT COUNT(*)
FROM ` + btCerts + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR scope = ?)
  AND (? = '' OR type = ?)`

const queryInsertCert = `
INSERT INTO ` + btCerts + ` (id, organization_id, name, display_name, scope, type, crypto_algorithm, bit_size, expire_in_years, expire_time, certificate, private_key, authority_public_key, authority_root_public_key, acme_issuer, acme_endpoint, provider, account, access_key, access_secret, domain, domain_expire_time, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateCert 占位符：display_name(1), scope(1), type(1), crypto_algorithm(1), bit_size(1), expire_in_years(1),
// expire_time(1), certificate(1), private_key(1), authority_public_key(1), authority_root_public_key(1),
// acme_issuer(1), acme_endpoint(1), provider(1), account(1), access_key(1), access_secret(1),
// domain(1), domain_expire_time(1), updated_at(1), id(1), old_revision(1), organization_id(1) → 23
// WHERE 不匹配返回零 affected rows → 调用方判断为 revision 冲突
// old_revision 由调用方传 Revision-1：app 层在 Update 前已 Revision++。
const queryUpdateCert = `
UPDATE ` + btCerts + `
SET display_name = ?,
    scope = ?,
    type = ?,
    crypto_algorithm = ?,
    bit_size = ?,
    expire_in_years = ?,
    expire_time = ?,
    certificate = ?,
    private_key = ?,
    authority_public_key = ?,
    authority_root_public_key = ?,
    acme_issuer = ?,
    acme_endpoint = ?,
    provider = ?,
    account = ?,
    access_key = ?,
    access_secret = ?,
    domain = ?,
    domain_expire_time = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ? AND organization_id = ?`

// querySelectCertRevision: UPDATE 后取新 revision
const querySelectCertRevision = `SELECT revision FROM ` + btCerts + ` WHERE id = ? AND organization_id = ?`

// queryDeleteCert 占位符：id(1)、organization_id(2)
const queryDeleteCert = `
DELETE FROM ` + btCerts + `
WHERE id = ? AND organization_id = ?`

// queryListExpiringCerts 是系统作用域的跨组织扫描：now(1), deadline(2), limit(3)。
const queryListExpiringCerts = `
SELECT id, organization_id, name, display_name, scope, type, crypto_algorithm, bit_size, expire_in_years,
       expire_time, certificate, private_key,
       COALESCE(authority_public_key, '') AS authority_public_key,
       COALESCE(authority_root_public_key, '') AS authority_root_public_key,
       COALESCE(acme_issuer, '') AS acme_issuer,
       COALESCE(acme_endpoint, '') AS acme_endpoint,
       provider, account, access_key, access_secret, domain,
       domain_expire_time,
       created_at, updated_at, revision
FROM ` + btCerts + `
WHERE expire_time IS NOT NULL
  AND expire_time > ?
  AND expire_time <= ?
ORDER BY expire_time ASC, id ASC
LIMIT ?`

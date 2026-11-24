package certs

const (
	queryGetCertByID = `
SELECT id, organization_id, name, display_name, scope, type, crypto_algorithm, bit_size, expire_in_years,
       expire_time, certificate, private_key,
       COALESCE(authority_public_key, '') AS authority_public_key,
       COALESCE(authority_root_public_key, '') AS authority_root_public_key,
       COALESCE(acme_issuer, '') AS acme_issuer,
       COALESCE(acme_endpoint, '') AS acme_endpoint,
       provider, account, access_key, access_secret, domain,
       domain_expire_time,
       created_at, updated_at, revision
FROM "guliangiam".certs
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListCerts = `
SELECT id, organization_id, name, display_name, scope, type, crypto_algorithm, bit_size, expire_in_years,
       expire_time, certificate, private_key,
       COALESCE(authority_public_key, '') AS authority_public_key,
       COALESCE(authority_root_public_key, '') AS authority_root_public_key,
       COALESCE(acme_issuer, '') AS acme_issuer,
       COALESCE(acme_endpoint, '') AS acme_endpoint,
       provider, account, access_key, access_secret, domain,
       domain_expire_time,
       created_at, updated_at, revision
FROM "guliangiam".certs
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR scope = $3)
  AND ($4 = '' OR type = $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6`

	queryCountCerts = `
SELECT COUNT(*)
FROM "guliangiam".certs
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR scope = $3)
  AND ($4 = '' OR type = $4)`

	queryInsertCert = `
INSERT INTO "guliangiam".certs (id, organization_id, name, display_name, scope, type, crypto_algorithm, bit_size, expire_in_years, expire_time, certificate, private_key, authority_public_key, authority_root_public_key, acme_issuer, acme_endpoint, provider, account, access_key, access_secret, domain, domain_expire_time, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21::jsonb, $22, $23, $24, $25)`

	queryUpdateCert = `
UPDATE "guliangiam".certs
SET display_name = $1,
    scope = $2,
    type = $3,
    crypto_algorithm = $4,
    bit_size = $5,
    expire_in_years = $6,
    expire_time = $7,
    certificate = $8,
    private_key = $9,
    authority_public_key = $10,
    authority_root_public_key = $11,
    acme_issuer = $12,
    acme_endpoint = $13,
    provider = $14,
    account = $15,
    access_key = $16,
    access_secret = $17,
    domain = $18::jsonb,
    domain_expire_time = $19,
    updated_at = $20,
    revision = revision + 1
WHERE id = $22::uuid AND revision = $21 AND organization_id = guliangiam.resolve_org_id($23) RETURNING revision`

	queryDeleteCert = `
DELETE FROM "guliangiam".certs
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	// queryListExpiringCerts 是系统作用域的跨组织扫描：只取到期时间落在提醒
	// 窗口内（now, deadline] 的证书，按最近到期排序。
	queryListExpiringCerts = `
SELECT id, organization_id, name, display_name, scope, type, crypto_algorithm, bit_size, expire_in_years,
       expire_time, certificate, private_key,
       COALESCE(authority_public_key, '') AS authority_public_key,
       COALESCE(authority_root_public_key, '') AS authority_root_public_key,
       COALESCE(acme_issuer, '') AS acme_issuer,
       COALESCE(acme_endpoint, '') AS acme_endpoint,
       provider, account, access_key, access_secret, domain,
       domain_expire_time,
       created_at, updated_at, revision
FROM "guliangiam".certs
WHERE expire_time IS NOT NULL
  AND expire_time > $1
  AND expire_time <= $2
ORDER BY expire_time ASC, id ASC
LIMIT $3`
)

package gateway

const (
	queryGetServerByID = `
SELECT id, organization_id, name, endpoint, protocol, status, metadata, created_at, updated_at, deleted_at, revision
FROM guliangiam.servers
WHERE id = $1::uuid AND deleted_at IS NULL`

	queryGetServerByIDForOrganization = `
SELECT id, organization_id, name, endpoint, protocol, status, metadata, created_at, updated_at, deleted_at, revision
FROM guliangiam.servers
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`

	queryListServers = `
SELECT id, organization_id, name, endpoint, protocol, status, metadata, created_at, updated_at, deleted_at, revision
FROM guliangiam.servers
WHERE deleted_at IS NULL
  AND (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR status = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountServers = `
SELECT COUNT(*)
FROM guliangiam.servers
WHERE deleted_at IS NULL
  AND (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR status = $3)`

	queryInsertServer = `
INSERT INTO guliangiam.servers (id, organization_id, name, endpoint, protocol, status, metadata, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10)`

	queryUpdateServer = `
UPDATE guliangiam.servers
SET name = $1,
    endpoint = $2,
    protocol = $3,
    status = $4,
    metadata = $5,
    updated_at = $6,
    revision = revision + 1
WHERE id = $8::uuid AND deleted_at IS NULL AND revision = $7 RETURNING revision`

	queryDeleteServer = `
UPDATE guliangiam.servers
SET deleted_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL`

	queryGetSiteByID = `
SELECT id, organization_id, name, host, status, ssl_mode, cert_id, force_https,
       enable_health_check, health_check_interval, last_health_status, last_health_at,
       enable_alert, alert_interval, alert_try_times, alert_provider_ids,
       metadata, created_at, updated_at, deleted_at, revision
FROM guliangiam.sites
WHERE id = $1::uuid AND deleted_at IS NULL`

	queryGetSiteByIDForOrganization = `
SELECT id, organization_id, name, host, status, ssl_mode, cert_id, force_https,
       enable_health_check, health_check_interval, last_health_status, last_health_at,
       enable_alert, alert_interval, alert_try_times, alert_provider_ids,
       metadata, created_at, updated_at, deleted_at, revision
FROM guliangiam.sites
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`

	queryListSites = `
SELECT id, organization_id, name, host, status, ssl_mode, cert_id, force_https,
       enable_health_check, health_check_interval, last_health_status, last_health_at,
       enable_alert, alert_interval, alert_try_times, alert_provider_ids,
       metadata, created_at, updated_at, deleted_at, revision
FROM guliangiam.sites
WHERE deleted_at IS NULL
  AND (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR status = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountSites = `
SELECT COUNT(*)
FROM guliangiam.sites
WHERE deleted_at IS NULL
  AND (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%')
  AND ($3 = '' OR status = $3)`

	queryInsertSite = `
INSERT INTO guliangiam.sites (id, organization_id, name, host, status, ssl_mode, cert_id, force_https,
    enable_health_check, health_check_interval, last_health_status, last_health_at,
    enable_alert, alert_interval, alert_try_times, alert_provider_ids,
    metadata, created_at, updated_at, revision)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16::text[], $17, $18, $19, $20)`

	queryUpdateSite = `
UPDATE guliangiam.sites
SET name = $1,
    host = $2,
    status = $3,
    ssl_mode = $4,
    cert_id = $5,
    force_https = $6,
    enable_health_check = $7,
    health_check_interval = $8,
    last_health_status = $9,
    last_health_at = $10,
    enable_alert = $11,
    alert_interval = $12,
    alert_try_times = $13,
    alert_provider_ids = $14::text[],
    metadata = $15,
    updated_at = $16,
    revision = revision + 1
WHERE id = $18::uuid AND deleted_at IS NULL AND revision = $17 RETURNING revision`

	queryDeleteSite = `
UPDATE guliangiam.sites
SET deleted_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL`
)

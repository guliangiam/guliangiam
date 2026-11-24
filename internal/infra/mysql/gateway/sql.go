package gateway

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text[] / $N::jsonb  → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   resolve_org_id($1)                 → 直接 organization_id 比较（去掉 PG 专用函数）
//   RETURNING revision                 → MySQL 用 UPDATE + SELECT revision 两步
//   now()                              → NOW()
//   pgx.Query/Exec                     → QueryRowContext/ExecContext，ctx 第一个
//   metadata jsonb                     → TEXT（JSON 字符串）
//   alert_provider_ids text[]          → TEXT（JSON 数组字符串）

const btServers = "`servers`"
const btSites = "`sites`"

// ── Servers ──────────────────────────────────────────────────────

// queryGetServerByID 占位符：id(1)
const queryGetServerByID = `
SELECT id, organization_id, name, endpoint, protocol, status, metadata, created_at, updated_at, deleted_at, revision
FROM ` + btServers + `
WHERE id = ? AND deleted_at IS NULL`

// queryGetServerByIDForOrganization 占位符：id(1), organization_id(2)
const queryGetServerByIDForOrganization = `
SELECT id, organization_id, name, endpoint, protocol, status, metadata, created_at, updated_at, deleted_at, revision
FROM ` + btServers + `
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

// queryListServers 占位符计数：
//
//	org_id: CASE WHEN ?=?  → 2
//	keyword: ?='' LIKE CONCAT('%', ?, '%') → 3
//	status: ?='' OR status=? → 2
//	limit, offset → 2
//	总计：9
const queryListServers = `
SELECT id, organization_id, name, endpoint, protocol, status, metadata, created_at, updated_at, deleted_at, revision
FROM ` + btServers + `
WHERE deleted_at IS NULL
  AND (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR status = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountServers 去掉 LIMIT/OFFSET：org_id×2 + keyword×3 + status×2 = 7
const queryCountServers = `
SELECT COUNT(*)
FROM ` + btServers + `
WHERE deleted_at IS NULL
  AND (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR status = ?)`

const queryInsertServer = `
INSERT INTO ` + btServers + ` (id, organization_id, name, endpoint, protocol, status, metadata, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateServer 占位符：name(1), endpoint(1), protocol(1), status(1), metadata(1),
// updated_at(1), id(1), revision(1) → 8
// WHERE 不匹配返回零 affected rows → 调用方判断为 revision 冲突
const queryUpdateServer = `
UPDATE ` + btServers + `
SET name = ?,
    endpoint = ?,
    protocol = ?,
    status = ?,
    metadata = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND deleted_at IS NULL AND revision = ?`

// querySelectServerRevision: UPDATE 后取新 revision
const querySelectServerRevision = `SELECT revision FROM ` + btServers + ` WHERE id = ?`

const queryDeleteServer = `
UPDATE ` + btServers + `
SET deleted_at = NOW()
WHERE id = ? AND deleted_at IS NULL`

// ── Sites ────────────────────────────────────────────────────────

// queryGetSiteByID 占位符：id(1)
const queryGetSiteByID = `
SELECT id, organization_id, name, host, status, ssl_mode, cert_id, force_https,
       enable_health_check, health_check_interval, last_health_status, last_health_at,
       enable_alert, alert_interval, alert_try_times, alert_provider_ids,
       metadata, created_at, updated_at, deleted_at, revision
FROM ` + btSites + `
WHERE id = ? AND deleted_at IS NULL`

// queryGetSiteByIDForOrganization 占位符：id(1), organization_id(2)
const queryGetSiteByIDForOrganization = `
SELECT id, organization_id, name, host, status, ssl_mode, cert_id, force_https,
       enable_health_check, health_check_interval, last_health_status, last_health_at,
       enable_alert, alert_interval, alert_try_times, alert_provider_ids,
       metadata, created_at, updated_at, deleted_at, revision
FROM ` + btSites + `
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

// queryListSites 占位符计数同 ListServers：org_id×2 + keyword×3 + status×2 + limit+offset = 9
const queryListSites = `
SELECT id, organization_id, name, host, status, ssl_mode, cert_id, force_https,
       enable_health_check, health_check_interval, last_health_status, last_health_at,
       enable_alert, alert_interval, alert_try_times, alert_provider_ids,
       metadata, created_at, updated_at, deleted_at, revision
FROM ` + btSites + `
WHERE deleted_at IS NULL
  AND (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR status = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountSites: org_id×2 + keyword×3 + status×2 = 7
const queryCountSites = `
SELECT COUNT(*)
FROM ` + btSites + `
WHERE deleted_at IS NULL
  AND (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR status = ?)`

const queryInsertSite = `
INSERT INTO ` + btSites + ` (id, organization_id, name, host, status, ssl_mode, cert_id, force_https,
    enable_health_check, health_check_interval, last_health_status, last_health_at,
    enable_alert, alert_interval, alert_try_times, alert_provider_ids,
    metadata, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateSite 占位符：name(1), host(1), status(1), ssl_mode(1), cert_id(1), force_https(1),
// enable_health_check(1), health_check_interval(1), last_health_status(1), last_health_at(1),
// enable_alert(1), alert_interval(1), alert_try_times(1), alert_provider_ids(1), metadata(1),
// updated_at(1), id(1), revision(1) → 18
const queryUpdateSite = `
UPDATE ` + btSites + `
SET name = ?,
    host = ?,
    status = ?,
    ssl_mode = ?,
    cert_id = ?,
    force_https = ?,
    enable_health_check = ?,
    health_check_interval = ?,
    last_health_status = ?,
    last_health_at = ?,
    enable_alert = ?,
    alert_interval = ?,
    alert_try_times = ?,
    alert_provider_ids = ?,
    metadata = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND deleted_at IS NULL AND revision = ?`

// querySelectSiteRevision: UPDATE 后取新 revision
const querySelectSiteRevision = `SELECT revision FROM ` + btSites + ` WHERE id = ?`

const queryDeleteSite = `
UPDATE ` + btSites + `
SET deleted_at = NOW()
WHERE id = ? AND deleted_at IS NULL`

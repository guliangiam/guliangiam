package webhooks

// SQL 方言转换规则（PG → MySQL）：
//
//	$N::uuid / $N::jsonb / $N::text / $N::boolean  → ?
//	"guliangiam".table                             → `table`（去掉 schema 前缀）
//	ILIKE                                          → LIKE
//	'a' || b || 'c'                                → CONCAT('a', b, 'c')
//	now()                                          → NOW()
//	$3::boolean IS NULL                            → (? IS NULL)
//	guliangiam.resolve_org_id($1)                  → $1（MySQL 无此函数，repo 层直接传 organization_id）
//	events @> $2::jsonb                            → JSON_CONTAINS(events, ?)
//	RETURNING col                                  → 删除（Update 拆成 UPDATE + SELECT revision）
//
// 表名常量（Go raw string 不能含反引号，用普通字符串拼接）。
const btWebhooks = "`webhooks`"

const (
	// queryGetWebhookByID — 按主键取 webhook。
	// 参数：1 个 ? = webhook id
	queryGetWebhookByID = `
SELECT id, organization_id, name, display_name, url, method, headers, events, is_enabled, COALESCE(secret, '') AS secret, retry_count, timeout_ms,
    COALESCE(organization, '') AS organization,
    COALESCE(content_type, 'application/json') AS content_type,
    COALESCE(custom_body, '') AS custom_body,
    COALESCE(custom_headers, '{}') AS custom_headers,
    COALESCE(cert_id, '') AS cert_id,
    COALESCE(is_error, false) AS is_error,
    COALESCE(error_text, '') AS error_text,
    COALESCE(token_fields, '{}') AS token_fields,
    COALESCE(object_fields, '{}') AS object_fields,
    COALESCE(is_user_extended, false) AS is_user_extended,
    COALESCE(single_org_only, false) AS single_org_only,
    COALESCE(max_retries, 3) AS max_retries,
    COALESCE(retry_interval, 60) AS retry_interval,
    COALESCE(use_exponential_backoff, false) AS use_exponential_backoff,
    created_at, updated_at, revision
FROM ` + btWebhooks + `
WHERE id = ?`

	// queryGetWebhookByIDForOrganization — 管理面 by-ID 读取的唯一入口。
	// MySQL 无 RLS，organization_id 谓词是唯一租户边界。
	// 参数：webhook id(1)、organization_id(2)
	queryGetWebhookByIDForOrganization = `
SELECT id, organization_id, name, display_name, url, method, headers, events, is_enabled, COALESCE(secret, '') AS secret, retry_count, timeout_ms,
    COALESCE(organization, '') AS organization,
    COALESCE(content_type, 'application/json') AS content_type,
    COALESCE(custom_body, '') AS custom_body,
    COALESCE(custom_headers, '{}') AS custom_headers,
    COALESCE(cert_id, '') AS cert_id,
    COALESCE(is_error, false) AS is_error,
    COALESCE(error_text, '') AS error_text,
    COALESCE(token_fields, '{}') AS token_fields,
    COALESCE(object_fields, '{}') AS object_fields,
    COALESCE(is_user_extended, false) AS is_user_extended,
    COALESCE(single_org_only, false) AS single_org_only,
    COALESCE(max_retries, 3) AS max_retries,
    COALESCE(retry_interval, 60) AS retry_interval,
    COALESCE(use_exponential_backoff, false) AS use_exponential_backoff,
    created_at, updated_at, revision
FROM ` + btWebhooks + `
WHERE id = ?
  AND organization_id = ?`

	// queryListWebhooks — 分页列出 webhooks。
	// 方言替换：
	//   PG CASE WHEN $1='' → (? = '' OR organization_id = ?)
	//   PG '%'||$2||'%' ILIKE → LIKE CONCAT('%', ?, '%')
	//   PG $3::boolean IS NULL → (? IS NULL)
	//   PG guliangiam.resolve_org_id($1) → $1
	//
	// 参数顺序：
	//   organization_id(×2), keyword(×4), is_enabled(×2), method(×2), limit, offset
	//   共 2+4+2+2+2 = 12 个
	queryListWebhooks = `
SELECT id, organization_id, name, display_name, url, method, headers, events, is_enabled, COALESCE(secret, '') AS secret, retry_count, timeout_ms,
    COALESCE(organization, '') AS organization,
    COALESCE(content_type, 'application/json') AS content_type,
    COALESCE(custom_body, '') AS custom_body,
    COALESCE(custom_headers, '{}') AS custom_headers,
    COALESCE(cert_id, '') AS cert_id,
    COALESCE(is_error, false) AS is_error,
    COALESCE(error_text, '') AS error_text,
    COALESCE(token_fields, '{}') AS token_fields,
    COALESCE(object_fields, '{}') AS object_fields,
    COALESCE(is_user_extended, false) AS is_user_extended,
    COALESCE(single_org_only, false) AS single_org_only,
    COALESCE(max_retries, 3) AS max_retries,
    COALESCE(retry_interval, 60) AS retry_interval,
    COALESCE(use_exponential_backoff, false) AS use_exponential_backoff,
    created_at, updated_at, revision
FROM ` + btWebhooks + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
  AND (? = '' OR method = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

	// queryCountWebhooks — 与 List 条件同构，只返回 COUNT。
	// 参数顺序：organization_id(×2), keyword(×4), is_enabled(×2), method(×2)
	// 共 2+4+2+2 = 10 个
	queryCountWebhooks = `
SELECT COUNT(*)
FROM ` + btWebhooks + `
WHERE (? = '' OR organization_id = ?)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? IS NULL OR is_enabled = ?)
  AND (? = '' OR method = ?)`

	// queryInsertWebhook — 插入 webhook。
	// 参数：id, organization_id, name, display_name, url, method, headers, events,
	//       is_enabled, secret, retry_count, timeout_ms, organization, content_type,
	//       custom_body, custom_headers, cert_id, is_error, error_text, token_fields,
	//       object_fields, is_user_extended, single_org_only, max_retries,
	//       retry_interval, use_exponential_backoff, created_at, updated_at, revision
	// 共 29 个
	queryInsertWebhook = `
INSERT INTO ` + btWebhooks + ` (id, organization_id, name, display_name, url, method, headers, events, is_enabled, secret, retry_count, timeout_ms, organization, content_type, custom_body, custom_headers, cert_id, is_error, error_text, token_fields, object_fields, is_user_extended, single_org_only, max_retries, retry_interval, use_exponential_backoff, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateWebhook — 乐观锁更新（不带 RETURNING，UPDATE 之后用 querySelectRevision 回读新 revision）。
	// 参数顺序：display_name, url, method, headers, events, is_enabled, secret, retry_count, timeout_ms,
	//           organization, content_type, custom_body, custom_headers, cert_id, is_error, error_text,
	//           token_fields, object_fields, is_user_extended, single_org_only, max_retries,
	//           retry_interval, use_exponential_backoff, updated_at, id, revision
	// 共 26 个（SET 24 + WHERE id + WHERE revision）
	queryUpdateWebhook = `
UPDATE ` + btWebhooks + `
SET display_name = ?,
    url = ?,
    method = ?,
    headers = ?,
    events = ?,
    is_enabled = ?,
    secret = ?,
    retry_count = ?,
    timeout_ms = ?,
    organization = ?,
    content_type = ?,
    custom_body = ?,
    custom_headers = ?,
    cert_id = ?,
    is_error = ?,
    error_text = ?,
    token_fields = ?,
    object_fields = ?,
    is_user_extended = ?,
    single_org_only = ?,
    max_retries = ?,
    retry_interval = ?,
    use_exponential_backoff = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ? AND revision = ?`

	// querySelectRevision — 回读 revision（UPDATE 后执行，替代 PG RETURNING）
	querySelectRevision = `SELECT revision FROM ` + btWebhooks + ` WHERE id = ?`

	// queryDeleteWebhook — 删除 webhook。
	queryDeleteWebhook = `DELETE FROM ` + btWebhooks + ` WHERE id = ?`

	// queryListWebhooksByOrgAndEvent — 按组织 + 事件类型查询已启用的 webhook。
	// 方言替换：
	//   PG guliangiam.resolve_org_id($1) → $1
	//   PG events @> $2::jsonb → JSON_CONTAINS(events, ?)
	//
	// 参数：organization_id, event_json 共 2 个
	queryListWebhooksByOrgAndEvent = `
SELECT id, organization_id, name, display_name, url, method, headers, events, is_enabled, COALESCE(secret, '') AS secret, retry_count, timeout_ms,
    COALESCE(organization, '') AS organization,
    COALESCE(content_type, 'application/json') AS content_type,
    COALESCE(custom_body, '') AS custom_body,
    COALESCE(custom_headers, '{}') AS custom_headers,
    COALESCE(cert_id, '') AS cert_id,
    COALESCE(is_error, false) AS is_error,
    COALESCE(error_text, '') AS error_text,
    COALESCE(token_fields, '{}') AS token_fields,
    COALESCE(object_fields, '{}') AS object_fields,
    COALESCE(is_user_extended, false) AS is_user_extended,
    COALESCE(single_org_only, false) AS single_org_only,
    COALESCE(max_retries, 3) AS max_retries,
    COALESCE(retry_interval, 60) AS retry_interval,
    COALESCE(use_exponential_backoff, false) AS use_exponential_backoff,
    created_at, updated_at, revision
FROM ` + btWebhooks + `
WHERE organization_id = ?
  AND is_enabled = true
  AND JSON_CONTAINS(events, ?)`
)

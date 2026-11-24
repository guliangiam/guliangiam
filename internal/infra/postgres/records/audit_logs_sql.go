package records

// 审计中心分类查询。
//
// 分类与结果完全由 action 约定推导，不改 records 表结构，也不影响审计链哈希：
//   - login:     action LIKE 'auth.login.%'
//   - error:     action LIKE '%.failed' OR action LIKE 'error.%'
//   - operation: 其余
var (
	queryListAuditLogs = `
SELECT id, organization_id, actor_id, action, resource_type, resource_id, ip::text, user_agent, surface, request_id, trace_id, metadata, created_at
FROM guliangiam.records
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR CASE $2
        WHEN 'login' THEN action LIKE 'auth.login.%'
        WHEN 'error' THEN (action LIKE '%.failed' OR action LIKE 'error.%')
        WHEN 'operation' THEN action NOT LIKE 'auth.login.%' AND action NOT LIKE '%.failed' AND action NOT LIKE 'error.%'
        ELSE TRUE END)
  AND ($3 = '' OR CASE $3
        WHEN 'failure' THEN action LIKE '%.failed'
        WHEN 'success' THEN action NOT LIKE '%.failed'
        ELSE TRUE END)
  AND ($4 = '' OR (resource_type = 'applications' AND resource_id = $4)
       OR COALESCE(metadata->>'applicationId', '') = $4)
  AND ($5::timestamptz IS NULL OR created_at >= $5::timestamptz)
  AND ($6::timestamptz IS NULL OR created_at <= $6::timestamptz)
  AND ($7 = '' OR STRPOS(LOWER(actor_id::text), LOWER($7)) > 0
       OR STRPOS(LOWER(action), LOWER($7)) > 0
       OR STRPOS(LOWER(resource_id), LOWER($7)) > 0)
ORDER BY created_at DESC
LIMIT $8 OFFSET $9`

	queryCountAuditLogs = `
SELECT COUNT(*)
FROM guliangiam.records
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR CASE $2
        WHEN 'login' THEN action LIKE 'auth.login.%'
        WHEN 'error' THEN (action LIKE '%.failed' OR action LIKE 'error.%')
        WHEN 'operation' THEN action NOT LIKE 'auth.login.%' AND action NOT LIKE '%.failed' AND action NOT LIKE 'error.%'
        ELSE TRUE END)
  AND ($3 = '' OR CASE $3
        WHEN 'failure' THEN action LIKE '%.failed'
        WHEN 'success' THEN action NOT LIKE '%.failed'
        ELSE TRUE END)
  AND ($4 = '' OR (resource_type = 'applications' AND resource_id = $4)
       OR COALESCE(metadata->>'applicationId', '') = $4)
  AND ($5::timestamptz IS NULL OR created_at >= $5::timestamptz)
  AND ($6::timestamptz IS NULL OR created_at <= $6::timestamptz)
  AND ($7 = '' OR STRPOS(LOWER(actor_id::text), LOWER($7)) > 0
       OR STRPOS(LOWER(action), LOWER($7)) > 0
       OR STRPOS(LOWER(resource_id), LOWER($7)) > 0)`

	// 登录日志：成功来自 sessions（权威会话事实），失败来自 records 的
	// auth.login.failed。UNION ALL 后统一排序分页。
	queryListLoginLogs = `
SELECT id, result, username, organization_id, application_id, ip, user_agent, reason, created_at FROM (
  SELECT s.id::text AS id,
         'success' AS result,
         COALESCE(u.username, '') AS username,
         u.organization_id::text AS organization_id,
         s.application_id::text AS application_id,
         COALESCE(s.ip::text, '') AS ip,
         COALESCE(s.user_agent, '') AS user_agent,
         '' AS reason,
         s.created_at AS created_at
  FROM guliangiam.sessions s
  LEFT JOIN guliangiam.users u ON u.id = s.user_id
  WHERE (CASE WHEN $1 = '' THEN TRUE
              ELSE EXISTS (SELECT 1 FROM guliangiam.users uu
                           WHERE uu.id = s.user_id
                             AND uu.organization_id = guliangiam.resolve_org_id($1)) END)
    AND ($2 = '' OR s.application_id::text = $2)
    AND ($3 = '' OR $3 = 'success')
    AND ($4 = '' OR STRPOS(LOWER(COALESCE(u.username, '')), LOWER($4)) > 0)
    AND ($5::timestamptz IS NULL OR s.created_at >= $5::timestamptz)
    AND ($6::timestamptz IS NULL OR s.created_at <= $6::timestamptz)
  UNION ALL
  SELECT r.id::text AS id,
         'failure' AS result,
         COALESCE(r.metadata->>'username', r.actor_id::text, '') AS username,
         r.organization_id::text AS organization_id,
         COALESCE(r.metadata->>'applicationId', '') AS application_id,
         COALESCE(r.ip::text, '') AS ip,
         COALESCE(r.user_agent, '') AS user_agent,
         COALESCE(r.metadata->>'reason', r.metadata->>'message', '') AS reason,
         r.created_at AS created_at
  FROM guliangiam.records r
  WHERE r.action = 'auth.login.failed'
    AND (CASE WHEN $1 = '' THEN TRUE ELSE r.organization_id = guliangiam.resolve_org_id($1) END)
    AND ($2 = '' OR COALESCE(r.metadata->>'applicationId', '') = $2)
    AND ($3 = '' OR $3 = 'failure')
    AND ($4 = '' OR STRPOS(LOWER(COALESCE(r.metadata->>'username', r.actor_id::text, '')), LOWER($4)) > 0)
    AND ($5::timestamptz IS NULL OR r.created_at >= $5::timestamptz)
    AND ($6::timestamptz IS NULL OR r.created_at <= $6::timestamptz)
) logs
ORDER BY created_at DESC
LIMIT $7 OFFSET $8`

	queryCountLoginLogs = `
SELECT COUNT(*) FROM (
  SELECT s.id
  FROM guliangiam.sessions s
  LEFT JOIN guliangiam.users u ON u.id = s.user_id
  WHERE (CASE WHEN $1 = '' THEN TRUE
              ELSE EXISTS (SELECT 1 FROM guliangiam.users uu
                           WHERE uu.id = s.user_id
                             AND uu.organization_id = guliangiam.resolve_org_id($1)) END)
    AND ($2 = '' OR s.application_id::text = $2)
    AND ($3 = '' OR $3 = 'success')
    AND ($4 = '' OR STRPOS(LOWER(COALESCE(u.username, '')), LOWER($4)) > 0)
    AND ($5::timestamptz IS NULL OR s.created_at >= $5::timestamptz)
    AND ($6::timestamptz IS NULL OR s.created_at <= $6::timestamptz)
  UNION ALL
  SELECT r.id
  FROM guliangiam.records r
  WHERE r.action = 'auth.login.failed'
    AND (CASE WHEN $1 = '' THEN TRUE ELSE r.organization_id = guliangiam.resolve_org_id($1) END)
    AND ($2 = '' OR COALESCE(r.metadata->>'applicationId', '') = $2)
    AND ($3 = '' OR $3 = 'failure')
    AND ($4 = '' OR STRPOS(LOWER(COALESCE(r.metadata->>'username', r.actor_id::text, '')), LOWER($4)) > 0)
    AND ($5::timestamptz IS NULL OR r.created_at >= $5::timestamptz)
    AND ($6::timestamptz IS NULL OR r.created_at <= $6::timestamptz)
) logs`
)

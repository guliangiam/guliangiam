package sessions

const (
	queryGetSessionByID = `
SELECT s.id,
       s.user_id,
       u.username,
       u.organization_id,
       o.slug,
       s.application_id,
       a.slug,
       s.session_state,
       s.sid,
       s.device_info,
       COALESCE(HOST(s.ip), ''),
       COALESCE(s.user_agent, ''),
       s.expires_at,
       s.revoked_at,
       s.created_at,
      s.last_seen_at,
       s.updated_at,
       s.revision,
       s.authentication_level,
       s.assurance_level,
       s.is_privileged,
       COALESCE(s.lifecycle_id::text, ''),
       COALESCE(s.ticket_id::text, '')
FROM guliangiam.sessions s
JOIN guliangiam.users u ON u.id = s.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = s.application_id
WHERE s.id = $1::uuid`

	// queryGetSessionByIDForOrganization — sessions 表没有 organization_id 列，
	// 租户归属经 users 关联判定（与 RLS predicate 同源）。外部或缺失的行统一 NotFound。
	queryGetSessionByIDForOrganization = `
SELECT s.id,
       s.user_id,
       u.username,
       u.organization_id,
       o.slug,
       s.application_id,
       a.slug,
       s.session_state,
       s.sid,
       s.device_info,
       COALESCE(HOST(s.ip), ''),
       COALESCE(s.user_agent, ''),
       s.expires_at,
       s.revoked_at,
       s.created_at,
      s.last_seen_at,
       s.updated_at,
       s.revision,
       s.authentication_level,
       s.assurance_level,
       s.is_privileged,
       COALESCE(s.lifecycle_id::text, ''),
       COALESCE(s.ticket_id::text, '')
FROM guliangiam.sessions s
JOIN guliangiam.users u ON u.id = s.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = s.application_id
WHERE s.id = $1::uuid
  AND u.organization_id = guliangiam.resolve_org_id($2)`

	queryListSessions = `
SELECT s.id,
       s.user_id,
       u.username,
       u.organization_id,
       o.slug,
       s.application_id,
       a.slug,
       s.session_state,
       s.sid,
       s.device_info,
       COALESCE(HOST(s.ip), ''),
       COALESCE(s.user_agent, ''),
       s.expires_at,
       s.revoked_at,
       s.created_at,
      s.last_seen_at,
       s.updated_at,
       s.revision,
       s.authentication_level,
       s.assurance_level,
       s.is_privileged,
       COALESCE(s.lifecycle_id::text, ''),
       COALESCE(s.ticket_id::text, '')
FROM guliangiam.sessions s
JOIN guliangiam.users u ON u.id = s.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = s.application_id
WHERE ($1 = '' OR u.organization_id::text = $1)
  AND ($2 = '' OR s.user_id::text = $2)
  AND ($3 = '' OR s.application_id::text = $3)
  AND ($4 = '' OR s.session_state = $4)
  AND ($5::boolean IS NULL OR (($5 = true AND s.revoked_at IS NULL AND s.expires_at > now()) OR ($5 = false AND (s.revoked_at IS NOT NULL OR s.expires_at <= now()))))
  AND ($6 = '' OR u.username ILIKE '%' || $6 || '%' OR o.slug ILIKE '%' || $6 || '%' OR a.slug ILIKE '%' || $6 || '%' OR COALESCE(s.user_agent, '') ILIKE '%' || $6 || '%')
ORDER BY s.created_at DESC
LIMIT $7 OFFSET $8`

	queryCountSessions = `
SELECT COUNT(*)
FROM guliangiam.sessions s
JOIN guliangiam.users u ON u.id = s.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
JOIN guliangiam.applications a ON a.id = s.application_id
WHERE ($1 = '' OR u.organization_id::text = $1)
  AND ($2 = '' OR s.user_id::text = $2)
  AND ($3 = '' OR s.application_id::text = $3)
  AND ($4 = '' OR s.session_state = $4)
  AND ($5::boolean IS NULL OR (($5 = true AND s.revoked_at IS NULL AND s.expires_at > now()) OR ($5 = false AND (s.revoked_at IS NOT NULL OR s.expires_at <= now()))))
  AND ($6 = '' OR u.username ILIKE '%' || $6 || '%' OR o.slug ILIKE '%' || $6 || '%' OR a.slug ILIKE '%' || $6 || '%' OR COALESCE(s.user_agent, '') ILIKE '%' || $6 || '%')`

	queryInsertSession = `
INSERT INTO guliangiam.sessions (id, user_id, application_id, session_state, sid, device_info, ip, user_agent, expires_at, revoked_at, created_at, last_seen_at, updated_at, revision, authentication_level, assurance_level) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6::jsonb, NULLIF($7, '')::inet, NULLIF($8, ''), $9, $10, $11, $12, $13, $14, $15, $16)`

	queryTouchSession = `
UPDATE guliangiam.sessions
SET last_seen_at = $2,
    updated_at = $2,
    revision = revision + 1
WHERE id = $1::uuid
  AND revoked_at IS NULL
  AND expires_at > $2`

	queryRevokeSession = `
UPDATE guliangiam.sessions
SET session_state = 'revoked',
    revoked_at = $2,
    updated_at = $2,
    revision = revision + 1
WHERE id = $1::uuid
  AND revoked_at IS NULL`

	// queryRevokeSessionForOrganization — 租户谓词经 users.org 子查询，
	// 与 MySQL 镜像语义一致（sessions 无 organization_id 列）。
	queryRevokeSessionForOrganization = `
UPDATE guliangiam.sessions
SET session_state = 'revoked',
    revoked_at = $2,
    updated_at = $2,
    revision = revision + 1
WHERE id = $1::uuid
  AND revoked_at IS NULL
  AND user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.resolve_org_id($3))`

	// 用户维度撤销语句成对存在：不带排除谓词版本用于“撤销全部会话”，带排除
	// 版本把“保留某个会话”的 uuid 谓词放在最后一个参数位。选择哪一条只能由
	// revokeSessionsQuery 决定：空 excludeSessionID 永远不会把 "" 绑定到 uuid。
	queryRevokeUserSessions = `
UPDATE guliangiam.sessions
SET session_state = 'revoked',
    revoked_at = $2,
    updated_at = $2,
    revision = revision + 1
WHERE user_id = $1::uuid
  AND revoked_at IS NULL
  AND expires_at > $2`

	queryRevokeUserSessionsExcept = `
UPDATE guliangiam.sessions
SET session_state = 'revoked',
    revoked_at = $2,
    updated_at = $2,
    revision = revision + 1
WHERE user_id = $1::uuid
  AND id <> $3
  AND revoked_at IS NULL
  AND expires_at > $2`

	queryRevokeUserAppSessions = `
UPDATE guliangiam.sessions
SET session_state = 'revoked',
    revoked_at = $3,
    updated_at = $3,
    revision = revision + 1
WHERE user_id = $1::uuid
  AND application_id = $2::uuid
  AND revoked_at IS NULL
  AND expires_at > $3`

	queryRevokeUserAppSessionsExcept = `
UPDATE guliangiam.sessions
SET session_state = 'revoked',
    revoked_at = $3,
    updated_at = $3,
    revision = revision + 1
WHERE user_id = $1::uuid
  AND application_id = $2::uuid
  AND id <> $4
  AND revoked_at IS NULL
  AND expires_at > $3`

	// queryRevokeAllOrgSessions revokes every active session in an organization.
	// Used for org-wide emergency lockdown (AC-RISK-005).
	// sessions has no organization_id column (the tenant lives on users), so the
	// org scope is resolved through the owning user — same semantics as the MySQL
	// repository. Filtering sessions.organization_id directly made the statement
	// fail at prepare time on a real schema.
	queryRevokeAllOrgSessions = `
UPDATE guliangiam.sessions
SET session_state = 'revoked',
    revoked_at = $2,
    updated_at = $2,
    revision = revision + 1
WHERE user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = $1::uuid)
  AND revoked_at IS NULL
  AND expires_at > $2`

	queryDeleteSession = `
DELETE FROM guliangiam.sessions
WHERE id = $1::uuid`

	// queryDeleteSessionForOrganization — 租户谓词经 users.org 子查询；
	// 0 行 = 不存在或他租户，统一 NotFound。
	queryDeleteSessionForOrganization = `
DELETE FROM guliangiam.sessions
WHERE id = $1::uuid
  AND user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.resolve_org_id($2))`

	queryUpdateSession = `
UPDATE guliangiam.sessions
SET session_state = $2,
    expires_at = $3,
    updated_at = $4,
    is_privileged = $6,
    lifecycle_id = NULLIF($7, '')::uuid,
    ticket_id = NULLIF($8, '')::uuid,
    assurance_level = $9,
    revision = revision + 1
WHERE id = $1::uuid
  AND revoked_at IS NULL
  AND revision = $5`
)

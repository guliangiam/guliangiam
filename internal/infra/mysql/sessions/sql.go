package sessions

// PG→MySQL 转换规则：
//
//	$N::uuid/jsonb/inet/text → ?            (MySQL 自动类型推断)
//	guliangiam.table → `table`              (MySQL 无 schema 前缀)
//	HOST(s.ip) → s.ip                       (MySQL ip 是 VARCHAR(45))
//	ILIKE → LIKE                             (MySQL 默认 CI collation)
//	now() → NOW()                            (MySQL 兼容，统一大写)
//	::text / ::inet cast → 去掉              (MySQL 隐式转换)
//	$N::boolean IS NULL → ? IS NULL           (布尔占位符简化)
//	NULLIF(?, '')::inet → NULLIF(?, '')       (inet cast 去掉)
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
       COALESCE(s.ip, ''),
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
       COALESCE(s.lifecycle_id, ''),
       COALESCE(s.ticket_id, '')
FROM ` + "`sessions`" + ` s
JOIN ` + "`users`" + ` u ON u.id = s.user_id
JOIN ` + "`organizations`" + ` o ON o.id = u.organization_id
JOIN ` + "`applications`" + ` a ON a.id = s.application_id
WHERE s.id = ?`

	// queryGetSessionByIDForOrganization — sessions 表没有 organization_id 列，
	// 租户归属经 users 关联判定（PG/MySQL 同构）。
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
       COALESCE(s.ip, ''),
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
       COALESCE(s.lifecycle_id, ''),
       COALESCE(s.ticket_id, '')
FROM ` + "`sessions`" + ` s
JOIN ` + "`users`" + ` u ON u.id = s.user_id
JOIN ` + "`organizations`" + ` o ON o.id = u.organization_id
JOIN ` + "`applications`" + ` a ON a.id = s.application_id
WHERE s.id = ?
  AND u.organization_id = ?`

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
       COALESCE(s.ip, ''),
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
       COALESCE(s.lifecycle_id, ''),
       COALESCE(s.ticket_id, '')
FROM ` + "`sessions`" + ` s
JOIN ` + "`users`" + ` u ON u.id = s.user_id
JOIN ` + "`organizations`" + ` o ON o.id = u.organization_id
JOIN ` + "`applications`" + ` a ON a.id = s.application_id
WHERE (? = '' OR u.organization_id = ?)
  AND (? = '' OR s.user_id = ?)
  AND (? = '' OR s.application_id = ?)
  AND (? = '' OR s.session_state = ?)
  AND (? IS NULL OR ((? = 1 AND s.revoked_at IS NULL AND s.expires_at > NOW()) OR (? = 0 AND (s.revoked_at IS NOT NULL OR s.expires_at <= NOW()))))
  AND (? = '' OR u.username LIKE CONCAT('%', ?, '%') OR o.slug LIKE CONCAT('%', ?, '%') OR a.slug LIKE CONCAT('%', ?, '%') OR COALESCE(s.user_agent, '') LIKE CONCAT('%', ?, '%'))
ORDER BY s.created_at DESC
LIMIT ? OFFSET ?`

	queryCountSessions = `
SELECT COUNT(*)
FROM ` + "`sessions`" + ` s
JOIN ` + "`users`" + ` u ON u.id = s.user_id
JOIN ` + "`organizations`" + ` o ON o.id = u.organization_id
JOIN ` + "`applications`" + ` a ON a.id = s.application_id
WHERE (? = '' OR u.organization_id = ?)
  AND (? = '' OR s.user_id = ?)
  AND (? = '' OR s.application_id = ?)
  AND (? = '' OR s.session_state = ?)
  AND (? IS NULL OR ((? = 1 AND s.revoked_at IS NULL AND s.expires_at > NOW()) OR (? = 0 AND (s.revoked_at IS NOT NULL OR s.expires_at <= NOW()))))
  AND (? = '' OR u.username LIKE CONCAT('%', ?, '%') OR o.slug LIKE CONCAT('%', ?, '%') OR a.slug LIKE CONCAT('%', ?, '%') OR COALESCE(s.user_agent, '') LIKE CONCAT('%', ?, '%'))`

	queryInsertSession = `
INSERT INTO ` + "`sessions`" + ` (id, user_id, application_id, session_state, sid, device_info, ip, user_agent, expires_at, revoked_at, created_at, last_seen_at, updated_at, revision, authentication_level, assurance_level)
VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?)`

	queryTouchSession = `
UPDATE ` + "`sessions`" + `
SET last_seen_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ?
  AND revoked_at IS NULL
  AND expires_at > ?`

	queryRevokeSession = `
UPDATE ` + "`sessions`" + `
SET session_state = 'revoked',
    revoked_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ?
  AND revoked_at IS NULL`

	// queryRevokeSessionForOrganization — 租户谓词经 users.org 子查询。
	queryRevokeSessionForOrganization = `
UPDATE ` + "`sessions`" + `
SET session_state = 'revoked',
    revoked_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE id = ?
  AND revoked_at IS NULL
  AND user_id IN (SELECT id FROM ` + "`users`" + ` WHERE organization_id = ?)`

	queryRevokeUserOtherSessions = `
UPDATE ` + "`sessions`" + `
SET session_state = 'revoked',
    revoked_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE user_id = ?
  AND id <> ?
  AND revoked_at IS NULL
  AND expires_at > ?`

	queryRevokeUserAppOtherSessions = `
UPDATE ` + "`sessions`" + `
SET session_state = 'revoked',
    revoked_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE user_id = ?
  AND application_id = ?
  AND id <> ?
  AND revoked_at IS NULL
  AND expires_at > ?`

	queryRevokeAllUserSessions = `
UPDATE ` + "`sessions`" + `
SET session_state = 'revoked',
    revoked_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE user_id = ?
  AND revoked_at IS NULL
  AND expires_at > ?`

	queryRevokeAllOrgSessions = `
UPDATE ` + "`sessions`" + `
SET session_state = 'revoked',
    revoked_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE user_id IN (SELECT id FROM ` + "`users`" + ` WHERE organization_id = ?)
  AND revoked_at IS NULL
  AND expires_at > ?`

	queryDeleteSession = `
DELETE FROM ` + "`sessions`" + `
WHERE id = ?`

	// queryDeleteSessionForOrganization — 0 行 = 不存在或他租户，统一 NotFound。
	queryDeleteSessionForOrganization = `
DELETE FROM ` + "`sessions`" + `
WHERE id = ?
  AND user_id IN (SELECT id FROM ` + "`users`" + ` WHERE organization_id = ?)`

	queryUpdateSession = `
UPDATE ` + "`sessions`" + `
SET session_state = ?,
    expires_at = ?,
    updated_at = ?,
    is_privileged = ?,
    lifecycle_id = NULLIF(?, ''),
    ticket_id = NULLIF(?, ''),
    assurance_level = ?,
    revision = revision + 1
WHERE id = ?
  AND revoked_at IS NULL
  AND revision = ?`

	queryCleanupExpired = `
DELETE FROM ` + "`sessions`" + `
WHERE expires_at < ?`
)

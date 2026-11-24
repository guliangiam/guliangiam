package dashboard

const (
	// CountUsers returns total, active, disabled, and new user counts.
	// Active = not forbidden, not deleted; Disabled = forbidden or deleted.
	queryCountUsers = `
SELECT COUNT(*)                                                    AS total,
       COUNT(*) FILTER (WHERE NOT is_forbidden AND deleted_at IS NULL) AS active,
       COUNT(*) FILTER (WHERE is_forbidden OR deleted_at IS NOT NULL)          AS disabled,
       COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE)          AS new_today,
       COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE - INTERVAL '7 days')  AS new_7d,
       COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE - INTERVAL '30 days') AS new_30d
FROM guliangiam.users
WHERE ($1 = '' OR organization_id = guliangiam.resolve_org_id($1))
  AND deleted_at IS NULL`

	queryCountOrganizations = `
SELECT COUNT(*)
FROM guliangiam.organizations`

	queryCountApplications = `
SELECT COUNT(*)
FROM guliangiam.applications
WHERE ($1 = '' OR organization_id = guliangiam.resolve_org_id($1))`

	queryCountRoles = `
SELECT COUNT(*)
FROM guliangiam.roles
WHERE ($1 = '' OR organization_id = guliangiam.resolve_org_id($1))`

	queryCountGroups = `
SELECT COUNT(*)
FROM guliangiam.groups
WHERE ($1 = '' OR organization_id = guliangiam.resolve_org_id($1))`

	queryCountPermissions = `
SELECT COUNT(*)
FROM guliangiam.permissions
WHERE ($1 = '' OR organization_id = guliangiam.resolve_org_id($1))`

	queryCountProviders = `
SELECT COUNT(*)
FROM guliangiam.providers
WHERE ($1 = '' OR organization_id = guliangiam.resolve_org_id($1))`

	// CountActiveSessions returns total and active session counts.
	// Active = not revoked and not expired.
	queryCountActiveSessions = `
SELECT COUNT(*)                                                    AS total,
       COUNT(*) FILTER (WHERE revoked_at IS NULL AND expires_at > NOW()) AS active
FROM guliangiam.sessions s
JOIN guliangiam.users u ON s.user_id = u.id
WHERE ($1 = '' OR u.organization_id = guliangiam.resolve_org_id($1))`

	// CountTokens returns total and active access token counts.
	// Active = not revoked and not expired.
	queryCountTokens = `
SELECT COUNT(*)                                                    AS total,
       COUNT(*) FILTER (WHERE revoked_at IS NULL AND expires_at > NOW()) AS active
FROM guliangiam.access_tokens at
JOIN guliangiam.users u ON at.user_id = u.id
WHERE ($1 = '' OR u.organization_id = guliangiam.resolve_org_id($1))`

	// CountFailedLogins24h counts failed login records in the last 24 hours.
	// 登录失败写入的是审计事件名 auth.login.failed（见 platform/outbox
	// providers/audit 与 app/auth 登录失败路径），不存在 'login-failed'。
	queryCountFailedLogins24h = `
SELECT COUNT(*)
FROM guliangiam.records
WHERE action = 'auth.login.failed'
  AND created_at > NOW() - INTERVAL '24 hours'
  AND ($1 = '' OR organization_id = guliangiam.resolve_org_id($1))`

	// GetLoginHeatmap returns login counts grouped by hour of day for the last 30 days.
	// 成功登录的权威源是 sessions（每条 session 创建即一次成功登录，与审计
	// 中心登录日志口径一致）；成功登录不写 records。sessions 无 organization_id
	// 列，租户过滤沿用 EXISTS(users) 模式。
	queryGetLoginHeatmap = `
SELECT EXTRACT(HOUR FROM s.created_at)::int AS hour,
       COUNT(*)                              AS count
FROM guliangiam.sessions s
WHERE s.created_at > NOW() - INTERVAL '30 days'
  AND ($1 = '' OR EXISTS (SELECT 1 FROM guliangiam.users u
                          WHERE u.id = s.user_id
                            AND u.organization_id = guliangiam.resolve_org_id($1)))
GROUP BY EXTRACT(HOUR FROM s.created_at)
ORDER BY hour`

	// GetMfaCoverageStats returns total users and users with MFA enabled.
	// preferred_mfa_type lives in the user_mfa_config 1:1 table (not users),
	// so we LEFT JOIN to preserve the "users without MFA = no row in config" semantics.
	queryGetMfaCoverageStats = `
SELECT COUNT(*)                                                          AS total,
       COUNT(*) FILTER (WHERE m.preferred_mfa_type IS NOT NULL
                             AND m.preferred_mfa_type != ''
                             AND m.preferred_mfa_type != 'none')        AS with_mfa
FROM guliangiam.users u
LEFT JOIN guliangiam.user_mfa_config m ON m.user_id = u.id
WHERE ($1 = '' OR u.organization_id = guliangiam.resolve_org_id($1))
  AND u.deleted_at IS NULL`

	// GetProviderDistribution returns provider counts grouped by category.
	queryGetProviderDistribution = `
SELECT category, COUNT(*) AS count
FROM guliangiam.providers
WHERE ($1 = '' OR organization_id = guliangiam.resolve_org_id($1))
GROUP BY category
ORDER BY count DESC`

	// GetTrends returns daily counts of users, applications, and providers
	// created in the last 30 days, plus daily login counts.
	// logins 桶来自 sessions（成功登录权威源），不再查 records。
	queryGetTrends = `
SELECT d::date::text                                                AS date,
       COUNT(DISTINCT u.id) FILTER (WHERE u.created_at::date = d::date)  AS users,
       COUNT(DISTINCT a.id) FILTER (WHERE a.created_at::date = d::date)  AS applications,
       COUNT(DISTINCT p.id) FILTER (WHERE p.created_at::date = d::date)  AS providers,
       COUNT(DISTINCT s.id)                                           AS logins
FROM generate_series(CURRENT_DATE - INTERVAL '29 days', CURRENT_DATE, INTERVAL '1 day') d
LEFT JOIN guliangiam.users u        ON u.created_at::date = d::date AND ($1 = '' OR u.organization_id = guliangiam.resolve_org_id($1)) AND u.deleted_at IS NULL
LEFT JOIN guliangiam.applications a ON a.created_at::date = d::date AND ($1 = '' OR a.organization_id = guliangiam.resolve_org_id($1))
LEFT JOIN guliangiam.providers p    ON p.created_at::date = d::date AND ($1 = '' OR p.organization_id = guliangiam.resolve_org_id($1))
LEFT JOIN guliangiam.sessions s     ON s.created_at::date = d::date AND ($1 = '' OR EXISTS (SELECT 1 FROM guliangiam.users su WHERE su.id = s.user_id AND su.organization_id = guliangiam.resolve_org_id($1)))
GROUP BY d::date
ORDER BY d::date`

	// GetActivityHeatmap returns daily login counts for the last 365 days.
	// 成功登录的权威源是 sessions（与审计中心登录日志口径一致）；
	// records 中不存在 action='login' 的行。
	queryGetActivityHeatmap = `
SELECT d::date::text AS date,
       COUNT(s.id)   AS count
FROM generate_series(CURRENT_DATE - INTERVAL '364 days', CURRENT_DATE, INTERVAL '1 day') d
LEFT JOIN guliangiam.sessions s ON s.created_at::date = d::date
    AND ($1 = '' OR EXISTS (SELECT 1 FROM guliangiam.users u
                            WHERE u.id = s.user_id
                              AND u.organization_id = guliangiam.resolve_org_id($1)))
GROUP BY d::date
ORDER BY d::date`

	// GetOrgUserDistribution returns user counts grouped by organization.
	queryGetOrgUserDistribution = `
SELECT o.display_name AS organization_name,
       COUNT(u.id)    AS user_count
FROM guliangiam.organizations o
LEFT JOIN guliangiam.users u ON u.organization_id = o.id AND u.deleted_at IS NULL
GROUP BY o.id, o.display_name
ORDER BY user_count DESC`
)

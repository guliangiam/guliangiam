package tokens

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text[] / $N::jsonb  → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $5 || '%'             → LIKE CONCAT('%', ?, '%')
//   resolve_org_id($1)                 → 直接 u.organization_id = ?
//   FOR UPDATE OF t                    → FOR UPDATE（MySQL 8.0）
//   RETURNING col                      → MySQL 不支持，分拆为 UPDATE + SELECT
//   WITH CTE + RETURNING               → MySQL 改写为多步 UPDATE
//   metadata ->> 'key'                 → JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.key'))
//   $4::boolean IS NULL                → ? IS NULL（MySQL 原生支持 bool）
//   text[]                             → TEXT（JSON 数组字符串）
//   jsonb                              → TEXT（JSON 字符串）
//   now()                              → NOW()

const btAccessTokens = "`access_tokens`"
const btRefreshTokens = "`refresh_tokens`"

// ── INSERT ──────────────────────────────────────────────────────

const queryInsertToken = `
INSERT INTO ` + btAccessTokens + ` (id, user_id, application_id, token_hash, token_prefix, device_secret_hash, device_secret_expires_in, scopes, audience, metadata, expires_at, revoked_at, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const queryInsertRefreshToken = `
INSERT INTO ` + btRefreshTokens + ` (id, access_token_id, user_id, application_id, token_hash, token_prefix, family_id, scopes, audience, expires_at, revoked_at, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateTokenDeviceSecret 持久化 Native SSO 设备密钥哈希。
// 调用时 access token 行已存在，因此用 UPDATE 而不是重复 INSERT。
const queryUpdateTokenDeviceSecret = `
UPDATE ` + btAccessTokens + `
SET device_secret_hash = ?, device_secret_expires_in = ?, updated_at = CURRENT_TIMESTAMP, revision = revision + 1
WHERE id = ?`

// ── SELECT Access Token（三表 JOIN users/organizations/applications，19 列）──

const tokenSelectBase = `
SELECT t.id,
       t.user_id,
       u.username,
       u.organization_id,
       o.slug,
       t.application_id,
       a.slug,
       t.token_prefix,
       t.token_hash,
       COALESCE(t.device_secret_hash, ''),
       COALESCE(t.device_secret_expires_in, 0),
       t.scopes,
       t.audience,
       t.metadata,
       t.expires_at,
       t.revoked_at,
       t.created_at,
       t.updated_at,
       t.revision
FROM ` + btAccessTokens + ` t
JOIN ` + "`users`" + ` u ON u.id = t.user_id
JOIN ` + "`organizations`" + ` o ON o.id = u.organization_id
JOIN ` + "`applications`" + ` a ON a.id = t.application_id`

const queryGetTokenByID = tokenSelectBase + `
WHERE t.id = ?`

// queryGetTokenByIDForOrganization — 占位符：id(1)、organization_id(2)。
// access_tokens 无 organization_id 列，租户归属经 users 表 join 绑定；
// MySQL 无 RLS，该谓词是 by-ID 读取的唯一租户边界。
const queryGetTokenByIDForOrganization = tokenSelectBase + `
WHERE t.id = ?
  AND u.organization_id = ?`

const queryGetTokenByHash = tokenSelectBase + `
WHERE t.token_hash = ?`

const queryGetTokenByDeviceSecretHash = tokenSelectBase + `
WHERE t.device_secret_hash = ?`

// ── SELECT Refresh Token（17 列）────────────────────────────────

const refreshTokenSelectBase = `
SELECT t.id,
       t.access_token_id,
       t.user_id,
       u.username,
       u.organization_id,
       o.slug,
       t.application_id,
       a.slug,
       t.token_hash,
       t.family_id,
       t.scopes,
       t.audience,
       t.expires_at,
       t.revoked_at,
       t.created_at,
       t.updated_at,
       t.revision
FROM ` + btRefreshTokens + ` t
JOIN ` + "`users`" + ` u ON u.id = t.user_id
JOIN ` + "`organizations`" + ` o ON o.id = u.organization_id
JOIN ` + "`applications`" + ` a ON a.id = t.application_id`

const queryGetRefreshTokenByHash = refreshTokenSelectBase + `
WHERE t.token_hash = ?
FOR UPDATE`

// ── List & Count（参数：org×2, user×2, app×2, active_only, keyword×6, limit, offset = 14）───
// MySQL 版的 active_only bool 直接用 ? 比较（PG 的 $4::boolean IS NULL 改为 ? IS NULL）
// keyword 出现 6 次：CASE WHEN + 5 个 LIKE CONCAT
// 实际用 ListTokens 动态拼装更简单，这里先定义基础 WHERE 子句

// queryListTokens 占位符：
//   CASE WHEN ? = '' THEN TRUE ELSE u.organization_id = ? END         — org_id ×2
//   CASE WHEN ? = '' THEN TRUE ELSE t.user_id = ? END                  — user_id ×2
//   CASE WHEN ? = '' THEN TRUE ELSE t.application_id = ? END           — app_id ×2
//   (? IS NULL OR ... active_only bool 直接比较)                       — active_only ×1
//   (? = '' OR u.username LIKE CONCAT('%', ?, '%')
//      OR o.slug LIKE CONCAT('%', ?, '%')
//      OR a.slug LIKE CONCAT('%', ?, '%')
//      OR t.token_prefix LIKE CONCAT('%', ?, '%')
//      OR t.token_hash LIKE CONCAT('%', ?, '%'))                      — keyword ×6
//   LIMIT ? OFFSET ?                                                    — 2
//   总计：15
//
// 但 active_only 在 PG 版里不是简单 bool 比较，它有 AND ($4::boolean IS NULL OR (($4 = true AND ...) OR ($4 = false AND ...))) 逻辑
// 这个 PG 条件是：
//   $4 IS NULL → TRUE（不过滤）
//   $4 = true → AND t.revoked_at IS NULL AND t.expires_at > NOW()
//   $4 = false → AND (t.revoked_at IS NOT NULL OR t.expires_at <= NOW())
//
// MySQL 版等价转换（直接展开）：
//   (? IS NULL OR (? = true AND t.revoked_at IS NULL AND t.expires_at > NOW())
//                 OR (? = false AND (t.revoked_at IS NOT NULL OR t.expires_at <= NOW())))
//   这会让 active_only 出现 3 次！为了简洁，我们在 repository 层展开这个条件。

// 简化方案：把 active_only WHERE 直接展开成 MySQL 可执行的动态 WHERE，
// 在 repository 层根据 q.ActiveOnly 是否为 nil 来决定是否追加。
// 这样 ListTokens 的 SQL 变成没有 active_only 条件，repository 层直接在 SQL 里拼接。
// 但 sqlmock 需要严格的 SQL 匹配，所以我们用一种方式：active_only 永远出现 3 次（nil 时三个都传 nil）。

const queryListTokens = tokenSelectBase + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE u.organization_id = ? END)
  AND (CASE WHEN ? = '' THEN TRUE ELSE t.user_id = ? END)
  AND (CASE WHEN ? = '' THEN TRUE ELSE t.application_id = ? END)
  AND (? IS NULL OR (? = true AND t.revoked_at IS NULL AND t.expires_at > NOW())
                 OR (? = false AND (t.revoked_at IS NOT NULL OR t.expires_at <= NOW())))
  AND (? = '' OR u.username LIKE CONCAT('%', ?, '%')
       OR o.slug LIKE CONCAT('%', ?, '%')
       OR a.slug LIKE CONCAT('%', ?, '%')
       OR t.token_prefix LIKE CONCAT('%', ?, '%')
       OR t.token_hash LIKE CONCAT('%', ?, '%'))
ORDER BY t.created_at DESC
LIMIT ? OFFSET ?`

// queryCountTokens 去掉 LIMIT/OFFSET，占位符数量相同（去掉最后 2 个）
const queryCountTokens = tokenSelectBase + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE u.organization_id = ? END)
  AND (CASE WHEN ? = '' THEN TRUE ELSE t.user_id = ? END)
  AND (CASE WHEN ? = '' THEN TRUE ELSE t.application_id = ? END)
  AND (? IS NULL OR (? = true AND t.revoked_at IS NULL AND t.expires_at > NOW())
                 OR (? = false AND (t.revoked_at IS NOT NULL OR t.expires_at <= NOW())))
  AND (? = '' OR u.username LIKE CONCAT('%', ?, '%')
       OR o.slug LIKE CONCAT('%', ?, '%')
       OR a.slug LIKE CONCAT('%', ?, '%')
       OR t.token_prefix LIKE CONCAT('%', ?, '%')
       OR t.token_hash LIKE CONCAT('%', ?, '%'))`

// 占位符计数验证：
// List: org×2 + user×2 + app×2 + active×3 + keyword×6 + limit+offset×2 = 17
// Count: org×2 + user×2 + app×2 + active×3 + keyword×6 = 15

// ── Revoke ────────────────────────────────────────────────────────

const queryRevokeToken = `
UPDATE ` + btAccessTokens + `
SET revoked_at = COALESCE(revoked_at, ?),
    updated_at = ?,
    revision = revision + 1
WHERE id = ?
  AND revoked_at IS NULL`

// queryRevokeTokensByQuery: PG 用 UPDATE FROM JOIN，MySQL 用 UPDATE ... JOIN ...
// PG: FROM guliangiam.users u WHERE u.id = t.user_id AND (CASE WHEN $1=... END)
// → MySQL: UPDATE at JOIN users u ON u.id = at.user_id WHERE (CASE WHEN ?=” THEN TRUE ELSE u.organization_id = ? END) ...
// 占位符：org_id×2, user_id×2, app_id×2, token_prefix×2, revoked_at × 2（在 SET COALESCE 和 WHERE expires_at） = 10
const queryRevokeTokensByQuery = `
UPDATE ` + btAccessTokens + ` at
JOIN ` + "`users`" + ` u ON u.id = at.user_id
SET at.revoked_at = COALESCE(at.revoked_at, ?),
    at.updated_at = ?,
    at.revision = at.revision + 1
WHERE (CASE WHEN ? = '' THEN TRUE ELSE u.organization_id = ? END)
  AND (CASE WHEN ? = '' THEN TRUE ELSE at.user_id = ? END)
  AND (CASE WHEN ? = '' THEN TRUE ELSE at.application_id = ? END)
  AND (CASE WHEN ? = '' THEN TRUE ELSE at.token_prefix = ? END)
  AND at.revoked_at IS NULL
  AND at.expires_at > ?`

const queryRevokeRefreshToken = `
UPDATE ` + btRefreshTokens + `
SET revoked_at = COALESCE(revoked_at, ?),
    updated_at = ?,
    revision = revision + 1
WHERE id = ?
  AND revoked_at IS NULL`

const queryDeleteToken = `
DELETE FROM ` + btAccessTokens + `
WHERE id = ?`

// ── CTE RETURNING 改写为 MySQL 多步 UPDATE ────────────────────────

// RevokeByFamilyID 改写：
//  1. revoke refresh_tokens where family_id = ? AND revoked_at IS NULL → affected = refresh_count
//  2. 同时 revoke 关联 access_tokens 用 UPDATE JOIN
//
// 占位符：revoked_at(2), family_id(2) = 4
const queryRevokeRefreshByFamily = `
UPDATE ` + btRefreshTokens + `
SET revoked_at = COALESCE(revoked_at, ?),
    updated_at = ?,
    revision = revision + 1
WHERE family_id = ?
  AND revoked_at IS NULL`

const queryRevokeAccessByFamilyJoin = `
UPDATE ` + btAccessTokens + ` at
JOIN ` + btRefreshTokens + ` rt ON rt.access_token_id = at.id AND rt.family_id = ?
SET at.revoked_at = COALESCE(at.revoked_at, ?),
    at.updated_at = ?,
    at.revision = at.revision + 1
WHERE rt.access_token_id IS NOT NULL
  AND at.revoked_at IS NULL`

// RevokeRefreshByUser 改写：
//  1. revoke refresh_tokens where user_id = ? AND revoked_at IS NULL → affected = refresh_count
//  2. 同时 revoke 关联 access_tokens 用 UPDATE JOIN
//
// 占位符：revoked_at(2), user_id(2) = 4
const queryRevokeRefreshByUser = `
UPDATE ` + btRefreshTokens + `
SET revoked_at = COALESCE(revoked_at, ?),
    updated_at = ?,
    revision = revision + 1
WHERE user_id = ?
  AND revoked_at IS NULL`

const queryRevokeAccessByUserJoin = `
UPDATE ` + btAccessTokens + ` at
JOIN ` + btRefreshTokens + ` rt ON rt.access_token_id = at.id AND rt.user_id = ?
SET at.revoked_at = COALESCE(at.revoked_at, ?),
    at.updated_at = ?,
    at.revision = at.revision + 1
WHERE rt.access_token_id IS NOT NULL
  AND at.revoked_at IS NULL`

// RevokeByAuthorizationCodeID 改写：
//
//	metadata ->> 'authorizationCodeId' = $1 → MySQL JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.authorizationCodeId')) = ?
//	1. revoke access_tokens where JSON_EXTRACT(metadata, '$..authorizationCodeId') = ? → affected = access_count
//	2. revoke refresh_tokens where access_token_id IN (SELECT id FROM access_tokens WHERE ...) → affected = refresh_count
//	3. return access_count + refresh_count
const queryRevokeAccessByCodeID = `
UPDATE ` + btAccessTokens + `
SET revoked_at = COALESCE(revoked_at, ?),
    updated_at = ?,
    revision = revision + 1
WHERE JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.authorizationCodeId')) = ?
  AND revoked_at IS NULL`

const queryRevokeRefreshByAccessToken = `
UPDATE ` + btRefreshTokens + `
SET revoked_at = COALESCE(revoked_at, ?),
    updated_at = ?,
    revision = revision + 1
WHERE access_token_id IN (SELECT id FROM ` + btAccessTokens + ` WHERE JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.authorizationCodeId')) = ? AND revoked_at IS NOT NULL AND updated_at = ?)
  AND revoked_at IS NULL`

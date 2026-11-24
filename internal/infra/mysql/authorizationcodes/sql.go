package authorizationcodes

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text / $N::boolean  → ?
//   "guliangiam".table                 → `table`（去掉 schema 前缀）
//   scopes text[]                      → TEXT（JSON 数组字符串）
//   COALESCE(x, NULL)                  → 保留

const btAuthorizationCodes = "`authorization_codes`"

// GetByCode: 通过 code_hash（SHA-256）查找授权码。
// MySQL 版把 $1 改成 ?，去掉 guliangiam schema 前缀和 ::uuid cast。
const queryGetAuthorizationCodeByCode = `
SELECT c.id,
       c.user_id,
       u.username,
       u.organization_id,
       o.slug,
       c.application_id,
       a.slug,
       c.redirect_uri,
       c.scopes,
       c.state,
       c.nonce,
       c.code_challenge,
       c.code_challenge_method,
       c.resource,
       c.sid,
       c.auth_time,
       c.expires_at,
       c.used_at,
       c.created_at
FROM ` + btAuthorizationCodes + ` c
JOIN ` + "`users`" + ` u ON u.id = c.user_id
JOIN ` + "`organizations`" + ` o ON o.id = u.organization_id
JOIN ` + "`applications`" + ` a ON a.id = c.application_id
WHERE c.code_hash = ?`

// Create: 插入授权码。
const queryInsertAuthorizationCode = `
INSERT INTO ` + btAuthorizationCodes + ` (id, code_hash, code_prefix, user_id, application_id, redirect_uri, scopes, state, sid, nonce, resource, code_challenge, code_challenge_method, auth_time, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// MarkUsed: 一次性消费授权码。used_at IS NULL 保证并发安全。
const queryMarkAuthorizationCodeUsed = `
UPDATE ` + btAuthorizationCodes + `
SET used_at = COALESCE(used_at, ?)
WHERE id = ?
  AND used_at IS NULL`

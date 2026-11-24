package consents

import (
	"github.com/guliangiam/guliangiam/internal/infra/dialect"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::jsonb  → ?
//   "guliangiam".t        → `t`
//   ON CONFLICT DO UPDATE SET EXCLUDED.x = EXCLUDED.y → 经 mysqlshared.Dialect.Upsert 生成
//   NOW()                 → NOW()（MySQL 也支持）
//   pgx.Query/Exec        → QueryRowContext/ExecContext，ctx 第一个

const btConsents = "`consents`"

// GetByUserAndApplication: 按 user_id + application_id 查 consent。
const queryGetConsentByUserAndApplication = `
SELECT id,
       user_id,
       username,
       organization_id,
       organization_slug,
       application_id,
       application_slug,
       granted_scopes,
       created_at,
       updated_at,
       revoked_at,
       revision
FROM ` + btConsents + `
WHERE user_id = ?
  AND application_id = ?`

// CountActiveByApplication: 按 application_id 统计未撤销的 consent。
const queryCountActiveConsentsByApplication = `
SELECT COUNT(*)
FROM ` + btConsents + `
WHERE application_id = ?
  AND revoked_at IS NULL`

// Upsert: 方言敏感部分经 mysqlshared.Dialect.Upsert 生成。
var queryUpsertConsent = `
INSERT INTO ` + btConsents + ` (id, user_id, username, organization_id, organization_slug, application_id, application_slug, granted_scopes, created_at, updated_at, revoked_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(
	dialect.DialectUpsert{
		UpdateColumns: []string{
			"username", "organization_id", "organization_slug",
			"application_slug", "granted_scopes", "updated_at", "revision",
		},
	}, nil) + `
`

// Revoke: 撤销 consent，同时 updated_at 和 revision 自增。
const queryRevokeConsent = `
UPDATE ` + btConsents + `
SET revoked_at = NOW(),
    updated_at = NOW(),
    revision = revision + 1
WHERE user_id = ?
  AND application_id = ?
  AND revoked_at IS NULL`

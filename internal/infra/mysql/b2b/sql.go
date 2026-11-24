package b2b

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid                        → ?（CHAR(36) 列直接比对）
//   "guliangiam".table             → `table`
//   LOWER(domain) = LOWER($1)      → 同构保留（utf8mb4_unicode_ci 本身已大小写不敏感，显式 LOWER 保持与 PG 语义一致）
//   pgx.Query/QueryRow/Exec        → QueryContext/QueryRowContext/ExecContext
//
// RLS：MySQL 无行级安全策略，organization_id 过滤必须由 SQL 显式携带。
// ClaimPending 无 RETURNING，按 MySQL outbox 既有模式拆成
// 「SELECT ids FOR UPDATE SKIP LOCKED → UPDATE 租约 → SELECT 全量行」三步。

import (
	"strings"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

const btOrganizationDomains = "`organization_domains`"

// 列清单集中维护：续验预算字段只在这里追加一次。
const orgDomainColumns = `id, organization_id, domain, verification_token, status, verified_at, created_at, updated_at, attempts, last_error, next_retry_at, last_checked_at`

// queryGetOrganizationDomainByID 占位符：id(1)
const queryGetOrganizationDomainByID = `SELECT ` + orgDomainColumns + ` FROM ` + btOrganizationDomains + ` WHERE id = ?`

// queryGetOrganizationDomainByIDForOrganization 占位符：id(1), organization_id(2)
const queryGetOrganizationDomainByIDForOrganization = `SELECT ` + orgDomainColumns + ` FROM ` + btOrganizationDomains + ` WHERE id = ? AND organization_id = ?`

// queryGetOrganizationDomainByName (organization_id, domain) 精确匹配 —— 创建前查重。
// 占位符：organization_id(1), domain(2)
const queryGetOrganizationDomainByName = `SELECT ` + orgDomainColumns + ` FROM ` + btOrganizationDomains + ` WHERE organization_id = ? AND domain = ?`

// queryFindVerifiedOrganizationDomain 邮箱域 SSO 发现 —— 跨租户查询，读取至多两条以拒绝歧义。
// 占位符：domain(1)
const queryFindVerifiedOrganizationDomain = `SELECT ` + orgDomainColumns + ` FROM ` + btOrganizationDomains + ` WHERE LOWER(domain) = LOWER(?) AND status = 'verified' LIMIT 2`

// queryListOrganizationDomains 占位符：organization_id(1) + status×2 + limit + offset = 5
const queryListOrganizationDomains = `SELECT ` + orgDomainColumns + ` FROM ` + btOrganizationDomains + ` WHERE organization_id = ? AND (? = '' OR status = ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`

// queryCountOrganizationDomains 占位符：organization_id(1) + status×2 = 3
const queryCountOrganizationDomains = `SELECT COUNT(*) FROM ` + btOrganizationDomains + ` WHERE organization_id = ? AND (? = '' OR status = ?)`

// queryClaimPendingIDs 选出到期 pending 行并加行锁；调用方必须处于事务中。
// 占位符：limit(1)
var queryClaimPendingIDs = `SELECT id FROM ` + btOrganizationDomains + `
WHERE status = 'pending' AND next_retry_at <= NOW(6)
ORDER BY next_retry_at ASC, created_at ASC
LIMIT ? ` + mysqlshared.Dialect.ForUpdateSkipLocked()

// queryClaimPendingLease 给已锁定的行推进租约；占位符：租约秒数 + IN 展开的 id
const queryClaimPendingLease = `UPDATE ` + btOrganizationDomains + `
SET next_retry_at = DATE_ADD(NOW(6), INTERVAL ? SECOND), updated_at = NOW(6)
WHERE id IN (__PICKED_IDS__)`

// queryClaimPendingSelect 取回已加租约的行；占位符：IN 展开的 id
const queryClaimPendingSelect = `SELECT ` + orgDomainColumns + ` FROM ` + btOrganizationDomains + ` WHERE id IN (__PICKED_IDS__)`

const queryInsertOrganizationDomain = `INSERT INTO ` + btOrganizationDomains + ` (id, organization_id, domain, verification_token, status, verified_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdatePendingOrganizationDomain 只更新 pending 行 —— 终态不可逆由 WHERE 兜底。
// last_error 空串写 NULL；next_retry_at/last_checked_at 传 NULL 时保留旧值。
// 占位符：domain, verification_token, status, verified_at, updated_at,
// attempts, last_error, next_retry_at, last_checked_at, id = 10
const queryUpdatePendingOrganizationDomain = `UPDATE ` + btOrganizationDomains + ` SET domain = ?, verification_token = ?, status = ?, verified_at = ?, updated_at = ?, attempts = ?, last_error = NULLIF(?, ''), next_retry_at = COALESCE(?, next_retry_at), last_checked_at = COALESCE(?, last_checked_at) WHERE id = ? AND status = 'pending'`

const queryDeleteOrganizationDomain = `DELETE FROM ` + btOrganizationDomains + ` WHERE id = ? AND organization_id = ?`

// buildIDINClause 把 picked ids 展开成 (?, ?, ...)；MySQL driver 不支持重复绑定同一占位符。
func buildIDINClause(ids []string) string {
	var b strings.Builder
	b.WriteByte('(')
	for i := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('?')
	}
	b.WriteByte(')')
	return b.String()
}

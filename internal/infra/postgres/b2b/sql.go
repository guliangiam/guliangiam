package b2b

// SQL constants for the organization_domains table (contracts/schema/10_tenant.yaml).
// All queries pass explicit organization_id to satisfy composite unique key
// and RLS predicate (rls: FORCE, predicate on organization_id).
//
// 列清单集中维护：新增续验预算字段（attempts/last_error/next_retry_at/
// last_checked_at）只在这里追加一次，避免 SELECT/RETURNING 漂移。
const orgDomainColumns = `id, organization_id, domain, verification_token, status, verified_at,
       created_at, updated_at, attempts, last_error, next_retry_at, last_checked_at`

const (
	QueryGetByID = `
SELECT ` + orgDomainColumns + `
FROM "guliangiam".organization_domains
WHERE id = $1::uuid`

	// QueryGetByIDForOrganization — 管理面 by-ID 读取；跨租户与不存在都无行。
	QueryGetByIDForOrganization = `
SELECT ` + orgDomainColumns + `
FROM "guliangiam".organization_domains
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	// GetByDomain — (organization_id, domain) 精确匹配。
	// 业务语义：同一组织下不能重复认领同一域名（唯一约束 uq_org_domain 兜底）。
	QueryGetByDomain = `
SELECT ` + orgDomainColumns + `
FROM "guliangiam".organization_domains
WHERE organization_id = $1::uuid AND domain = $2`

	// FindVerifiedByDomain — 邮箱域 SSO 发现入口。
	// domain 大小写不敏感精确匹配 + status=verified；读取至多两条以拒绝跨组织歧义。
	// 不带 organization_id 过滤 —— SSO discovery 需要跨租户查询。
	QueryFindVerifiedByDomain = `
SELECT ` + orgDomainColumns + `
FROM "guliangiam".organization_domains
WHERE LOWER(domain) = LOWER($1) AND status = 'verified'
LIMIT 2`

	// List — 按组织列出，支持 status 过滤。
	QueryList = `
SELECT ` + orgDomainColumns + `
FROM "guliangiam".organization_domains
WHERE organization_id = $1::uuid
  AND ($2 = '' OR status = $2)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4`

	QueryListCount = `
SELECT COUNT(*)
FROM "guliangiam".organization_domains
WHERE organization_id = $1::uuid
  AND ($2 = '' OR status = $2)`

	// ClaimPending — 单语句原子认领 + 租约。
	// 语义：只取 pending 且 next_retry_at <= now() 的行；按 next_retry_at/created_at
	// 排序保证公平；LIMIT $1 + FOR UPDATE SKIP LOCKED 让多实例互不重复。
	// 认领即把 next_retry_at 推到 now()+$2，防止实例在 DNS 查询期间被其他实例再次领取；
	// 若实例崩溃，租约到期后自动恢复可认领。$2 是 interval 文本（如 "300 seconds"）。
	QueryClaimPending = `
WITH claimed AS (
  SELECT id
  FROM "guliangiam".organization_domains
  WHERE status = 'pending' AND next_retry_at <= now()
  ORDER BY next_retry_at ASC, created_at ASC
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE "guliangiam".organization_domains d
SET next_retry_at = now() + $2::interval, updated_at = now()
FROM claimed c
WHERE d.id = c.id
RETURNING d.id, d.organization_id, d.domain, d.verification_token, d.status,
          d.verified_at, d.created_at, d.updated_at,
          d.attempts, d.last_error, d.next_retry_at, d.last_checked_at`

	QueryInsert = `
INSERT INTO "guliangiam".organization_domains
  (id, organization_id, domain, verification_token, status, verified_at, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)`

	// Update — 只允许 pending→pending/verified/failed；WHERE status='pending'
	// 兜底并发终态化（冲突方得到 UPDATE 0 → Conflict）。
	// last_error 空串写 NULL；next_retry_at/last_checked_at 传 NULL 时保留旧值
	// （成功路径不改变 next_retry_at；失败路径由 RecordVerificationFailure 显式赋值）。
	QueryUpdate = `
UPDATE "guliangiam".organization_domains
SET domain=$1, verification_token=$2, status=$3, verified_at=$4, updated_at=$5,
    attempts=$6,
    last_error=NULLIF($7::text, ''),
    next_retry_at=COALESCE($8::timestamptz, next_retry_at),
    last_checked_at=COALESCE($9::timestamptz, last_checked_at)
WHERE id=$10 AND status = 'pending'`

	QueryDelete = `
DELETE FROM "guliangiam".organization_domains
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)

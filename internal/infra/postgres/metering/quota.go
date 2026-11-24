package metering

import (
	"context"
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
	"github.com/jackc/pgx/v5"
)

const queryConsumeQuota = `UPDATE guliangiam.tenant_quotas
SET used_value = used_value + $3,
    updated_at = now(),
    revision = revision + 1
WHERE organization_id = $1::uuid
  AND quota_key = $2
  AND $3 > 0
  AND used_value + $3 <= limit_value
RETURNING used_value, limit_value`

const queryQuotaStatus = `SELECT used_value, limit_value
FROM guliangiam.tenant_quotas
WHERE organization_id = $1::uuid AND quota_key = $2`

// ConsumeQuota atomically advances one tenant quota counter. A missing row
// means the tenant has no quota for that key (unlimited). When the update
// matches nothing, a follow-up read distinguishes "no row" (allow) from
// "would exceed" (deny with the current counters).
//
// RLS: called inside the ingest UoW transaction, so the tenant context is
// already set; the explicit organization_id predicate remains the MySQL mirror
// contract.
func (r Repository) ConsumeQuota(ctx context.Context, organizationID, quotaKey string, delta int64) (domain.QuotaStatus, error) {
	if delta <= 0 {
		// 非正增量不消费配额（SQL 侧同样有 $3 > 0 守卫，BE-C-12）：
		// 直接放行，避免零增量走到“未命中即拒绝”的读回分支。
		return domain.QuotaStatus{Allowed: true}, nil
	}
	var used, limit int64
	err := r.db.QueryRow(ctx, queryConsumeQuota, organizationID, quotaKey, delta).Scan(&used, &limit)
	if err == nil {
		return domain.QuotaStatus{Allowed: true, Used: used, Limit: limit}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.QuotaStatus{}, err
	}
	var currentUsed, currentLimit int64
	if err := r.db.QueryRow(ctx, queryQuotaStatus, organizationID, quotaKey).Scan(&currentUsed, &currentLimit); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.QuotaStatus{Allowed: true}, nil
		}
		return domain.QuotaStatus{}, err
	}
	return domain.QuotaStatus{Allowed: false, Used: currentUsed, Limit: currentLimit}, nil
}

// compile-time capability assertion
var _ domain.QuotaConsumer = Repository{}

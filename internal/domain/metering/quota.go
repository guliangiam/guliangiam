package metering

import (
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// QuotaThreshold 表示一次扣减所处的阈值区间（设计：软上限告警 / 硬上限拒绝）。
type QuotaThreshold string

const (
	QuotaThresholdOK   QuotaThreshold = "ok"
	QuotaThresholdSoft QuotaThreshold = "soft"
	QuotaThresholdHard QuotaThreshold = "hard"
)

// PlanQuota 是套餐配额模板：hard_limit 必填，soft_limit 可空且必须 ≤ hard_limit。
type PlanQuota struct {
	OrganizationID string
	PlanID         string
	QuotaKey       string
	SoftLimit      *int64
	HardLimit      int64
	Unit           string
}

// Validate 对齐 ck_plan_quotas_hard_limit / ck_plan_quotas_soft_limit。
func (q PlanQuota) Validate() error {
	if q.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if q.PlanID == "" {
		return apperrors.NewInvalidArgument("planId is required")
	}
	if q.QuotaKey == "" {
		return apperrors.NewInvalidArgument("quotaKey is required")
	}
	if q.HardLimit < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "hardLimit must be non-negative",
			map[string]any{"field": "hardLimit", "value": q.HardLimit})
	}
	if q.SoftLimit != nil && (*q.SoftLimit < 0 || *q.SoftLimit > q.HardLimit) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "softLimit must be within [0, hardLimit]",
			map[string]any{"field": "softLimit", "value": *q.SoftLimit, "hardLimit": q.HardLimit})
	}
	return nil
}

// TenantQuota 是租户配额实例；扣减以 LimitValue 为硬上限，Revision 用于并发仲裁。
type TenantQuota struct {
	OrganizationID string
	QuotaKey       string
	LimitValue     int64
	UsedValue      int64
	Revision       int64
}

// QuotaDecision 是一次扣减的判定结果，供限流/配额协调层组装 429 + RateLimit-Policy。
type QuotaDecision struct {
	Allowed   bool
	Threshold QuotaThreshold
	Requested int64
	Used      int64
	Limit     int64
}

// Remaining 返回剩余可用量（不为负）。
func (q TenantQuota) Remaining() int64 {
	if q.UsedValue >= q.LimitValue {
		return 0
	}
	return q.LimitValue - q.UsedValue
}

// Consume 原子推进一次配额扣减：
//   - delta ≤ 0 → 参数错误（不改变用量）；
//   - used+delta > limit → 拒绝（429 + quota 维度元数据，用量不变）；
//   - 跨越软上限 → 允许但返回 soft 阈值（供告警）；
//   - 其余 → 允许并累加用量、revision++。
func (q *TenantQuota) Consume(delta int64) (QuotaDecision, error) {
	if delta <= 0 {
		return QuotaDecision{Allowed: false, Threshold: QuotaThresholdHard, Requested: delta, Used: q.UsedValue, Limit: q.LimitValue},
			apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "quota delta must be positive",
				map[string]any{"field": "delta", "value": delta})
	}
	decision := QuotaDecision{Requested: delta, Used: q.UsedValue, Limit: q.LimitValue}
	if q.UsedValue > q.LimitValue-delta { // 防溢出写法，等价 used+delta > limit
		decision.Allowed = false
		decision.Threshold = QuotaThresholdHard
		return decision, apperrors.NewWithMeta(apperrors.CodeTooManyRequests,
			"quota exceeded",
			map[string]any{"field": "quota", "quotaKey": q.QuotaKey, "used": q.UsedValue, "limit": q.LimitValue, "requested": delta})
	}
	q.UsedValue += delta
	q.Revision++
	decision.Allowed = true
	decision.Threshold = QuotaThresholdOK
	decision.Used = q.UsedValue
	return decision, nil
}

// ApplySoftThreshold 把套餐软上限并入判定结果：仅当本次扣减跨越软上限时降级为 soft。
func ApplySoftThreshold(decision QuotaDecision, softLimit *int64, previousUsed int64) QuotaDecision {
	if !decision.Allowed || softLimit == nil {
		return decision
	}
	if previousUsed < *softLimit && decision.Used >= *softLimit {
		decision.Threshold = QuotaThresholdSoft
	}
	return decision
}

// ValidateTenantQuota 对齐 ck_tenant_quotas_limit / ck_tenant_quotas_used。
func ValidateTenantQuota(quota TenantQuota) error {
	if quota.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if quota.QuotaKey == "" {
		return apperrors.NewInvalidArgument("quotaKey is required")
	}
	if quota.LimitValue < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "limitValue must be non-negative",
			map[string]any{"field": "limitValue", "value": quota.LimitValue})
	}
	if quota.UsedValue < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "usedValue must be non-negative",
			map[string]any{"field": "usedValue", "value": quota.UsedValue})
	}
	return nil
}

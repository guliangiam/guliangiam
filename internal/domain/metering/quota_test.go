package metering

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantQuota_ConsumeBoundaries(t *testing.T) {
	quota := TenantQuota{OrganizationID: "org-1", QuotaKey: "mau", LimitValue: 100, UsedValue: 90, Revision: 3}
	decision, err := quota.Consume(10)
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
	assert.Equal(t, QuotaThresholdOK, decision.Threshold)
	assert.Equal(t, int64(100), quota.UsedValue)
	assert.Equal(t, int64(4), quota.Revision)
	assert.Zero(t, quota.Remaining())

	decision, err = quota.Consume(1)
	require.Error(t, err)
	assert.False(t, decision.Allowed)
	assert.Equal(t, QuotaThresholdHard, decision.Threshold)
	assert.Equal(t, int64(100), quota.UsedValue, "denied consumption must not change usage")
	assert.Equal(t, int64(4), quota.Revision, "denied consumption must not bump revision")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeTooManyRequests, appErr.Code)
	assert.Equal(t, "mau", appErr.Meta["quotaKey"])
}

func TestTenantQuota_ConsumeInvalidDelta(t *testing.T) {
	for _, delta := range []int64{0, -1} {
		quota := TenantQuota{OrganizationID: "org-1", QuotaKey: "mau", LimitValue: 10}
		decision, err := quota.Consume(delta)
		require.Error(t, err)
		assert.False(t, decision.Allowed)
		assert.Zero(t, quota.UsedValue)
		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		// 4.14.1：值违反规则（已成功解析）→ 422（DEF-04）。
		assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	}
}

func TestTenantQuota_NoOverflowAtLargeLimit(t *testing.T) {
	quota := TenantQuota{LimitValue: 1 << 62, UsedValue: 1<<62 - 1}
	_, err := quota.Consume(1)
	require.NoError(t, err)
	assert.Equal(t, int64(1<<62), quota.UsedValue)
}

func TestApplySoftThreshold(t *testing.T) {
	soft := int64(80)
	decision := QuotaDecision{Allowed: true, Threshold: QuotaThresholdOK, Requested: 5, Used: 82, Limit: 100}
	got := ApplySoftThreshold(decision, &soft, 78)
	assert.Equal(t, QuotaThresholdSoft, got.Threshold, "crossing the soft limit must warn")

	got = ApplySoftThreshold(decision, &soft, 85)
	assert.Equal(t, QuotaThresholdOK, got.Threshold, "already above soft limit is not a crossing")

	got = ApplySoftThreshold(decision, nil, 0)
	assert.Equal(t, QuotaThresholdOK, got.Threshold)

	hard := QuotaDecision{Allowed: false, Threshold: QuotaThresholdHard}
	got = ApplySoftThreshold(hard, &soft, 0)
	assert.Equal(t, QuotaThresholdHard, got.Threshold)
}

func TestPlanQuotaValidate(t *testing.T) {
	soft := int64(50)
	require.NoError(t, PlanQuota{OrganizationID: "org-1", PlanID: "plan-1", QuotaKey: "mau", SoftLimit: &soft, HardLimit: 100}.Validate())

	for _, quota := range []PlanQuota{
		{OrganizationID: "org-1", PlanID: "plan-1", QuotaKey: "mau", HardLimit: -1},
		{OrganizationID: "org-1", PlanID: "plan-1", QuotaKey: "mau", SoftLimit: &soft, HardLimit: 10},
		{OrganizationID: "", PlanID: "plan-1", QuotaKey: "mau", HardLimit: 10},
	} {
		require.Error(t, quota.Validate())
	}
}

func TestValidateTenantQuota(t *testing.T) {
	require.NoError(t, ValidateTenantQuota(TenantQuota{OrganizationID: "org-1", QuotaKey: "mau", LimitValue: 10, UsedValue: 0}))
	require.Error(t, ValidateTenantQuota(TenantQuota{OrganizationID: "org-1", QuotaKey: "mau", LimitValue: -1}))
	require.Error(t, ValidateTenantQuota(TenantQuota{OrganizationID: "org-1", QuotaKey: "mau", UsedValue: -1}))
}

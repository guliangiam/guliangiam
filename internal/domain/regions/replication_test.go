package regions

import (
	"context"
	"fmt"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── DeploymentRegion.Validate ───────────────────────────────────

func TestDeploymentRegion_Validate_CodeTooLongAnswers422(t *testing.T) {
	r := &DeploymentRegion{RegionCode: "a-region-code-that-is-way-too-long"}
	err := r.Validate()
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "region_code", appErr.Meta["field"])
	assert.Equal(t, 32, appErr.Meta["max"])
}

// ── local stub: avoids import cycle (platform/regions → domain/regions) ──
type stubRegionResolver struct {
	data map[string]string
}

func (s *stubRegionResolver) Resolve(ctx context.Context, orgID string) (string, error) {
	if c2, ok := s.data[orgID]; ok {
		return c2, nil
	}
	return "", fmt.Errorf("not found: %s", orgID)
}

func (s *stubRegionResolver) ResolveBatch(ctx context.Context, orgIDs []string) (map[string]string, error) {
	r := make(map[string]string)
	for _, id := range orgIDs {
		if c2, ok := s.data[id]; ok {
			r[id] = c2
		}
	}
	return r, nil
}

func newStubResolver(m map[string]string) *stubRegionResolver {
	return &stubRegionResolver{data: m}
}

// ═══════════════════════════════════════════════
// DR-002: table → Tier → Policy classification
// ═══════════════════════════════════════════════

func TestClassify_IdentityTables(t *testing.T) {
	identityTables := []string{"users", "sessions", "tokens", "user_role_bindings"}
	for _, table := range identityTables {
		t.Run(table, func(t *testing.T) {
			tier, policy, err := Classify(table)
			require.NoError(t, err)
			assert.Equal(t, TierIdentity, tier)
			assert.Equal(t, PolicyNeverCross, policy)
		})
	}
}

func TestClassify_ConfigTables(t *testing.T) {
	tier, policy, err := Classify("organizations")
	require.NoError(t, err)
	assert.Equal(t, TierConfig, tier)
	assert.Equal(t, PolicyQuasiRealtime, policy)
}

func TestClassify_AuditTables(t *testing.T) {
	tier, policy, err := Classify("records")
	require.NoError(t, err)
	assert.Equal(t, TierAudit, tier)
	assert.Equal(t, PolicyAsyncAggregate, policy)
}

func TestClassify_UnknownTable(t *testing.T) {
	_, _, err := Classify("nonexistent_table")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownTable)
}

// ── DR-002 + Decision 1 combined interception ──

func TestRegionResidentPolicy_NeverCross_CrossRegion_Blocked(t *testing.T) {
	resolver := newStubResolver(map[string]string{
		"org-cn": "cn-north-1",
		"org-us": "us-east-1",
	})
	policy := NewRegionResidentPolicy(NewCrossRegionGuard(resolver))

	err := policy.CanReplicate(context.Background(), "users", "org-cn", "org-us", "etl.migration")
	require.Error(t, err)

	ve, ok := IsReplicationViolation(err)
	require.True(t, ok)
	assert.Equal(t, "users", ve.Table)
	assert.Equal(t, TierIdentity, ve.Tier)
	assert.Equal(t, PolicyNeverCross, ve.Policy)
	assert.Contains(t, ve.Reason, "never-cross")
}

func TestRegionResidentPolicy_NeverCross_SameRegion_Allowed(t *testing.T) {
	resolver := newStubResolver(map[string]string{
		"org-cn-1": "cn-north-1",
		"org-cn-2": "cn-north-1",
	})
	policy := NewRegionResidentPolicy(NewCrossRegionGuard(resolver))

	err := policy.CanReplicate(context.Background(), "users", "org-cn-1", "org-cn-2", "etl.migration")
	assert.NoError(t, err)
}

func TestRegionResidentPolicy_QuasiRealtime_CrossRegion_Allowed(t *testing.T) {
	resolver := newStubResolver(map[string]string{
		"org-cn": "cn-north-1",
		"org-us": "us-east-1",
	})
	policy := NewRegionResidentPolicy(NewCrossRegionGuard(resolver))

	// TierConfig quasi-realtime → domain layer doesn't block
	err := policy.CanReplicate(context.Background(), "organizations", "org-cn", "org-us", "config.sync")
	assert.NoError(t, err, "TierConfig quasi-realtime should allow cross-region")
}

func TestRegionResidentPolicy_AsyncAggregate_CrossRegion_Allowed(t *testing.T) {
	resolver := newStubResolver(map[string]string{
		"org-cn": "cn-north-1",
		"org-us": "us-east-1",
	})
	policy := NewRegionResidentPolicy(NewCrossRegionGuard(resolver))

	// TierAudit async-aggregate → allowed by policy
	err := policy.CanReplicate(context.Background(), "records", "org-cn", "org-us", "audit.aggregate")
	assert.NoError(t, err, "TierAudit async-aggregate should allow cross-region")
}

func TestRegionResidentPolicy_SameOrg_AlwaysAllowed(t *testing.T) {
	resolver := newStubResolver(nil)
	policy := NewRegionResidentPolicy(NewCrossRegionGuard(resolver))

	// Same org — never blocked
	err := policy.CanReplicate(context.Background(), "users", "org-x", "org-x", "any.op")
	assert.NoError(t, err)
}

func TestRegionResidentPolicy_UnknownTable_DefaultNeverCross(t *testing.T) {
	resolver := newStubResolver(map[string]string{
		"org-cn": "cn-north-1",
		"org-us": "us-east-1",
	})
	policy := NewRegionResidentPolicy(NewCrossRegionGuard(resolver))

	err := policy.CanReplicate(context.Background(), "custom_plugin_table", "org-cn", "org-us", "etl")
	require.Error(t, err)
	ve, ok := IsReplicationViolation(err)
	require.True(t, ok)
	assert.Equal(t, PolicyNeverCross, ve.Policy, "unknown table defaults to never-cross (safe default)")
}

// ═══════════════════════════════════════════════
// DR-005: three-tier RTO/RPO
// ═══════════════════════════════════════════════

func TestRecoveryTierCritical_Target(t *testing.T) {
	rto, rpo := RecoveryTierCritical.TargetMinutes()
	assert.Equal(t, 1, rto, "L1 RTO < 1min")
	assert.Equal(t, -1, rpo, "L1 RPO = 0")
}

func TestRecoveryTierImportant_Target(t *testing.T) {
	rto, rpo := RecoveryTierImportant.TargetMinutes()
	assert.Equal(t, 5, rto, "L2 RTO < 5min")
	assert.Equal(t, 5, rpo, "L2 RPO < 5min")
}

func TestRecoveryTierStandard_Target(t *testing.T) {
	rto, rpo := RecoveryTierStandard.TargetMinutes()
	assert.Equal(t, 240, rto, "L3 RTO < 4h")
	assert.Equal(t, 1440, rpo, "L3 RPO < 24h")
}

func TestTierRecoveryDefaults_Identity_IsCritical(t *testing.T) {
	assert.Equal(t, RecoveryTierCritical, TierRecoveryDefaults[TierIdentity])
}

func TestTierRecoveryDefaults_Audit_IsCritical(t *testing.T) {
	assert.Equal(t, RecoveryTierCritical, TierRecoveryDefaults[TierAudit])
}

func TestTierRecoveryDefaults_Config_IsImportant(t *testing.T) {
	assert.Equal(t, RecoveryTierImportant, TierRecoveryDefaults[TierConfig])
}

func TestTierRecoveryDefaults_Billing_IsStandard(t *testing.T) {
	assert.Equal(t, RecoveryTierStandard, TierRecoveryDefaults[TierBilling])
}

// ── boundary tests ──

func TestTier_String_AllKnown(t *testing.T) {
	for _, t2 := range []Tier{TierIdentity, TierConfig, TierAudit, TierBilling, TierSystem} {
		assert.NotContains(t, t2.String(), "unknown", "Tier %d should have a known string")
	}
}

func TestReplicationPolicy_String_AllKnown(t *testing.T) {
	for _, p := range []ReplicationPolicy{PolicyNeverCross, PolicyQuasiRealtime, PolicyAsyncAggregate} {
		assert.NotContains(t, p.String(), "unknown")
	}
}

func TestPolicyOf_KnownTier(t *testing.T) {
	p, err := PolicyOf(TierIdentity)
	require.NoError(t, err)
	assert.Equal(t, PolicyNeverCross, p)
}

func TestPolicyOf_UnknownTier(t *testing.T) {
	_, err := PolicyOf(Tier(99))
	require.Error(t, err)
}

func TestDeploymentRegion_Validate_CountryCodeAnswers422(t *testing.T) {
	region := &DeploymentRegion{RegionCode: "cn", CountryCode: "CHN"}
	appErr := apperrors.FromError(region.Validate())
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "country_code", appErr.Meta["field"])
}

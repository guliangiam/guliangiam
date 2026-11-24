package auth

import (
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConcurrentSessionPolicy(t *testing.T) {
	policy := defaultConcurrentSessionPolicy()
	assert.Equal(t, 0, policy.MaxSessions)
	assert.Equal(t, "revoke_oldest", policy.Action)
}

func TestResolveConcurrentSessionPolicy_NilOrg(t *testing.T) {
	policy := resolveConcurrentSessionPolicy(nil)
	assert.Equal(t, 0, policy.MaxSessions)
	assert.Equal(t, "revoke_oldest", policy.Action)
}

func TestResolveConcurrentSessionPolicy_EmptyPolicy(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{ConcurrentSessionPolicy: nil},
	}
	policy := resolveConcurrentSessionPolicy(org)
	assert.Equal(t, 0, policy.MaxSessions)
}

func TestResolveConcurrentSessionPolicy_WithValues(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			ConcurrentSessionPolicy: map[string]any{
				"maxSessions": 5,
				"action":      "reject_new",
			},
		},
	}
	policy := resolveConcurrentSessionPolicy(org)
	assert.Equal(t, 5, policy.MaxSessions)
	assert.Equal(t, "reject_new", policy.Action)
}

func TestResolveConcurrentSessionPolicy_EmptyAction(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			ConcurrentSessionPolicy: map[string]any{
				"maxSessions": 3,
				"action":      "",
			},
		},
	}
	policy := resolveConcurrentSessionPolicy(org)
	assert.Equal(t, 3, policy.MaxSessions)
	// Empty action should keep default
	assert.Equal(t, "revoke_oldest", policy.Action)
}

func newPolicyOrg(policy map[string]any) *orgdomain.Organization {
	return &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{ConcurrentSessionPolicy: policy},
	}
}

// 目标态键（《开发设计》22.5）与历史键的解析口径：目标态键优先，历史键保持兼容，
// 无法识别的 concurrentSessionPolicy 按 limit（从严）处理。
func TestResolveConcurrentSessionPolicy_TargetKeys(t *testing.T) {
	tests := []struct {
		name       string
		policy     map[string]any
		wantMax    int
		wantAction string
	}{
		{"unlimited", map[string]any{"concurrentSessionPolicy": "unlimited"}, 0, "revoke_oldest"},
		{"single", map[string]any{"concurrentSessionPolicy": "single"}, 1, "revoke_oldest"},
		{"limit_with_bound", map[string]any{"concurrentSessionPolicy": "limit", "maxActiveSessions": 3}, 3, "revoke_oldest"},
		{"limit_default_bound", map[string]any{"concurrentSessionPolicy": "limit"}, 10, "revoke_oldest"},
		{"limit_legacy_bound", map[string]any{"concurrentSessionPolicy": "limit", "maxSessions": 4}, 4, "revoke_oldest"},
		{"target_kind_wins_over_legacy_bound", map[string]any{"concurrentSessionPolicy": "unlimited", "maxSessions": 5}, 0, "revoke_oldest"},
		{"evict_oldest_maps_to_revoke_oldest", map[string]any{"action": "evict_oldest"}, 0, "revoke_oldest"},
		{"action_case_insensitive", map[string]any{"maxSessions": 2, "action": "REJECT_NEW"}, 2, "reject_new"},
		{"require_mfa_is_recognised", map[string]any{"concurrentSessionPolicy": "limit", "action": "require_mfa"}, 10, "require_mfa"},
		{"unknown_kind_is_treated_as_limit", map[string]any{"concurrentSessionPolicy": "weird"}, 10, "revoke_oldest"},
		{"unknown_action_keeps_default", map[string]any{"maxSessions": 2, "action": "nuke"}, 2, "revoke_oldest"},
		{"legacy_keys_unchanged", map[string]any{"maxSessions": 5, "action": "reject_new"}, 5, "reject_new"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := resolveConcurrentSessionPolicy(newPolicyOrg(tt.policy))
			assert.Equal(t, tt.wantMax, policy.MaxSessions)
			assert.Equal(t, tt.wantAction, policy.Action)
		})
	}
}

func TestStrFromMap_Nil(t *testing.T) {
	v, ok := strFromMap(nil, "key")
	assert.False(t, ok)
	assert.Equal(t, "", v)
}

func TestStrFromMap_MissingKey(t *testing.T) {
	v, ok := strFromMap(map[string]any{}, "key")
	assert.False(t, ok)
	assert.Equal(t, "", v)
}

func TestStrFromMap_StringValue(t *testing.T) {
	v, ok := strFromMap(map[string]any{"key": "value"}, "key")
	assert.True(t, ok)
	assert.Equal(t, "value", v)
}

func TestStrFromMap_NonStringValue(t *testing.T) {
	v, ok := strFromMap(map[string]any{"key": 123}, "key")
	assert.False(t, ok)
	assert.Equal(t, "", v)
}

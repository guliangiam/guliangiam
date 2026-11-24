package organizations

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePolicy_InheritOverrideLock(t *testing.T) {
	chain := []PolicyChainNode{
		{
			OrganizationID: "root",
			Values:         map[string]any{"passwordPolicy": map[string]any{"minLength": 12}},
		},
		{
			OrganizationID: "child",
			Rules:          map[string]PolicyInheritanceRule{"passwordPolicy": {PolicyKey: "passwordPolicy", Mode: PolicyModeInherit}},
		},
		{
			OrganizationID: "grandchild",
			Rules:          map[string]PolicyInheritanceRule{"passwordPolicy": {PolicyKey: "passwordPolicy", Mode: PolicyModeOverride}},
			Values:         map[string]any{"passwordPolicy": map[string]any{"minLength": 16}},
		},
	}
	resolution, err := ResolvePolicy(chain, "passwordPolicy")
	require.NoError(t, err)
	assert.Equal(t, "grandchild", resolution.EffectiveSourceOrganizationID)
	assert.Equal(t, 16, resolution.EffectiveValue.(map[string]any)["minLength"])
	assert.False(t, resolution.Locked)
}

func TestResolvePolicy_LockRejectsDeeperOverride(t *testing.T) {
	chain := []PolicyChainNode{
		{
			OrganizationID: "root",
			Rules:          map[string]PolicyInheritanceRule{"passwordPolicy": {PolicyKey: "passwordPolicy", Mode: PolicyModeLock}},
			Values:         map[string]any{"passwordPolicy": map[string]any{"minLength": 14}},
		},
		{
			OrganizationID: "child",
			Rules:          map[string]PolicyInheritanceRule{"passwordPolicy": {PolicyKey: "passwordPolicy", Mode: PolicyModeOverride}},
			Values:         map[string]any{"passwordPolicy": map[string]any{"minLength": 6}},
		},
	}
	resolution, err := ResolvePolicy(chain, "passwordPolicy")
	require.NoError(t, err)
	assert.True(t, resolution.Locked)
	assert.Equal(t, "root", resolution.LockedByOrganizationID)
	assert.Equal(t, "root", resolution.EffectiveSourceOrganizationID)
	assert.Equal(t, 14, resolution.EffectiveValue.(map[string]any)["minLength"])
	require.Len(t, resolution.Steps, 2)
	assert.True(t, resolution.Steps[1].Rejected)
}

func TestResolvePolicy_LockUsesSourceOrganization(t *testing.T) {
	chain := []PolicyChainNode{
		{OrganizationID: "root", Values: map[string]any{"passwordPolicy": map[string]any{"minLength": 14}}},
		{
			OrganizationID: "child",
			Rules: map[string]PolicyInheritanceRule{"passwordPolicy": {
				PolicyKey: "passwordPolicy", Mode: PolicyModeLock, SourceOrganizationID: "root",
			}},
		},
	}
	resolution, err := ResolvePolicy(chain, "passwordPolicy")
	require.NoError(t, err)
	assert.True(t, resolution.Locked)
	assert.Equal(t, "root", resolution.EffectiveSourceOrganizationID)
	assert.Equal(t, 14, resolution.EffectiveValue.(map[string]any)["minLength"])
}

func TestResolvePolicy_Validation(t *testing.T) {
	_, err := ResolvePolicy(nil, "passwordPolicy")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)

	_, err = ResolvePolicy([]PolicyChainNode{{OrganizationID: "root"}}, "")
	require.Error(t, err)
}

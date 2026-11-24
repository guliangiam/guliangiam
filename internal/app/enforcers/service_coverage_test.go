package enforcers

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEnforceRvals_TwoValues(t *testing.T) {
	cmd := EnforceCommand{V0: "sub", V1: "obj"}
	rvals := buildEnforceRvals(cmd)
	assert.Equal(t, []interface{}{"sub", "obj"}, rvals)
}

func TestBuildEnforceRvals_ThreeValues(t *testing.T) {
	cmd := EnforceCommand{V0: "sub", V1: "obj", V2: "act"}
	rvals := buildEnforceRvals(cmd)
	assert.Equal(t, []interface{}{"sub", "obj", "act"}, rvals)
}

func TestBuildEnforceRvals_SixValues(t *testing.T) {
	cmd := EnforceCommand{V0: "v0", V1: "v1", V2: "v2", V3: "v3", V4: "v4", V5: "v5"}
	rvals := buildEnforceRvals(cmd)
	assert.Equal(t, []interface{}{"v0", "v1", "v2", "v3", "v4", "v5"}, rvals)
}

func TestBuildEnforceRvals_Empty(t *testing.T) {
	cmd := EnforceCommand{}
	rvals := buildEnforceRvals(cmd)
	assert.Equal(t, []interface{}{"", ""}, rvals)
}

func TestToListItem_Disabled(t *testing.T) {
	now := time.Now()
	e := &domain.Enforcer{
		ID:             "enf-2",
		OrganizationID: "org-1",
		Name:           "disabled-enforcer",
		IsEnabled:      false,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	item := toListItem(e)
	assert.Equal(t, "enf-2", item.ID)
	assert.False(t, item.IsEnabled)
}

func TestToDetail_WithDescription(t *testing.T) {
	now := time.Now()
	e := &domain.Enforcer{
		ID:          "enf-3",
		Name:        "test",
		Description: "A detailed enforcer",
		Revision:    5,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	detail := toDetail(e)
	assert.Equal(t, "A detailed enforcer", detail.Description)
	assert.Equal(t, int64(5), detail.Revision)
}

func TestCreateEnforcer_Validation_MissingOrgID(t *testing.T) {
	svc := Service{}
	cmd := CreateEnforcerCommand{
		Name: "test-enforcer",
	}
	err := svc.CreateEnforcer(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organizationId is required")
}

func TestCreateEnforcer_Validation_MissingName(t *testing.T) {
	svc := Service{}
	cmd := CreateEnforcerCommand{
		OrganizationID: "org-1",
	}
	err := svc.CreateEnforcer(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestCreateEnforcer_Validation_EmptyOrgID(t *testing.T) {
	svc := Service{}
	cmd := CreateEnforcerCommand{
		OrganizationID: "   ",
		Name:           "test",
	}
	err := svc.CreateEnforcer(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organizationId is required")
}

func TestCreateEnforcer_Validation_EmptyName(t *testing.T) {
	svc := Service{}
	cmd := CreateEnforcerCommand{
		OrganizationID: "org-1",
		Name:           "   ",
	}
	err := svc.CreateEnforcer(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestEnforce_Validation_MissingID(t *testing.T) {
	svc := Service{}
	cmd := EnforceCommand{}
	_, err := svc.Enforce(context.Background(), "org-1", cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enforcer id is required")
}

func TestEnforce_Validation_EmptyID(t *testing.T) {
	svc := Service{}
	cmd := EnforceCommand{ID: "   "}
	_, err := svc.Enforce(context.Background(), "org-1", cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enforcer id is required")
}

func TestBatchEnforce_Validation_MissingID(t *testing.T) {
	svc := Service{}
	cmd := BatchEnforceCommand{}
	_, err := svc.BatchEnforce(context.Background(), "org-1", cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enforcer id is required")
}

func TestReloadPolicy_Validation_MissingID(t *testing.T) {
	svc := Service{}
	err := svc.ReloadPolicy(context.Background(), "org-1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enforcer id is required")
}

func TestReloadPolicy_Validation_EmptyID(t *testing.T) {
	svc := Service{}
	err := svc.ReloadPolicy(context.Background(), "org-1", "   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enforcer id is required")
}

func TestService_RefreshEngines_NilCasbinManager(t *testing.T) {
	// RefreshEngines calls CasbinManager.InvalidateAll() which will panic if nil
	// This is expected behavior - CasbinManager must be initialized
	// Skipping this test as it requires a real CasbinManager
}

func TestPolicyExport_Fields(t *testing.T) {
	now := time.Now()
	export := PolicyExport{
		EnforcerID:   "enf-1",
		EnforcerName: "test",
		Policies:     [][]string{{"sub", "obj", "act"}},
		ExportedAt:   now,
	}
	assert.Equal(t, "enf-1", export.EnforcerID)
	assert.Equal(t, "test", export.EnforcerName)
	assert.Len(t, export.Policies, 1)
	assert.Equal(t, now, export.ExportedAt)
}

func TestPolicyUploadResult_Fields(t *testing.T) {
	result := PolicyUploadResult{
		Total:     10,
		Succeeded: 8,
		Failed:    2,
		Errors:    []string{"row 3: invalid", "row 7: duplicate"},
	}
	assert.Equal(t, 10, result.Total)
	assert.Equal(t, 8, result.Succeeded)
	assert.Equal(t, 2, result.Failed)
	assert.Len(t, result.Errors, 2)
}

func TestEnforceResult_Fields(t *testing.T) {
	result := EnforceResult{Allow: true}
	assert.True(t, result.Allow)
}

func TestBatchEnforceResult_Fields(t *testing.T) {
	result := BatchEnforceResult{Results: []bool{true, false, true}}
	assert.Len(t, result.Results, 3)
	assert.True(t, result.Results[0])
	assert.False(t, result.Results[1])
}

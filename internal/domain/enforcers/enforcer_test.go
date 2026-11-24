package enforcers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEnforcer_Fields(t *testing.T) {
	now := time.Now()

	e := Enforcer{
		ID:             "enf-001",
		OrganizationID: "org-001",
		Name:           "rbac-enforcer",
		DisplayName:    "RBAC Enforcer",
		Description:    "Main RBAC enforcer for org",
		ModelID:        "model-001",
		AdapterID:      "adapter-001",
		IsEnabled:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       3,
		Owner:          "admin-1",
	}

	assert.Equal(t, "enf-001", e.ID)
	assert.Equal(t, "org-001", e.OrganizationID)
	assert.Equal(t, "rbac-enforcer", e.Name)
	assert.Equal(t, "RBAC Enforcer", e.DisplayName)
	assert.Equal(t, "Main RBAC enforcer for org", e.Description)
	assert.Equal(t, "model-001", e.ModelID)
	assert.Equal(t, "adapter-001", e.AdapterID)
	assert.True(t, e.IsEnabled)
	assert.Equal(t, now, e.CreatedAt)
	assert.Equal(t, now, e.UpdatedAt)
	assert.Equal(t, int64(3), e.Revision)
	assert.Equal(t, "admin-1", e.Owner)
}

func TestEnforcer_ZeroValue(t *testing.T) {
	var e Enforcer
	assert.Equal(t, "", e.ID)
	assert.False(t, e.IsEnabled)
	assert.Equal(t, int64(0), e.Revision)
	assert.Equal(t, "", e.Owner)
}

func TestEnforcer_Disabled(t *testing.T) {
	e := Enforcer{
		ID:        "enf-002",
		IsEnabled: false,
	}
	assert.False(t, e.IsEnabled)
}

func TestPolicy_Fields(t *testing.T) {
	p := Policy{
		PType: "p",
		V0:    "sub",
		V1:    "dom",
		V2:    "obj",
		V3:    "act",
		V4:    "",
		V5:    "",
	}
	assert.Equal(t, "p", p.PType)
	assert.Equal(t, "sub", p.V0)
	assert.Equal(t, "dom", p.V1)
	assert.Equal(t, "obj", p.V2)
	assert.Equal(t, "act", p.V3)
	assert.Equal(t, "", p.V4)
	assert.Equal(t, "", p.V5)
}

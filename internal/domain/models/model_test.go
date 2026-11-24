package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestModel_Creation(t *testing.T) {
	now := time.Now()
	m := Model{
		ID:             "model-001",
		OrganizationID: "org-001",
		Name:           "rbac-model",
		DisplayName:    "RBAC Model",
		Description:    "Role-based access control model",
		ModelText: `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act`,
		Owner:     "admin",
		IsEnabled: true,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}

	assert.Equal(t, "model-001", m.ID)
	assert.Equal(t, "rbac-model", m.Name)
	assert.True(t, m.IsEnabled)
	assert.Contains(t, m.ModelText, "request_definition")
	assert.Contains(t, m.ModelText, "role_definition")
}

func TestModel_Disabled(t *testing.T) {
	m := Model{Name: "disabled-model", IsEnabled: false}
	assert.False(t, m.IsEnabled)
}

func TestModel_ModelTextFormat(t *testing.T) {
	// Verify that the model text follows Casbin INI format
	m := Model{
		ModelText: `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act`,
	}
	assert.Contains(t, m.ModelText, "[request_definition]")
	assert.Contains(t, m.ModelText, "[policy_definition]")
}

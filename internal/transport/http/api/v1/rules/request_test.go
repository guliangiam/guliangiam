package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRuleRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		isEnabled := true
		req := CreateRuleRequest{
			OrganizationID: "org-001",
			Name:           "email-domain-rule",
			Scope:          "user",
			RuleType:       "condition",
			Conditions:     []map[string]any{{"field": "email", "op": "contains", "value": "@example.com"}},
			Action:         "allow",
			Priority:       10,
			IsEnabled:      &isEnabled,
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "email-domain-rule", cmd.Name)
		assert.Equal(t, "condition", cmd.RuleType)
		assert.Len(t, cmd.Conditions, 1)
		assert.Equal(t, "allow", cmd.Action)
		assert.Equal(t, 10, cmd.Priority)
		assert.True(t, *cmd.IsEnabled)
	})
}

func TestPatchRuleRequest_ToCommand(t *testing.T) {
	t.Run("with_nil_conditions", func(t *testing.T) {
		name := "updated-rule"
		req := PatchRuleRequest{
			Name:       &name,
			Conditions: nil,
			Revision:   2,
		}

		cmd := req.toCommand("rule-001")

		assert.Equal(t, "rule-001", cmd.ID)
		assert.Equal(t, &name, cmd.Name)
		assert.Nil(t, cmd.Conditions)
	})

	t.Run("with_conditions", func(t *testing.T) {
		req := PatchRuleRequest{
			Conditions: []map[string]any{{"field": "ip", "op": "in", "value": "192.168.0.0/16"}},
		}

		cmd := req.toCommand("rule-002")

		assert.Len(t, cmd.Conditions, 1)
		assert.Equal(t, "ip", cmd.Conditions[0]["field"])
	})
}

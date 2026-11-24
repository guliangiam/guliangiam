package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCreateRuleCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateRuleCommand{
			OrganizationID: "org-001",
			Name:           "email-rule",
			Scope:          "user",
			RuleType:       "condition",
			Conditions:     []map[string]any{{"field": "email", "operator": "eq", "value": "a@b.c"}},
			Action:         "allow",
		}
		assert.NoError(t, validateCreateRuleCommand(cmd))
	})

	t.Run("missing_organization_id", func(t *testing.T) {
		cmd := CreateRuleCommand{Name: "rule", Scope: "user", RuleType: "cond", Action: "allow"}
		assert.Error(t, validateCreateRuleCommand(cmd))
	})

	t.Run("missing_name", func(t *testing.T) {
		cmd := CreateRuleCommand{OrganizationID: "org-001", Scope: "user", RuleType: "cond", Action: "allow"}
		assert.Error(t, validateCreateRuleCommand(cmd))
	})

	t.Run("missing_scope", func(t *testing.T) {
		cmd := CreateRuleCommand{OrganizationID: "org-001", Name: "rule", RuleType: "cond", Action: "allow"}
		assert.Error(t, validateCreateRuleCommand(cmd))
	})

	t.Run("missing_rule_type", func(t *testing.T) {
		cmd := CreateRuleCommand{OrganizationID: "org-001", Name: "rule", Scope: "user", Action: "allow"}
		assert.Error(t, validateCreateRuleCommand(cmd))
	})

	t.Run("missing_action", func(t *testing.T) {
		cmd := CreateRuleCommand{OrganizationID: "org-001", Name: "rule", Scope: "user", RuleType: "cond"}
		assert.Error(t, validateCreateRuleCommand(cmd))
	})
}

func TestValidatePatchRuleCommand(t *testing.T) {
	t.Run("valid_all_nil", func(t *testing.T) {
		cmd := PatchRuleCommand{}
		assert.NoError(t, validatePatchRuleCommand(cmd))
	})

	t.Run("blank_name", func(t *testing.T) {
		blank := ""
		cmd := PatchRuleCommand{Name: &blank}
		assert.Error(t, validatePatchRuleCommand(cmd))
	})

	t.Run("blank_scope", func(t *testing.T) {
		blank := ""
		cmd := PatchRuleCommand{Scope: &blank}
		assert.Error(t, validatePatchRuleCommand(cmd))
	})

	t.Run("blank_rule_type", func(t *testing.T) {
		blank := ""
		cmd := PatchRuleCommand{RuleType: &blank}
		assert.Error(t, validatePatchRuleCommand(cmd))
	})

	t.Run("blank_action", func(t *testing.T) {
		blank := ""
		cmd := PatchRuleCommand{Action: &blank}
		assert.Error(t, validatePatchRuleCommand(cmd))
	})

	t.Run("valid_values", func(t *testing.T) {
		name := "updated-rule"
		scope := "global"
		ruleType := "condition"
		action := "deny"
		cmd := PatchRuleCommand{Name: &name, Scope: &scope, RuleType: &ruleType, Action: &action}
		assert.NoError(t, validatePatchRuleCommand(cmd))
	})
}

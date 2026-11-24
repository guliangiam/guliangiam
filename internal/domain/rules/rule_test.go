package rules

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRule_Creation(t *testing.T) {
	now := time.Now()
	r := Rule{
		ID:             "rule-001",
		OrganizationID: "org-001",
		Name:           "block-malicious-ips",
		Scope:          "org",
		RuleType:       "IP",
		Conditions: []map[string]any{
			{"type": "cidr", "value": "10.0.0.0/8", "action": "Block"},
		},
		Action:    "Block",
		Priority:  100,
		IsEnabled: true,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}

	assert.Equal(t, "rule-001", r.ID)
	assert.Equal(t, "IP", r.RuleType)
	assert.Equal(t, "Block", r.Action)
	assert.True(t, r.IsEnabled)
	assert.Equal(t, 100, r.Priority)
}

func TestRule_Types(t *testing.T) {
	validTypes := map[string]bool{
		"IP":   true,
		"UA":   true,
		"Rate": true,
		"WAF":  true,
	}

	for ruleType, valid := range validTypes {
		t.Run("type_"+ruleType, func(t *testing.T) {
			r := Rule{RuleType: ruleType}
			_, ok := validTypes[r.RuleType]
			assert.Equal(t, valid, ok)
		})
	}
}

func TestRule_Actions(t *testing.T) {
	validActions := map[string]bool{
		"Allow":   true,
		"Block":   true,
		"Captcha": true,
	}

	tests := []struct {
		name   string
		action string
		valid  bool
	}{
		{name: "allow", action: "Allow", valid: true},
		{name: "block", action: "Block", valid: true},
		{name: "captcha", action: "Captcha", valid: true},
		{name: "invalid", action: "Unknown", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := validActions[tt.action]
			assert.Equal(t, tt.valid, ok)
		})
	}
}

func TestRule_Scope(t *testing.T) {
	tests := []struct {
		name  string
		scope string
	}{
		{name: "org_scope", scope: "org"},
		{name: "application_scope", scope: "application"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Rule{Scope: tt.scope}
			assert.Equal(t, tt.scope, r.Scope)
		})
	}
}

func TestRule_PriorityOrdering(t *testing.T) {
	r1 := Rule{Name: "high-priority", Priority: 100}
	r2 := Rule{Name: "low-priority", Priority: 10}

	assert.True(t, r1.Priority > r2.Priority, "higher priority value should be processed first")
}

func TestRule_Disabled(t *testing.T) {
	r := Rule{Name: "disabled-rule", IsEnabled: false}
	assert.False(t, r.IsEnabled, "disabled rule should not be active")
}

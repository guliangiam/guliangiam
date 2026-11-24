package rules

import (
	"encoding/json"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToRule(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		conditions := []map[string]any{
			{"field": "email", "operator": "contains", "value": "@example.com"},
		}
		conditionsJSON, err := json.Marshal(conditions)
		require.NoError(t, err)

		row := ruleRow{
			ID:             "rule-001",
			OrganizationID: "org-001",
			Name:           "email-domain-rule",
			Scope:          "user",
			RuleType:       "condition",
			Conditions:     conditionsJSON,
			Action:         "allow",
			Priority:       10,
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		result, err := toRule(row)
		require.NoError(t, err)

		expected := &domain.Rule{
			ID:             "rule-001",
			OrganizationID: "org-001",
			Name:           "email-domain-rule",
			Scope:          "user",
			RuleType:       "condition",
			Conditions:     conditions,
			Action:         "allow",
			Priority:       10,
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		assert.Equal(t, expected, result)
	})

	t.Run("nil_conditions_becomes_empty_slice", func(t *testing.T) {
		now := time.Now()
		row := ruleRow{
			ID:             "rule-002",
			OrganizationID: "org-001",
			Name:           "no-conditions-rule",
			Conditions:     nil,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result, err := toRule(row)
		require.NoError(t, err)

		assert.Equal(t, []map[string]any{}, result.Conditions)
	})

	t.Run("empty_json_conditions", func(t *testing.T) {
		now := time.Now()
		row := ruleRow{
			ID:             "rule-003",
			OrganizationID: "org-001",
			Name:           "empty-conditions",
			Conditions:     []byte(`[]`),
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result, err := toRule(row)
		require.NoError(t, err)

		assert.Equal(t, []map[string]any{}, result.Conditions)
	})

	t.Run("invalid_json_returns_error", func(t *testing.T) {
		row := ruleRow{
			ID:             "rule-004",
			OrganizationID: "org-001",
			Name:           "bad-rule",
			Conditions:     []byte(`{invalid}`),
		}

		_, err := toRule(row)
		assert.Error(t, err)
	})
}

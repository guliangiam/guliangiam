package enforcers

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	"github.com/stretchr/testify/assert"
)

func TestToListItem(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		e := &domain.Enforcer{
			ID:             "enf-001",
			OrganizationID: "org-001",
			Name:           "rbac-enforcer",
			DisplayName:    "RBAC Enforcer",
			ModelID:        "model-001",
			AdapterID:      "adapter-001",
			IsEnabled:      true,
			Owner:          "admin",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toListItem(e)

		assert.Equal(t, "enf-001", result.ID)
		assert.Equal(t, "rbac-enforcer", result.Name)
		assert.Equal(t, "model-001", result.ModelID)
		assert.True(t, result.IsEnabled)
	})
}

func TestToDetail(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		e := &domain.Enforcer{
			ID:             "enf-001",
			OrganizationID: "org-001",
			Name:           "rbac-enforcer",
			DisplayName:    "RBAC Enforcer",
			Description:    "Role-based access control",
			Revision:       2,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toDetail(e)

		assert.Equal(t, "enf-001", result.ID)
		assert.Equal(t, "Role-based access control", result.Description)
		assert.Equal(t, int64(2), result.Revision)
	})
}

func TestBuildEnforceRvals(t *testing.T) {
	t.Run("two_values", func(t *testing.T) {
		cmd := EnforceCommand{V0: "alice", V1: "data1", V2: "", V3: "", V4: "", V5: ""}
		result := buildEnforceRvals(cmd)
		assert.Equal(t, []interface{}{"alice", "data1"}, result)
	})

	t.Run("three_values", func(t *testing.T) {
		cmd := EnforceCommand{V0: "alice", V1: "data1", V2: "read"}
		result := buildEnforceRvals(cmd)
		assert.Equal(t, []interface{}{"alice", "data1", "read"}, result)
	})

	t.Run("six_values", func(t *testing.T) {
		cmd := EnforceCommand{V0: "a", V1: "b", V2: "c", V3: "d", V4: "e", V5: "f"}
		result := buildEnforceRvals(cmd)
		assert.Equal(t, []interface{}{"a", "b", "c", "d", "e", "f"}, result)
	})
}

func TestParsePolicyRow(t *testing.T) {
	t.Run("no_trailing_empty", func(t *testing.T) {
		result := parsePolicyRow([]string{"p", "alice", "data1", "read"})
		assert.Equal(t, []string{"p", "alice", "data1", "read"}, result)
	})

	t.Run("trailing_empty_trimmed", func(t *testing.T) {
		result := parsePolicyRow([]string{"p", "alice", "data1", "read", "", "", ""})
		assert.Equal(t, []string{"p", "alice", "data1", "read"}, result)
	})

	t.Run("all_empty", func(t *testing.T) {
		result := parsePolicyRow([]string{"", "", ""})
		assert.Equal(t, []string{}, result)
	})

	t.Run("whitespace_trimmed", func(t *testing.T) {
		result := parsePolicyRow([]string{"  p  ", " alice ", "data1"})
		assert.Equal(t, []string{"p", "alice", "data1"}, result)
	})
}

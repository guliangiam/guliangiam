package groups

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGroup_Fields(t *testing.T) {
	now := time.Now()
	group := Group{
		ID:             "019test-group-id",
		OrganizationID: "019test-org-id",
		Name:           "engineering",
		DisplayName:    "Engineering Team",
		Manager:        "manager-1",
		ContactEmail:   "eng@example.com",
		Type:           "department",
		ParentID:       "019parent-id",
		IsTopGroup:     false,
		IsEnabled:      true,
		Homepage:       "https://eng.example.com",
		Users:          []string{"user-1", "user-2"},
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	assert.Equal(t, "019test-group-id", group.ID)
	assert.Equal(t, "engineering", group.Name)
	assert.True(t, group.IsEnabled)
	assert.Len(t, group.Users, 2)
}

func TestGroup_Validate(t *testing.T) {
	t.Run("valid_group", func(t *testing.T) {
		g := Group{ID: "group-001", OrganizationID: "org-001", Name: "admins"}
		assert.NoError(t, g.Validate())
	})

	t.Run("missing_id", func(t *testing.T) {
		g := Group{OrganizationID: "org-001", Name: "admins"}
		assert.Error(t, g.Validate())
	})

	t.Run("missing_organizationId", func(t *testing.T) {
		g := Group{ID: "group-001", Name: "admins"}
		assert.Error(t, g.Validate())
	})

	t.Run("missing_name", func(t *testing.T) {
		g := Group{ID: "group-001", OrganizationID: "org-001"}
		assert.Error(t, g.Validate())
	})
}

func TestGroup_HasUser(t *testing.T) {
	g := Group{
		ID:    "group-001",
		Users: []string{"user-001", "user-002", "user-003"},
	}

	t.Run("member_exists", func(t *testing.T) {
		assert.True(t, g.HasUser("user-001"))
		assert.True(t, g.HasUser("user-003"))
	})

	t.Run("member_not_exists", func(t *testing.T) {
		assert.False(t, g.HasUser("user-999"))
	})

	t.Run("empty_users", func(t *testing.T) {
		emptyG := Group{Users: []string{}}
		assert.False(t, emptyG.HasUser("user-001"))
	})
}

func TestGroup_IsSubGroup(t *testing.T) {
	t.Run("top_level_group", func(t *testing.T) {
		g := Group{ID: "group-001", ParentID: ""}
		assert.False(t, g.IsSubGroup())
	})

	t.Run("sub_group", func(t *testing.T) {
		g := Group{ID: "group-002", ParentID: "group-001"}
		assert.True(t, g.IsSubGroup())
	})
}

// ---------------------------------------------------------------------------
// Extended boundary and tree structure tests
// ---------------------------------------------------------------------------

func TestGroup_Validate_Boundary(t *testing.T) {
	tests := []struct {
		name    string
		group   Group
		wantErr bool
	}{
		{"valid minimal", Group{ID: "g-1", OrganizationID: "org-1", Name: "team"}, false},
		{"missing id", Group{OrganizationID: "org-1", Name: "team"}, true},
		{"missing org id", Group{ID: "g-1", Name: "team"}, true},
		{"missing name", Group{ID: "g-1", OrganizationID: "org-1"}, true},
		{"all missing", Group{}, true},
		{"valid with parent", Group{ID: "g-2", OrganizationID: "org-1", Name: "sub", ParentID: "g-1"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroup_TreeStructure(t *testing.T) {
	t.Run("multi_level_hierarchy", func(t *testing.T) {
		root := Group{ID: "g-root", Name: "Root", IsTopGroup: true, ParentID: ""}
		level1 := Group{ID: "g-l1", Name: "Level1", ParentID: "g-root", IsTopGroup: false}
		level2 := Group{ID: "g-l2", Name: "Level2", ParentID: "g-l1", IsTopGroup: false}

		assert.False(t, root.IsSubGroup())
		assert.True(t, level1.IsSubGroup())
		assert.True(t, level2.IsSubGroup())
		assert.Equal(t, "g-root", level1.ParentID)
		assert.Equal(t, "g-l1", level2.ParentID)
	})

	t.Run("top_group_consistency", func(t *testing.T) {
		// A top group should not have a parent ID
		topGroup := Group{ID: "g-top", Name: "Top", IsTopGroup: true, ParentID: ""}
		if topGroup.IsTopGroup && topGroup.ParentID != "" {
			t.Error("top group should have empty ParentID")
		}
	})

	t.Run("circular_reference_detection_conceptual", func(t *testing.T) {
		// This test documents the expectation that circular references are invalid.
		// In practice, the service layer should detect this.
		g1 := Group{ID: "g-1", ParentID: "g-2"}
		g2 := Group{ID: "g-2", ParentID: "g-1"}
		// Both are sub-groups, which is a conceptual circular reference
		assert.True(t, g1.IsSubGroup())
		assert.True(t, g2.IsSubGroup())
	})
}

func TestGroup_HasUser_Boundary(t *testing.T) {
	tests := []struct {
		name   string
		group  Group
		userID string
		want   bool
	}{
		{"member present", Group{Users: []string{"u-1", "u-2"}}, "u-1", true},
		{"member absent", Group{Users: []string{"u-1", "u-2"}}, "u-3", false},
		{"empty users", Group{Users: []string{}}, "u-1", false},
		{"nil users", Group{Users: nil}, "u-1", false},
		{"single member match", Group{Users: []string{"u-1"}}, "u-1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.group.HasUser(tt.userID)
			if got != tt.want {
				t.Errorf("HasUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroup_Types(t *testing.T) {
	validTypes := map[string]bool{
		"department": true,
		"virtual":    true,
		"physical":   true,
	}
	tests := []struct {
		name  string
		gtype string
		want  bool
	}{
		{"department", "department", true},
		{"virtual", "virtual", true},
		{"physical", "physical", true},
		{"unknown", "unknown", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if validTypes[tt.gtype] != tt.want {
				t.Errorf("type %q valid = %v, want %v", tt.gtype, validTypes[tt.gtype], tt.want)
			}
		})
	}
}

func TestGroup_ContactEmail(t *testing.T) {
	g := Group{ID: "g-1", ContactEmail: "team@example.com"}
	assert.Equal(t, "team@example.com", g.ContactEmail)

	emptyG := Group{ID: "g-2", ContactEmail: ""}
	assert.Equal(t, "", emptyG.ContactEmail)
}

func TestGroup_Manager(t *testing.T) {
	g := Group{ID: "g-1", Manager: "user-001"}
	assert.Equal(t, "user-001", g.Manager)
}

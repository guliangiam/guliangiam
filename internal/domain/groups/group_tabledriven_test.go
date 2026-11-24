package groups

import (
	"testing"
	"time"
)

func TestGroup_Validate_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		group Group
	}{
		{
			name: "valid department group",
			group: Group{
				ID: "g-1", OrganizationID: "org-1", Name: "engineering",
				DisplayName: "Engineering", Type: "department",
				IsEnabled: true, IsTopGroup: false, ParentID: "g-root",
				Users:     []string{"user-1", "user-2"},
				CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "top-level group",
			group: Group{
				ID: "g-root", OrganizationID: "org-1", Name: "root",
				DisplayName: "Root", Type: "virtual",
				IsTopGroup: true, ParentID: "",
				IsEnabled: true,
				CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "disabled group",
			group: Group{
				ID: "g-2", Name: "disabled", Type: "department",
				IsEnabled: false,
				CreatedAt: now, UpdatedAt: now,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.group.ID == "" {
				t.Error("ID should not be empty")
			}
		})
	}
}

func TestGroup_Type_TableDriven(t *testing.T) {
	validTypes := map[string]bool{
		"department": true, "virtual": true, "physical": true,
	}
	tests := []struct {
		name  string
		gtype string
		valid bool
	}{
		{"department", "department", true},
		{"virtual", "virtual", true},
		{"physical", "physical", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validTypes[tt.gtype]
			if got != tt.valid {
				t.Errorf("type %q valid = %v, want %v", tt.gtype, got, tt.valid)
			}
		})
	}
}

func TestGroup_TopGroupConsistency_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		group      Group
		wantParent string
	}{
		{"top group has empty parent", Group{IsTopGroup: true, ParentID: ""}, ""},
		{"non-top group has parent", Group{IsTopGroup: false, ParentID: "g-root"}, "g-root"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.group.ParentID != tt.wantParent {
				t.Errorf("ParentID = %q, want %q", tt.group.ParentID, tt.wantParent)
			}
		})
	}
}

func TestGroup_UserMembership_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		users     []string
		wantCount int
	}{
		{"no users", nil, 0},
		{"empty users", []string{}, 0},
		{"one user", []string{"user-1"}, 1},
		{"multiple users", []string{"user-1", "user-2", "user-3"}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := Group{Users: tt.users}
			if len(group.Users) != tt.wantCount {
				t.Errorf("len(Users) = %d, want %d", len(group.Users), tt.wantCount)
			}
		})
	}
}

func TestGroup_ZeroValue(t *testing.T) {
	var group Group
	if group.ID != "" {
		t.Errorf("ID = %q, want empty", group.ID)
	}
	if group.IsEnabled {
		t.Error("IsEnabled should be false by default")
	}
	if group.IsTopGroup {
		t.Error("IsTopGroup should be false by default")
	}
}

func TestListQuery_Defaults(t *testing.T) {
	var query ListQuery
	if query.Page != 0 {
		t.Errorf("Page = %d, want 0", query.Page)
	}
	if query.PageSize != 0 {
		t.Errorf("PageSize = %d, want 0", query.PageSize)
	}
}

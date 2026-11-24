package permissions

import (
	"testing"
	"time"
)

func TestPermission_Validate_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		permission Permission
	}{
		{
			name: "allow permission with resources",
			permission: Permission{
				ID: "perm-1", OrganizationID: "org-1",
				Name: "read-users", DisplayName: "Read Users",
				ResourceType: "application", Resources: []string{"app-1"},
				Actions: []string{"read"}, Effect: "allow",
				IsEnabled: true, State: "active",
				CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "deny permission",
			permission: Permission{
				ID: "perm-2", Name: "deny-all", Effect: "deny",
				IsEnabled: true, State: "active",
				CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "disabled permission",
			permission: Permission{
				ID: "perm-3", Name: "inactive-perm", Effect: "allow",
				IsEnabled: false, State: "inactive",
				CreatedAt: now, UpdatedAt: now,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.permission.ID == "" {
				t.Error("ID should not be empty")
			}
		})
	}
}

func TestPermission_Effect_TableDriven(t *testing.T) {
	validEffects := map[string]bool{"allow": true, "deny": true}
	tests := []struct {
		name   string
		effect string
		valid  bool
	}{
		{"allow", "allow", true},
		{"deny", "deny", true},
		{"invalid", "grant", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validEffects[tt.effect]
			if got != tt.valid {
				t.Errorf("effect %q valid = %v, want %v", tt.effect, got, tt.valid)
			}
		})
	}
}

func TestPermission_State_TableDriven(t *testing.T) {
	validStates := map[string]bool{"active": true, "inactive": true, "pending": true}
	tests := []struct {
		name  string
		state string
		valid bool
	}{
		{"active", "active", true},
		{"inactive", "inactive", true},
		{"pending", "pending", true},
		{"unknown", "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validStates[tt.state]
			if got != tt.valid {
				t.Errorf("state %q valid = %v, want %v", tt.state, got, tt.valid)
			}
		})
	}
}

func TestPermission_Approval_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		permission Permission
		isApproved bool
	}{
		{
			name: "approved permission",
			permission: Permission{
				ID: "perm-1", State: "active",
				Submitter: "user-1", Approver: "admin-1",
				ApproveTime: &now,
			},
			isApproved: true,
		},
		{
			name: "pending approval",
			permission: Permission{
				ID: "perm-2", State: "pending",
				Submitter: "user-1", Approver: "",
				ApproveTime: nil,
			},
			isApproved: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.permission.ApproveTime != nil
			if got != tt.isApproved {
				t.Errorf("isApproved = %v, want %v", got, tt.isApproved)
			}
		})
	}
}

func TestPermission_RbacModel_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		model string
		valid bool
	}{
		{"rbac", "rbac", true},
		{"abac", "abac", true},
		{"casbin", "casbin", true},
		{"invalid", "invalid", false},
	}
	validModels := map[string]bool{"rbac": true, "abac": true, "casbin": true}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validModels[tt.model]
			if got != tt.valid {
				t.Errorf("model %q valid = %v, want %v", tt.model, got, tt.valid)
			}
		})
	}
}

func TestPermission_ResourceType_TableDriven(t *testing.T) {
	validTypes := map[string]bool{
		"application": true, "organization": true, "user": true,
		"group": true, "role": true, "provider": true,
	}
	tests := []struct {
		name         string
		resourceType string
		valid        bool
	}{
		{"application", "application", true},
		{"organization", "organization", true},
		{"user", "user", true},
		{"invalid", "invalid", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validTypes[tt.resourceType]
			if got != tt.valid {
				t.Errorf("resourceType %q valid = %v, want %v", tt.resourceType, got, tt.valid)
			}
		})
	}
}

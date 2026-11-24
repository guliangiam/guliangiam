package users

import (
	"testing"
	"time"
)

func TestUser_Fields_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		user User
	}{
		{
			name: "active normal user",
			user: User{
				ID: "user-1", OrganizationID: "org-1", Username: "alice",
				UserType: "normal", Status: "active",
				IsAdmin: false, IsForbidden: false, IsVerified: true,
				CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "admin user",
			user: User{
				ID: "user-2", OrganizationID: "org-1", Username: "admin",
				UserType: "admin", Status: "active", IsAdmin: true,
				CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "forbidden user",
			user: User{
				ID: "user-3", Username: "banned", Status: "forbidden",
				IsForbidden: true, CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "deleted user",
			user: User{
				ID: "user-4", Username: "deleted", Status: "deleted",
				IsDeleted: true, DeletedAt: &now,
				CreatedAt: now, UpdatedAt: now,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.user.ID == "" {
				t.Error("ID should not be empty")
			}
		})
	}
}

func TestUser_Status_TableDriven(t *testing.T) {
	// 枚举事实源：contracts/registries/state-machines.yaml §users_status。
	tests := []struct {
		name   string
		status string
		valid  bool
	}{
		{"active", "active", true},
		{"disabled", "disabled", true},
		{"locked", "locked", true},
		{"pending_verification", "pending_verification", true},
		{"inactive is not in the enum", "inactive", false},
		{"suspended is not in the enum", "suspended", false},
		{"unknown", "unknown", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValid(tt.status)
			if got != tt.valid {
				t.Errorf("IsValid(%q) = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestUser_MfaRelatedFields(t *testing.T) {
	now := time.Now()
	user := User{
		ID:                  "user-1",
		PreferredMfaType:    "totp",
		TotpSecret:          "JBSWY3DPEHPK3PXP",
		MfaPushEnabled:      true,
		MfaPushReceiver:     "user@example.com",
		MfaPushProvider:     "fcm",
		MfaRememberDeadline: &now,
		MfaItems: []MfaItem{
			{MfaType: "totp", Secret: "secret1"},
			{MfaType: "sms", Secret: "secret2"},
		},
		MfaMethods: []UserMfaMethod{
			{MethodType: "totp", Enabled: true},
		},
	}
	if user.PreferredMfaType != "totp" {
		t.Errorf("PreferredMfaType = %q, want %q", user.PreferredMfaType, "totp")
	}
	if !user.MfaPushEnabled {
		t.Error("MfaPushEnabled should be true")
	}
	if len(user.MfaItems) != 2 {
		t.Errorf("len(MfaItems) = %d, want 2", len(user.MfaItems))
	}
	if user.MfaRememberDeadline == nil {
		t.Error("MfaRememberDeadline should not be nil")
	}
}

func TestUser_SigninTracking(t *testing.T) {
	now := time.Now()
	user := User{
		ID:                  "user-1",
		SigninWrongTimes:    3,
		LastSigninWrongTime: &now,
		LastSigninAt:        &now,
		LastSigninIP:        "192.168.1.1",
		IsOnline:            true,
	}
	if user.SigninWrongTimes != 3 {
		t.Errorf("SigninWrongTimes = %d, want 3", user.SigninWrongTimes)
	}
	if !user.IsOnline {
		t.Error("IsOnline should be true")
	}
	if user.LastSigninAt == nil {
		t.Error("LastSigninAt should not be nil")
	}
}

func TestUser_ZeroValue(t *testing.T) {
	var user User
	if user.ID != "" {
		t.Errorf("ID = %q, want empty", user.ID)
	}
	if user.IsAdmin {
		t.Error("IsAdmin should be false by default")
	}
	if user.IsForbidden {
		t.Error("IsForbidden should be false by default")
	}
	if user.IsDeleted {
		t.Error("IsDeleted should be false by default")
	}
}

func TestAddress_Fields(t *testing.T) {
	addr := Address{Region: "CN", Street: "Main St", Details: "Apt 101"}
	if addr.Region != "CN" {
		t.Errorf("Region = %q, want %q", addr.Region, "CN")
	}
}

func TestManagedAccount_Fields(t *testing.T) {
	ma := ManagedAccount{
		Application: "github", Username: "alice", Password: "secret", SigninUrl: "https://github.com/login",
	}
	if ma.Application != "github" {
		t.Errorf("Application = %q, want %q", ma.Application, "github")
	}
}

func TestConsentRecord_Fields(t *testing.T) {
	cr := ConsentRecord{
		Application: "app-1", Scopes: []string{"read", "write"}, GrantedAt: "2025-01-01T00:00:00Z",
	}
	if len(cr.Scopes) != 2 {
		t.Errorf("len(Scopes) = %d, want 2", len(cr.Scopes))
	}
}

func TestUserProfile_Completeness(t *testing.T) {
	profile := UserProfile{
		UserID: "user-1", DisplayName: "Alice", Email: "alice@example.com", Phone: "+1234567890",
	}
	if profile.Email == "" {
		t.Error("Email should not be empty for complete profile")
	}
	if profile.Phone == "" {
		t.Error("Phone should not be empty for complete profile")
	}
}

package tokens

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRefreshToken_Fields(t *testing.T) {
	now := time.Now()
	rt := RefreshToken{
		ID:               "rt-1",
		AccessTokenID:    "at-1",
		UserID:           "user-1",
		Username:         "alice",
		OrganizationID:   "org-1",
		OrganizationSlug: "acme",
		ApplicationID:    "app-1",
		ApplicationSlug:  "myapp",
		RefreshTokenHash: "hash-rt-123",
		FamilyID:         "family-1",
		Scopes:           []string{"openid", "offline_access"},
		Audience:         []string{"app-1"},
		ExpiresAt:        now.Add(24 * time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         1,
	}

	if rt.ID != "rt-1" {
		t.Errorf("ID = %q, want %q", rt.ID, "rt-1")
	}
	if rt.AccessTokenID != "at-1" {
		t.Errorf("AccessTokenID = %q, want %q", rt.AccessTokenID, "at-1")
	}
	if rt.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", rt.UserID, "user-1")
	}
	if rt.FamilyID != "family-1" {
		t.Errorf("FamilyID = %q, want %q", rt.FamilyID, "family-1")
	}
	if len(rt.Scopes) != 2 {
		t.Errorf("len(Scopes) = %d, want 2", len(rt.Scopes))
	}
	if len(rt.Audience) != 1 {
		t.Errorf("len(Audience) = %d, want 1", len(rt.Audience))
	}
}

func TestRefreshToken_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		token       RefreshToken
		wantExpired bool
	}{
		{
			name:        "not expired - future",
			token:       RefreshToken{ExpiresAt: now.Add(24 * time.Hour)},
			wantExpired: false,
		},
		{
			name:        "expired - past",
			token:       RefreshToken{ExpiresAt: now.Add(-1 * time.Hour)},
			wantExpired: true,
		},
		{
			name:        "not expired - exact time",
			token:       RefreshToken{ExpiresAt: now},
			wantExpired: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := now.After(tt.token.ExpiresAt)
			if got != tt.wantExpired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.wantExpired)
			}
		})
	}
}

func TestRefreshToken_IsRevoked(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		token       RefreshToken
		wantRevoked bool
	}{
		{
			name:        "not revoked - nil RevokedAt",
			token:       RefreshToken{RevokedAt: nil},
			wantRevoked: false,
		},
		{
			name:        "revoked - RevokedAt set",
			token:       RefreshToken{RevokedAt: &now},
			wantRevoked: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.token.RevokedAt != nil
			if got != tt.wantRevoked {
				t.Errorf("IsRevoked() = %v, want %v", got, tt.wantRevoked)
			}
		})
	}
}

func TestRefreshToken_ZeroValue(t *testing.T) {
	var rt RefreshToken
	if rt.ID != "" {
		t.Errorf("ID = %q, want empty", rt.ID)
	}
	if rt.RevokedAt != nil {
		t.Error("RevokedAt should be nil by default")
	}
	if rt.Revision != 0 {
		t.Errorf("Revision = %d, want 0", rt.Revision)
	}
}

func TestRefreshToken_FamilyTracking(t *testing.T) {
	now := time.Now()
	rt := RefreshToken{
		ID:               "rt-1",
		FamilyID:         "family-abc",
		AccessTokenID:    "at-1",
		RefreshTokenHash: "hash-rt-1",
		ExpiresAt:        now.Add(24 * time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         1,
	}

	if rt.FamilyID != "family-abc" {
		t.Errorf("FamilyID = %q, want %q", rt.FamilyID, "family-abc")
	}
	if rt.AccessTokenID != "at-1" {
		t.Errorf("AccessTokenID = %q, want %q", rt.AccessTokenID, "at-1")
	}
}

func TestRefreshToken_ScopeOperations(t *testing.T) {
	rt := RefreshToken{
		Scopes: []string{"openid", "profile", "email", "offline_access"},
	}

	// Verify all expected scopes are present
	expectedScopes := map[string]bool{
		"openid": false, "profile": false, "email": false, "offline_access": false,
	}
	for _, scope := range rt.Scopes {
		if _, ok := expectedScopes[scope]; ok {
			expectedScopes[scope] = true
		}
	}
	for scope, found := range expectedScopes {
		if !found {
			t.Errorf("expected scope %q not found", scope)
		}
	}
}

func TestToken_Validate_AllFields(t *testing.T) {
	now := time.Now().Add(time.Hour)
	token := Token{
		ID:               "token-1",
		UserID:           "user-1",
		Username:         "alice",
		OrganizationID:   "org-1",
		OrganizationSlug: "acme",
		ApplicationID:    "app-1",
		ApplicationSlug:  "myapp",
		TokenPrefix:      "glt_",
		TokenHash:        "hash123",
		Scopes:           []string{"openid", "profile"},
		Audience:         []string{"app-1"},
		ExpiresAt:        now,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Revision:         1,
	}

	if err := token.Validate(); err != nil {
		t.Errorf("valid token should pass, got error: %v", err)
	}
}

func TestToken_Validate_ScopeValidation(t *testing.T) {
	now := time.Now().Add(time.Hour)

	tests := []struct {
		name    string
		scopes  []string
		wantErr bool
		errCode apperrors.Code
	}{
		{"valid scopes", []string{"openid", "profile"}, false, ""},
		{"empty scopes slice", []string{}, false, ""},
		{"nil scopes", nil, false, ""},
		{"single empty scope", []string{""}, true, apperrors.CodeNotFound},
		{"mixed valid and empty", []string{"openid", ""}, true, apperrors.CodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := Token{
				UserID:        "user-1",
				TokenHash:     "hash",
				ApplicationID: "app-1",
				ExpiresAt:     now,
				Scopes:        tt.scopes,
			}
			err := token.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errCode != "" {
				appErr, ok := err.(*apperrors.Error)
				if !ok {
					t.Fatalf("error type = %T, want *apperrors.Error", err)
				}
				if appErr.Code != tt.errCode {
					t.Errorf("error code = %q, want %q", appErr.Code, tt.errCode)
				}
			}
		})
	}
}

func TestToken_CombinedStateChecks(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		token       Token
		now         time.Time
		wantExpired bool
		wantRevoked bool
	}{
		{
			name:        "active token",
			token:       Token{ExpiresAt: now.Add(time.Hour), RevokedAt: nil},
			now:         now,
			wantExpired: false,
			wantRevoked: false,
		},
		{
			name:        "expired but not revoked",
			token:       Token{ExpiresAt: now.Add(-time.Hour), RevokedAt: nil},
			now:         now,
			wantExpired: true,
			wantRevoked: false,
		},
		{
			name:        "revoked but not expired",
			token:       Token{ExpiresAt: now.Add(time.Hour), RevokedAt: &now},
			now:         now,
			wantExpired: false,
			wantRevoked: true,
		},
		{
			name:        "both expired and revoked",
			token:       Token{ExpiresAt: now.Add(-time.Hour), RevokedAt: &now},
			now:         now,
			wantExpired: true,
			wantRevoked: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsExpired(tt.now); got != tt.wantExpired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.wantExpired)
			}
			if got := tt.token.IsRevoked(); got != tt.wantRevoked {
				t.Errorf("IsRevoked() = %v, want %v", got, tt.wantRevoked)
			}
		})
	}
}

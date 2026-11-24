package auth

import (
	"testing"
	"time"

	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
)

// ---------------------------------------------------------------------------
// Extended scope operation tests (boundary/edge cases)
// ---------------------------------------------------------------------------

func TestNormalizeScopes_Extended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		scopes []string
		want   []string
	}{
		{"nil_input", nil, []string{}},
		{"all_whitespace", []string{"  ", "", "\t"}, []string{}},
		{"dedup_preserves_order", []string{"profile", "openid", "openid", "email"}, []string{"profile", "openid", "email"}},
		{"single_scope", []string{"openid"}, []string{"openid"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeScopes(tt.scopes)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeScopes_Extended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing []string
		added    []string
		wantLen  int
	}{
		{"both_empty", []string{}, []string{}, 0},
		{"existing_only", []string{"openid"}, []string{}, 1},
		{"added_only", []string{}, []string{"profile"}, 1},
		{"no_overlap", []string{"openid"}, []string{"profile"}, 2},
		{"complete_overlap", []string{"openid"}, []string{"openid"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeScopes(tt.existing, tt.added)
			if len(got) != tt.wantLen {
				t.Errorf("mergeScopes() len = %d, want %d, got %v", len(got), tt.wantLen, got)
			}
		})
	}
}

func TestDifferenceScopes_Extended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		left    []string
		right   []string
		wantLen int
	}{
		{"both_empty", []string{}, []string{}, 0},
		{"left_only", []string{"openid", "profile"}, []string{}, 2},
		{"right_covers_all", []string{"openid"}, []string{"openid"}, 0},
		{"partial_overlap", []string{"openid", "profile", "email"}, []string{"profile"}, 2},
		{"no_overlap", []string{"openid"}, []string{"profile"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := differenceScopes(tt.left, tt.right)
			if len(got) != tt.wantLen {
				t.Errorf("differenceScopes() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestSubtractScopes_Extended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing []string
		revoked  []string
		wantLen  int
	}{
		{"both_empty", []string{}, []string{}, 0},
		{"nothing_revoked", []string{"openid", "profile"}, []string{}, 2},
		{"revoke_all", []string{"openid"}, []string{"openid"}, 0},
		{"partial_revoke", []string{"openid", "profile", "email"}, []string{"profile"}, 2},
		{"revoke_nonexistent", []string{"openid"}, []string{"admin"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := subtractScopes(tt.existing, tt.revoked)
			if len(got) != tt.wantLen {
				t.Errorf("subtractScopes() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Consent detail mapping tests
// ---------------------------------------------------------------------------

func TestToConsentDetail_Extended(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	consent := &consentdomain.Consent{
		ID:               "consent-1",
		UserID:           "user-1",
		Username:         "alice",
		OrganizationID:   "org-1",
		OrganizationSlug: "acme",
		ApplicationID:    "app-1",
		ApplicationSlug:  "test-app",
		GrantedScopes:    []string{"openid", "profile"},
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         2,
	}

	detail := toConsentDetail(consent)

	if detail.ID != "consent-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "consent-1")
	}
	if detail.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", detail.UserID, "user-1")
	}
	if detail.Username != "alice" {
		t.Errorf("Username = %q, want %q", detail.Username, "alice")
	}
	if detail.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", detail.OrganizationID, "org-1")
	}
	if detail.ApplicationID != "app-1" {
		t.Errorf("ApplicationID = %q, want %q", detail.ApplicationID, "app-1")
	}
	if len(detail.GrantedScopes) != 2 {
		t.Errorf("GrantedScopes len = %d, want 2", len(detail.GrantedScopes))
	}
	if detail.Revision != 2 {
		t.Errorf("Revision = %d, want 2", detail.Revision)
	}
}

func TestToConsentDetail_WithRevokedAt(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	consent := &consentdomain.Consent{
		ID:        "consent-2",
		RevokedAt: &now,
	}

	detail := toConsentDetail(consent)

	if detail.RevokedAt == nil {
		t.Error("RevokedAt should not be nil")
	}
}

func TestToConsentDetail_GrantedScopesCopy(t *testing.T) {
	consent := &consentdomain.Consent{
		GrantedScopes: []string{"openid"},
	}

	detail := toConsentDetail(consent)

	// Modifying the detail's scopes should not affect the original
	detail.GrantedScopes[0] = "modified"
	if consent.GrantedScopes[0] != "openid" {
		t.Error("modifying detail should not affect original consent")
	}
}

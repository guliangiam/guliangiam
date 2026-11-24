package auth

import (
	"slices"
	"testing"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
)

func TestNormalizeScopes(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		want   []string
	}{
		{"nil", nil, []string{}},
		{"empty", []string{}, []string{}},
		{"whitespace_only", []string{"  ", ""}, []string{}},
		{"trims", []string{" openid ", "profile "}, []string{"openid", "profile"}},
		{"dedup", []string{"openid", "openid", "profile"}, []string{"openid", "profile"}},
		{"mixed", []string{"openid", " ", "profile", "openid"}, []string{"openid", "profile"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeScopes(tt.scopes)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMergeScopes(t *testing.T) {
	got := mergeScopes([]string{"openid", "profile"}, []string{"profile", "email"})
	want := []string{"openid", "profile", "email"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestDifferenceScopes(t *testing.T) {
	got := differenceScopes([]string{"openid", "profile", "email"}, []string{"profile"})
	want := []string{"openid", "email"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestSubtractScopes(t *testing.T) {
	got := subtractScopes([]string{"openid", "profile", "email"}, []string{"profile", "email"})
	want := []string{"openid"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterConsentableScopes(t *testing.T) {
	customScopes := []applicationdomain.ApplicationCustomScope{
		{Scope: "read"}, {Scope: "write"},
	}
	tests := []struct {
		name      string
		requested []string
		want      []string
	}{
		{"empty_requested", []string{}, []string{}},
		{"match", []string{"read", "admin", "write"}, []string{"read", "write"}},
		{"no_match", []string{"admin"}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterConsentableScopes(tt.requested, customScopes)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestValidateGrantedConsentScopes(t *testing.T) {
	app := &applicationdomain.Application{
		CustomScopes: []applicationdomain.ApplicationCustomScope{
			{Scope: "read"}, {Scope: "write"},
		},
	}
	t.Run("valid", func(t *testing.T) {
		scopes, err := validateGrantedConsentScopes(app, []string{"read"})
		if err != nil {
			t.Fatal(err)
		}
		if len(scopes) != 1 || scopes[0] != "read" {
			t.Fatalf("got %v", scopes)
		}
	})
	t.Run("empty_scopes", func(t *testing.T) {
		_, err := validateGrantedConsentScopes(app, []string{})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("no_custom_scopes", func(t *testing.T) {
		noScopeApp := &applicationdomain.Application{}
		_, err := validateGrantedConsentScopes(noScopeApp, []string{"read"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("invalid_scope", func(t *testing.T) {
		_, err := validateGrantedConsentScopes(app, []string{"admin"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestToConsentDetail(t *testing.T) {
	consent := &consentdomain.Consent{
		ID:               "c1",
		UserID:           "u1",
		Username:         "alice",
		OrganizationID:   "o1",
		OrganizationSlug: "org1",
		ApplicationID:    "a1",
		ApplicationSlug:  "app1",
		GrantedScopes:    []string{"read", "write"},
		Revision:         2,
	}
	detail := toConsentDetail(consent)
	if detail.ID != "c1" || detail.Username != "alice" || detail.Revision != 2 {
		t.Fatalf("unexpected: %+v", detail)
	}
	if len(detail.GrantedScopes) != 2 {
		t.Fatalf("got %v", detail.GrantedScopes)
	}
}

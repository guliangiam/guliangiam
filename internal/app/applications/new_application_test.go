package applications

import (
	"strings"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var errUUIDFailed = apperrors.NewInternal("uuid generation failed")

// ---------------------------------------------------------------------------
// Tests for NewApplication
// ---------------------------------------------------------------------------

func TestService_NewApplication_UUIDError(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &errorUUIDGenerator{},
	}

	_, err := svc.NewApplication(CreateApplicationCommand{
		OrganizationID: "org-1",
		Slug:           "my-app",
		DisplayName:    "My App",
		Category:       "web",
		Status:         "active",
	})
	if err == nil {
		t.Fatal("NewApplication() expected error when UUID fails, got nil")
	}
}

func TestService_NewApplication_WithClientSecret(t *testing.T) {
	now := time.Now()
	uuid := &fakeUUIDGenerator{fixedID: "019app-test-id"}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: uuid}

	app, err := svc.NewApplication(CreateApplicationCommand{
		OrganizationID: "org-1",
		Slug:           "my-app",
		DisplayName:    "My App",
		Category:       "web",
		AppType:        "native",
		Status:         "active",
		ClientID:       "", // Empty → must auto-generate gl_cid_*
		ClientSecret:   "super-secret",
		RedirectURIs:   []string{"  https://example.com/callback  ", "  "},
		GrantTypes:     []string{"authorization_code", "  ", "authorization_code"},
	})
	if err != nil {
		t.Fatalf("NewApplication() error = %v", err)
	}

	// ClientID must be auto-generated with gl_cid_ prefix when command.ClientID is empty.
	if !strings.HasPrefix(app.ClientID, "gl_cid_") {
		t.Errorf("ClientID = %q, want gl_cid_ auto-generated (empty command must trigger auto-gen)", app.ClientID)
	}
}

func TestService_NewApplication_RespectsSuppliedClientID(t *testing.T) {
	now := time.Now()
	uuid := &fakeUUIDGenerator{fixedID: "019app-test-id-000"}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: uuid}

	// DCR passes its own generated clientID → must be respected (not overwritten).
	app, err := svc.NewApplication(CreateApplicationCommand{
		OrganizationID: "org-1",
		Slug:           "dcr-app",
		DisplayName:    "DCR App",
		Status:         "active",
		ClientID:       "gl_cid_d8f3a2b1c9e4f5a6b7c8d9e0f1a2b3c4d",
	})
	if err != nil {
		t.Fatal(err)
	}
	if app.ClientID != "gl_cid_d8f3a2b1c9e4f5a6b7c8d9e0f1a2b3c4d" {
		t.Errorf("supplied ClientID must be respected, got %q", app.ClientID)
	}
}

func TestService_NewApplication_NoClientSecret(t *testing.T) {
	now := time.Now()
	uuid := &fakeUUIDGenerator{fixedID: "019app-test-id"}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: uuid}

	app, err := svc.NewApplication(CreateApplicationCommand{
		OrganizationID: "org-1",
		Slug:           "my-app",
		DisplayName:    "My App",
		Category:       "web",
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("NewApplication() error = %v", err)
	}

	// ClientSecretHash should be empty when no secret is provided
	if app.ClientSecretHash != "" {
		t.Error("ClientSecretHash should be empty when no ClientSecret is provided")
	}

	// TokenEndpointAuthMethod should default to "none" when no secret
	if app.TokenEndpointAuthMethod != "none" {
		t.Errorf("TokenEndpointAuthMethod = %q, want %q", app.TokenEndpointAuthMethod, "none")
	}
}

func TestService_NewApplication_ExplicitTokenEndpointAuthMethod(t *testing.T) {
	now := time.Now()
	uuid := &fakeUUIDGenerator{fixedID: "019app-test-id"}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: uuid}

	tests := []struct {
		name         string
		method       string
		clientSecret string
		wantMethod   string
	}{
		{"none explicit", "none", "", "none"},
		{"client_secret_post explicit", "client_secret_post", "secret", "client_secret_post"},
		{"client_secret_basic explicit", "client_secret_basic", "secret", "client_secret_basic"},
		{"unknown method with secret", "unknown_method", "secret", "client_secret_basic"},
		{"unknown method without secret", "unknown_method", "", "none"},
		{"whitespace method", "  client_secret_basic  ", "secret", "client_secret_basic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := svc.NewApplication(CreateApplicationCommand{
				OrganizationID:          "org-1",
				Slug:                    "my-app",
				DisplayName:             "My App",
				Category:                "web",
				Status:                  "active",
				ClientSecret:            tt.clientSecret,
				TokenEndpointAuthMethod: tt.method,
			})
			if err != nil {
				t.Fatalf("NewApplication() error = %v", err)
			}
			if app.TokenEndpointAuthMethod != tt.wantMethod {
				t.Errorf("TokenEndpointAuthMethod = %q, want %q", app.TokenEndpointAuthMethod, tt.wantMethod)
			}
		})
	}
}

func TestService_NewApplication_CustomScopes(t *testing.T) {
	now := time.Now()
	uuid := &fakeUUIDGenerator{fixedID: "019app-test-id"}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: uuid}

	app, err := svc.NewApplication(CreateApplicationCommand{
		OrganizationID: "org-1",
		Slug:           "my-app",
		DisplayName:    "My App",
		Category:       "web",
		Status:         "active",
		CustomScopes: []ApplicationCustomScopeCommand{
			{Scope: "read:users", DisplayName: "Read Users", Description: "Read user data"},
			{Scope: "write:users", DisplayName: "Write Users", Description: "Write user data"},
		},
	})
	if err != nil {
		t.Fatalf("NewApplication() error = %v", err)
	}

	if len(app.CustomScopes) != 2 {
		t.Fatalf("len(CustomScopes) = %d, want 2", len(app.CustomScopes))
	}
	if app.CustomScopes[0].Scope != "read:users" {
		t.Errorf("CustomScopes[0].Scope = %q, want %q", app.CustomScopes[0].Scope, "read:users")
	}
	if app.CustomScopes[1].DisplayName != "Write Users" {
		t.Errorf("CustomScopes[1].DisplayName = %q, want %q", app.CustomScopes[1].DisplayName, "Write Users")
	}
}

func TestService_NewApplication_EmptyCustomScopes(t *testing.T) {
	now := time.Now()
	uuid := &fakeUUIDGenerator{fixedID: "019app-test-id"}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: uuid}

	app, err := svc.NewApplication(CreateApplicationCommand{
		OrganizationID: "org-1",
		Slug:           "my-app",
		DisplayName:    "My App",
		Category:       "web",
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("NewApplication() error = %v", err)
	}

	if app.CustomScopes == nil {
		t.Error("CustomScopes should not be nil, want empty slice")
	}
	if len(app.CustomScopes) != 0 {
		t.Errorf("len(CustomScopes) = %d, want 0", len(app.CustomScopes))
	}
}

// ---------------------------------------------------------------------------
// Tests for normalizeStringList
// ---------------------------------------------------------------------------

func TestNormalizeStringList_Deduplicates(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b"}
	result := normalizeStringList(input)
	if len(result) != 3 {
		t.Fatalf("len = %d, want 3", len(result))
	}
	expected := []string{"a", "b", "c"}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %q, want %q", i, result[i], v)
		}
	}
}

func TestNormalizeStringList_TrimsAndRemovesEmpty(t *testing.T) {
	input := []string{"  hello  ", " ", "world", "  "}
	result := normalizeStringList(input)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0] != "hello" {
		t.Errorf("result[0] = %q, want %q", result[0], "hello")
	}
	if result[1] != "world" {
		t.Errorf("result[1] = %q, want %q", result[1], "world")
	}
}

func TestNormalizeStringList_NilInput(t *testing.T) {
	result := normalizeStringList(nil)
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestNormalizeStringList_EmptyInput(t *testing.T) {
	result := normalizeStringList([]string{})
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

// ---------------------------------------------------------------------------
// Tests for normalizeOptionalString
// ---------------------------------------------------------------------------

func TestNormalizeOptionalString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trims spaces", "  hello  ", "hello"},
		{"empty string", "", ""},
		{"only spaces", "   ", ""},
		{"no spaces", "hello", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeOptionalString(tt.input); got != tt.want {
				t.Errorf("normalizeOptionalString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helper: errorUUIDGenerator
// ---------------------------------------------------------------------------

type errorUUIDGenerator struct{}

func (g *errorUUIDGenerator) NewString() (string, error) {
	return "", errUUIDFailed
}

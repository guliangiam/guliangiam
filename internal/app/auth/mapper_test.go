package auth

import (
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

func TestToAccountContext(t *testing.T) {
	org := &orgdomain.Organization{
		ID: "org1", Slug: "myorg", DisplayName: "My Org", LogoURL: "logo.png", FaviconURL: "fav.ico",
	}
	user := &userdomain.User{
		Username: "alice", IsAdmin: true,
		Profile: userdomain.UserProfile{DisplayName: "Alice", AvatarURL: "avatar.png"},
	}
	ctx := toAccountContext(org, user, "token123")
	if ctx.Account.Owner != "myorg" || ctx.Account.Name != "alice" {
		t.Fatalf("unexpected: %+v", ctx)
	}
	if ctx.AccessToken != "token123" {
		t.Fatalf("got %s", ctx.AccessToken)
	}
}

func TestToAccountContext_NeedUpdatePassword(t *testing.T) {
	org := &orgdomain.Organization{ID: "org1", Slug: "myorg", DisplayName: "My Org"}
	tests := []struct {
		name  string
		flag  bool
		wants bool
	}{
		{"flagged", true, true},
		{"not_flagged", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &userdomain.User{
				Username:    "alice",
				Credentials: userdomain.UserCredentials{NeedUpdatePassword: tt.flag},
			}
			ctx := toAccountContext(org, user, "token")
			if ctx.NeedUpdatePassword != tt.wants {
				t.Fatalf("NeedUpdatePassword = %v, want %v", ctx.NeedUpdatePassword, tt.wants)
			}
			resp := toSuccessfulLoginResponse(ctx)
			if resp.NeedUpdatePassword != tt.wants {
				t.Fatalf("login NeedUpdatePassword = %v, want %v", resp.NeedUpdatePassword, tt.wants)
			}
		})
	}
}

func TestExtractNavItems(t *testing.T) {
	t.Run("empty_preferences_returns_nil", func(t *testing.T) {
		nav, user, order := extractNavItems(&orgdomain.Organization{})
		if nav != nil || user != nil || order != nil {
			t.Fatalf("expected nil slices, got %v %v %v", nav, user, order)
		}
	})

	t.Run("canonical_shape", func(t *testing.T) {
		org := &orgdomain.Organization{Settings: orgdomain.OrganizationSettings{
			NavPreferences: map[string]any{
				"navItems":      []any{"/users", "/roles"},
				"userNavItems":  []any{"/users"},
				"navGroupOrder": []any{"identity", "audit"},
			},
		}}
		nav, user, order := extractNavItems(org)
		if len(nav) != 2 || nav[0] != "/users" || nav[1] != "/roles" {
			t.Fatalf("navItems = %v", nav)
		}
		if len(user) != 1 || user[0] != "/users" {
			t.Fatalf("userNavItems = %v", user)
		}
		if len(order) != 2 || order[0] != "identity" {
			t.Fatalf("navGroupOrder = %v", order)
		}
	})

	t.Run("legacy_admin_nav_items_fallback", func(t *testing.T) {
		org := &orgdomain.Organization{Settings: orgdomain.OrganizationSettings{
			NavPreferences: map[string]any{
				"adminNavItems": []any{"/dashboard", "/users"},
			},
		}}
		nav, _, _ := extractNavItems(org)
		if len(nav) != 2 || nav[0] != "/dashboard" {
			t.Fatalf("legacy adminNavItems fallback = %v", nav)
		}
	})

	t.Run("no_system_union", func(t *testing.T) {
		// 配置了 navItems 后不得再并入系统级分组：未列出的路由键原样不下发。
		org := &orgdomain.Organization{Settings: orgdomain.OrganizationSettings{
			NavPreferences: map[string]any{"navItems": []any{"/users"}},
		}}
		nav, user, _ := extractNavItems(org)
		if len(nav) != 1 || nav[0] != "/users" {
			t.Fatalf("navItems = %v, want exactly [/users] without union", nav)
		}
		if user == nil || len(user) != 0 {
			t.Fatalf("userNavItems must be non-nil empty, got %v", user)
		}
	})
}

func TestToSessionIdentity(t *testing.T) {
	session := &sessiondomain.Session{OrganizationSlug: "org1", Username: "bob"}
	identity := toSessionIdentity(session)
	if identity.OrganizationSlug != "org1" || identity.Username != "bob" {
		t.Fatalf("unexpected: %+v", identity)
	}
}

func TestToSuccessfulLoginResponse(t *testing.T) {
	acct := AccountContext{
		Account:      SessionAccount{Name: "alice"},
		Organization: SessionOrganization{ID: "o1"},
		AccessToken:  "tok",
	}
	resp := toSuccessfulLoginResponse(acct)
	if resp.Status != LoginStatusSuccess || resp.AccessToken != "tok" {
		t.Fatalf("unexpected: %+v", resp)
	}
	if resp.Account == nil || resp.Account.Name != "alice" {
		t.Fatal("account not set")
	}
}

func TestLoginMfaMethodTarget(t *testing.T) {
	user := &userdomain.User{
		Profile:           userdomain.UserProfile{Email: "a@b.com", Phone: "123", CountryCode: "+1"},
		MfaRadiusUsername: "radius_user",
		MfaPushReceiver:   "push_recv",
	}
	tests := []struct {
		methodType      string
		wantTarget      string
		wantCountryCode string
	}{
		{"email", "a@b.com", ""},
		{"sms", "123", "+1"},
		{"radius", "radius_user", ""},
		{"push", "push_recv", ""},
		{"unknown", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.methodType, func(t *testing.T) {
			target, cc := loginMfaMethodTarget(user, tt.methodType)
			if target != tt.wantTarget || cc != tt.wantCountryCode {
				t.Fatalf("got (%s, %s), want (%s, %s)", target, cc, tt.wantTarget, tt.wantCountryCode)
			}
		})
	}
	t.Run("nil_user", func(t *testing.T) {
		target, cc := loginMfaMethodTarget(nil, "email")
		if target != "" || cc != "" {
			t.Fatalf("got (%s, %s)", target, cc)
		}
	})
}

func TestToMfaSetupRequiredLoginResponse(t *testing.T) {
	expires := time.Now()
	resp := toMfaSetupRequiredLoginResponse([]string{"totp", "sms"}, "ch1", expires)
	if resp.Status != LoginStatusMfaSetup {
		t.Fatalf("got %s", resp.Status)
	}
	if len(resp.RequiredMfa) != 2 || resp.ChallengeID != "ch1" {
		t.Fatalf("unexpected: %+v", resp)
	}
}

func TestToMfaRequiredLoginResponse(t *testing.T) {
	user := &userdomain.User{
		Profile: userdomain.UserProfile{Email: "a@b.com", Phone: "123", CountryCode: "+1"},
	}
	methods := []userdomain.UserMfaMethod{
		{ID: "m1", MethodType: "email", DisplayName: "Email"},
		{ID: "m2", MethodType: "sms", DisplayName: "SMS"},
	}
	expires := time.Now()
	resp := toMfaRequiredLoginResponse(user, "sms", methods, "ch1", "state1", expires)
	if resp.Status != LoginStatusMfaRequired {
		t.Fatalf("got %s", resp.Status)
	}
	if resp.NextMfa != "sms" {
		t.Fatalf("got %s", resp.NextMfa)
	}
	if len(resp.AvailableMfaMethods) != 2 {
		t.Fatalf("got %d", len(resp.AvailableMfaMethods))
	}
}

func TestResolveSessionPolicy(t *testing.T) {
	tests := []struct {
		name string
		org  *orgdomain.Organization
		want int
	}{
		{"default when nil", nil, 30},
		{"default when absent", &orgdomain.Organization{}, 30},
		{"configured minutes", &orgdomain.Organization{Security: orgdomain.OrganizationSecurityPolicy{SessionPolicy: map[string]any{"idleTimeoutMinutes": float64(15)}}}, 15},
		{"string minutes", &orgdomain.Organization{Security: orgdomain.OrganizationSecurityPolicy{SessionPolicy: map[string]any{"idleTimeoutMinutes": "45"}}}, 45},
		{"non-positive falls back", &orgdomain.Organization{Security: orgdomain.OrganizationSecurityPolicy{SessionPolicy: map[string]any{"idleTimeoutMinutes": float64(0)}}}, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveSessionPolicy(tt.org).IdleTimeoutMinutes; got != tt.want {
				t.Fatalf("IdleTimeoutMinutes = %d, want %d", got, tt.want)
			}
		})
	}
}

package idempotency

import "testing"

func TestScopeFor(t *testing.T) {
	tests := []struct {
		name    string
		orgSlug string
		userID  string
		path    string
		want    string
	}{
		{name: "org and user scoped", orgSlug: "acme", userID: "u-1", path: "/api/v1/orders", want: "acme:u-1:/api/v1/orders"},
		{name: "empty org keeps separators", orgSlug: "", userID: "u-1", path: "/api/v1/orders", want: ":u-1:/api/v1/orders"},
		{name: "empty user keeps separators", orgSlug: "acme", userID: "", path: "/api/v1/orders", want: "acme::/api/v1/orders"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScopeFor(tt.orgSlug, tt.userID, tt.path); got != tt.want {
				t.Fatalf("ScopeFor(%q, %q, %q) = %q, want %q", tt.orgSlug, tt.userID, tt.path, got, tt.want)
			}
		})
	}
}

// TestScopeFor_UsersCannotCollide pins BE-E-01: two users in the same tenant
// sending the same key to the same path must land in different scopes, and an
// empty identity component can never alias another combination.
func TestScopeFor_UsersCannotCollide(t *testing.T) {
	a := ScopeFor("acme", "user-a", "/api/v1/orders")
	b := ScopeFor("acme", "user-b", "/api/v1/orders")
	if a == b {
		t.Fatalf("scopes for different users collide: %q", a)
	}
	if ScopeFor("", "acme", "/p") == ScopeFor("acme", "", "/p") {
		t.Fatal("empty org/user combinations alias each other")
	}
}

package auth

import "testing"

func TestMatchPermissionOverride(t *testing.T) {
	overrides := []PermissionConfig{{
		Method:     "POST",
		Pattern:    "/api/v1/users/{id}/set-password",
		Permission: "users:password:reset",
	}}

	cases := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{"exact route", "POST", "/api/v1/users/01a1aca5-1bbd-7ecd-a450-f59fc333cb23/set-password", true},
		{"method mismatch", "PUT", "/api/v1/users/u1/set-password", false},
		{"other route", "POST", "/api/v1/users/u1", false},
		{"suffix only", "POST", "/api/v1/groups/g1/set-password", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := matchPermissionOverride(tc.method, tc.path, overrides)
			if ok != tc.want {
				t.Fatalf("match=%v want=%v", ok, tc.want)
			}
			if ok && got.Permission != "users:password:reset" {
				t.Fatalf("permission=%q", got.Permission)
			}
		})
	}
}

func TestSplitDeclaredPermission(t *testing.T) {
	resource, action := splitDeclaredPermission("users:password:reset")
	if resource != "users" || action != "password:reset" {
		t.Fatalf("got (%q,%q)", resource, action)
	}
	if resource, action := splitDeclaredPermission("users:read"); resource != "users" || action != "read" {
		t.Fatalf("got (%q,%q)", resource, action)
	}
}

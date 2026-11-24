package db

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func TestRLSRoleForMode(t *testing.T) {
	cases := []struct {
		mode tenantctx.TenantFilterMode
		want string
	}{
		{tenantctx.TenantFilterSuperAdmin, RLSRoleSuperAdmin},
		{tenantctx.TenantFilterSystem, RLSRoleSuperAdmin},
		{tenantctx.TenantFilterStrict, RLSRoleMember},
		{tenantctx.TenantFilterOptional, RLSRoleMember},
		{tenantctx.TenantFilterMode("unknown"), RLSRoleMember},
	}
	for _, c := range cases {
		if got := rlsRoleForMode(c.mode); got != c.want {
			t.Errorf("rlsRoleForMode(%q) = %q, want %q", c.mode, got, c.want)
		}
	}
}

func TestIsValidUUID(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"01a0ea76-0c00-7000-8000-000000000001", true},
		{"01a1488a-32a4-7367-b1a0-9addef5b40df", true},
		{"a1b2c3d4-e5f6-7890-abcd-ef1234567890", true},
		{"A1B2C3D4-E5F6-7890-ABCD-EF1234567890", true},
		{"", false},
		{"not-a-uuid", false},
		{"01a0ea76", false},
		{"01a0ea76-0c00-7000-8000", false},
		{"zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz", false},
		{"01a0ea76-0c00-7000-8000-00000000000g", false}, // last char 'g' not hex
	}
	for _, c := range cases {
		if got := isValidUUID(c.input); got != c.want {
			t.Errorf("isValidUUID(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

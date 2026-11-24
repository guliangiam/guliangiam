package tenantctx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTenantWhereClause_AllModes(t *testing.T) {
	cases := []struct {
		name       string
		mode       TenantFilterMode
		orgID      string
		orgName    string
		wantClause string
		wantArg    interface{}
		wantPanic  bool
	}{
		{"super_admin ignores tenant", TenantFilterSuperAdmin, "org-1", "acme", "", nil, false},
		{"system ignores tenant", TenantFilterSystem, "org-1", "acme", "", nil, false},
		{"cross filters target id", TenantFilterCross, "org-1", "", "organization_id = $1", "org-1", false},
		{"cross empty panics", TenantFilterCross, "", "", "", nil, true},
		{"strict with id", TenantFilterStrict, "org-1", "", "organization_id = $1", "org-1", false},
		{"strict with name", TenantFilterStrict, "", "acme", "organization_id = $1", "acme", false},
		{"strict empty panics", TenantFilterStrict, "", "", "", nil, true},
		{"optional empty skips filter", TenantFilterOptional, "", "", "", nil, false},
		{"optional with id", TenantFilterOptional, "org-1", "", "organization_id = $1", "org-1", false},
		{"zero mode behaves optional", TenantFilterMode(""), "", "", "", nil, false},
		{"zero mode with id", TenantFilterMode(""), "org-1", "", "organization_id = $1", "org-1", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithTenantScope(context.Background(), tc.orgID, tc.orgName, tc.mode)
			if tc.wantPanic {
				assert.Panics(t, func() { TenantWhereClause(ctx, "organization_id") })
				return
			}
			clause, arg := TenantWhereClause(ctx, "organization_id")
			assert.Equal(t, tc.wantClause, clause)
			assert.Equal(t, tc.wantArg, arg)
		})
	}
}

func TestTenantAndClause_AllModes(t *testing.T) {
	cases := []struct {
		name       string
		mode       TenantFilterMode
		orgID      string
		orgName    string
		wantClause string
		wantArg    interface{}
		wantPanic  bool
	}{
		{"super_admin ignores tenant", TenantFilterSuperAdmin, "org-1", "acme", "", nil, false},
		{"system ignores tenant", TenantFilterSystem, "org-1", "acme", "", nil, false},
		{"cross filters target id", TenantFilterCross, "org-1", "", "AND organization_id = $2", "org-1", false},
		{"cross empty panics", TenantFilterCross, "", "", "", nil, true},
		{"strict with id", TenantFilterStrict, "org-1", "", "AND organization_id = $2", "org-1", false},
		{"strict with name", TenantFilterStrict, "", "acme", "AND organization_id = $2", "acme", false},
		{"strict empty panics", TenantFilterStrict, "", "", "", nil, true},
		{"optional empty skips filter", TenantFilterOptional, "", "", "", nil, false},
		{"optional with name", TenantFilterOptional, "", "acme", "AND organization_id = $2", "acme", false},
		{"zero mode behaves optional", TenantFilterMode(""), "", "", "", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithTenantScope(context.Background(), tc.orgID, tc.orgName, tc.mode)
			if tc.wantPanic {
				assert.Panics(t, func() { TenantAndClause(ctx, "organization_id", 2) })
				return
			}
			clause, arg := TenantAndClause(ctx, "organization_id", 2)
			assert.Equal(t, tc.wantClause, clause)
			assert.Equal(t, tc.wantArg, arg)
		})
	}
}

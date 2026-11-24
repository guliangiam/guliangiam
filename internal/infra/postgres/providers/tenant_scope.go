package providers

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// providerTenantID returns the organization that constrains by-ID SQL.
// super_admin/system deliberately keep the all-tenant behavior, and a
// background context without a scope keeps the pre-existing semantics.
// HTTP requests are injected with strict scope by TenantFilterMiddleware; that
// is where the predicate must hold on dialects without RLS (MySQL).
func providerTenantID(ctx context.Context) string {
	scope := tenantctx.TenantScopeFromContext(ctx)
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		return ""
	}
	return scope.OrganizationID
}

package tenant_instances

import (
	"context"
	"time"
)

// Repository persists tenant isolation topology rows.
type Repository interface {
	Create(ctx context.Context, instance *TenantInstance) error
	GetPrimaryByOrganization(ctx context.Context, organizationID string) (*TenantInstance, error)
	// DecommissionByOrganization decommissions every live topology row of a
	// tenant. It is idempotent: already decommissioned rows are untouched.
	DecommissionByOrganization(ctx context.Context, organizationID string, at time.Time) error
	// PrimaryRegion resolves the default region from deployment_regions using
	// a stable ordering. No environment variable participates in the decision.
	PrimaryRegion(ctx context.Context) (*Region, error)
}

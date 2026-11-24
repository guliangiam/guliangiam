package tenant_identities

import (
	"context"
	"time"
)

// Repository persists the 1:1 tenant identity subject.
type Repository interface {
	Create(ctx context.Context, identity *TenantIdentity) error
	GetByOrganization(ctx context.Context, organizationID string) (*TenantIdentity, error)
	// Decommission flips status to decommissioned and stamps decommissioned_at.
	// It is idempotent: rows already decommissioned are left untouched.
	Decommission(ctx context.Context, organizationID string, at time.Time) error
}

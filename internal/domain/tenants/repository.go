// Package tenants exposes the tenant identity and isolation topology ports as
// one provisioning surface. Application code depends on this interface only;
// no SQL or infrastructure type leaks upward.
package tenants

import (
	"context"
	"time"

	tenantidentities "github.com/guliangiam/guliangiam/internal/domain/tenant_identities"
	tenantinstances "github.com/guliangiam/guliangiam/internal/domain/tenant_instances"
)

// Repository is the provisioning/retirement port for tenant identities and
// topology. CreateOrganization must call CreateIdentity and
// CreatePrimaryTopology inside the same transaction that writes the
// organization row (see contracts/registries/tenant-isolation.yaml).
type Repository interface {
	CreateIdentity(ctx context.Context, identity *tenantidentities.TenantIdentity) error
	GetIdentity(ctx context.Context, organizationID string) (*tenantidentities.TenantIdentity, error)
	DecommissionIdentity(ctx context.Context, organizationID string, at time.Time) error

	CreatePrimaryTopology(ctx context.Context, instance *tenantinstances.TenantInstance) error
	GetPrimaryTopology(ctx context.Context, organizationID string) (*tenantinstances.TenantInstance, error)
	DecommissionTopologies(ctx context.Context, organizationID string, at time.Time) error

	PrimaryRegion(ctx context.Context) (*tenantinstances.Region, error)
}

package tenants

import (
	"context"
	"database/sql"
	"time"

	tenantidentities "github.com/guliangiam/guliangiam/internal/domain/tenant_identities"
	tenantinstances "github.com/guliangiam/guliangiam/internal/domain/tenant_instances"
	"github.com/guliangiam/guliangiam/internal/domain/tenants"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// Repository implements the tenants port over PostgreSQL. All statements run
// on the caller-provided DBTX so the provisioning writes stay in the
// organization creation transaction (UnitOfWork pattern).
type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) CreateIdentity(ctx context.Context, identity *tenantidentities.TenantIdentity) error {
	_, err := r.db.Exec(ctx, queryInsertIdentity,
		identity.OrganizationID, identity.Status, identity.IsolationTier, identity.IsBuiltIn, identity.LastError,
		identity.ProvisionedAt, identity.DecommissionedAt, identity.CreatedAt, identity.UpdatedAt, identity.Revision,
	)
	return err
}

func (r Repository) GetIdentity(ctx context.Context, organizationID string) (*tenantidentities.TenantIdentity, error) {
	var identity tenantidentities.TenantIdentity
	var provisionedAt, decommissionedAt *time.Time
	err := r.db.QueryRow(ctx, queryGetIdentity, organizationID).Scan(
		&identity.OrganizationID, &identity.Status, &identity.IsolationTier, &identity.IsBuiltIn, &identity.LastError,
		&provisionedAt, &decommissionedAt, &identity.CreatedAt, &identity.UpdatedAt, &identity.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("tenant identity not found")
		}
		return nil, err
	}
	identity.ProvisionedAt = provisionedAt
	identity.DecommissionedAt = decommissionedAt
	return &identity, nil
}

func (r Repository) DecommissionIdentity(ctx context.Context, organizationID string, at time.Time) error {
	_, err := r.db.Exec(ctx, queryDecommissionIdentity, organizationID, at)
	return err
}

func (r Repository) CreatePrimaryTopology(ctx context.Context, instance *tenantinstances.TenantInstance) error {
	_, err := r.db.Exec(ctx, queryInsertTopology,
		instance.ID, instance.OrganizationID, instance.RegionID, instance.RegionCode, instance.IsPrimary, instance.Status,
		instance.PlacementKind, instance.StorageBoundary, instance.BackupBoundary, instance.DataResidencyCountry,
		instance.CapacityQuotaUnits, instance.ProvisionedAt, instance.EffectiveFrom, instance.CreatedAt, instance.UpdatedAt, instance.Revision,
	)
	return err
}

func (r Repository) GetPrimaryTopology(ctx context.Context, organizationID string) (*tenantinstances.TenantInstance, error) {
	var instance tenantinstances.TenantInstance
	var provisionedAt, effectiveFrom *time.Time
	err := r.db.QueryRow(ctx, queryGetPrimaryTopology, organizationID).Scan(
		&instance.ID, &instance.OrganizationID, &instance.RegionID, &instance.RegionCode, &instance.IsPrimary, &instance.Status,
		&instance.PlacementKind, &instance.StorageBoundary, &instance.BackupBoundary, &instance.DataResidencyCountry,
		&instance.CapacityQuotaUnits, &provisionedAt, &effectiveFrom, &instance.CreatedAt, &instance.UpdatedAt, &instance.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("tenant primary topology not found")
		}
		return nil, err
	}
	instance.ProvisionedAt = provisionedAt
	instance.EffectiveFrom = effectiveFrom
	return &instance, nil
}

func (r Repository) DecommissionTopologies(ctx context.Context, organizationID string, at time.Time) error {
	_, err := r.db.Exec(ctx, queryDecommissionTopologies, organizationID, at)
	return err
}

func (r Repository) PrimaryRegion(ctx context.Context) (*tenantinstances.Region, error) {
	var region tenantinstances.Region
	err := r.db.QueryRow(ctx, queryPrimaryRegion).Scan(&region.ID, &region.RegionCode, &region.CountryCode)
	if err != nil {
		if err == pgx.ErrNoRows || err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("no primary deployment region is configured")
		}
		return nil, err
	}
	return &region, nil
}

// Compile-time assertion against the domain port.
var _ tenants.Repository = Repository{}

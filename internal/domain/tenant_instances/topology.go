package tenant_instances

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Topology lifecycle states. state_generated.go owns the legal value set.
const (
	StatusPending        = string(TenantInstancesStatePending_State)
	StatusActive         = string(TenantInstancesStateActive_State)
	StatusFailed         = string(TenantInstancesStateFailed_State)
	StatusDecommissioned = string(TenantInstancesStateDecommissioned_State)

	PlacementSharedCluster = "shared_cluster"
	BoundaryShared         = "shared"
	BoundaryTenant         = "tenant"
)

// TenantInstance describes where a tenant's data physically lives. In the
// single-schema deployment only the metadata differs; RLS predicates never
// read these columns.
type TenantInstance struct {
	ID                   string
	OrganizationID       string
	RegionID             string
	RegionCode           string
	IsPrimary            bool
	Status               string
	PlacementKind        string
	StorageBoundary      string
	BackupBoundary       string
	DataResidencyCountry string
	CapacityQuotaUnits   int64
	ProvisionedAt        *time.Time
	EffectiveFrom        *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Revision             int64
}

// Region is the resolved deployment region used as the topology default.
type Region struct {
	ID          string
	RegionCode  string
	CountryCode string
}

// Validate checks the topology row against the lifecycle and placement model.
func (t *TenantInstance) Validate() error {
	if t.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if t.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if t.RegionID == "" || t.RegionCode == "" {
		return apperrors.NewInvalidArgument("regionId and regionCode are required")
	}
	if !IsValid(t.Status) {
		return apperrors.NewUnprocessableEntityf("status %q is not a valid tenant instance state", t.Status)
	}
	if t.PlacementKind != PlacementSharedCluster {
		return apperrors.NewUnprocessableEntityf("placementKind %q is not supported; only %q is available", t.PlacementKind, PlacementSharedCluster)
	}
	if t.StorageBoundary != BoundaryShared && t.StorageBoundary != BoundaryTenant {
		return apperrors.NewUnprocessableEntityf("storageBoundary %q is not supported", t.StorageBoundary)
	}
	if t.BackupBoundary != BoundaryShared && t.BackupBoundary != BoundaryTenant {
		return apperrors.NewUnprocessableEntityf("backupBoundary %q is not supported", t.BackupBoundary)
	}
	if t.DataResidencyCountry != "" && len(t.DataResidencyCountry) != 2 {
		return apperrors.NewUnprocessableEntity("dataResidencyCountry must be an ISO 3166-1 alpha-2 code")
	}
	return nil
}

// TransitionTo applies the generated state machine in memory.
func (t *TenantInstance) TransitionTo(next string, at time.Time) error {
	if !IsValid(next) {
		return apperrors.NewUnprocessableEntityf("status %q is not a valid tenant instance state", next)
	}
	if !CanTransitionTo(TenantInstancesState(t.Status), TenantInstancesState(next)) {
		return apperrors.NewConflictf("tenant instance cannot transition from %q to %q", t.Status, next)
	}
	t.Status = next
	t.UpdatedAt = at
	t.Revision++
	return nil
}

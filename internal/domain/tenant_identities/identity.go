package tenant_identities

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Tenant identity lifecycle states. The generated state machine in
// state_generated.go is the single source of truth for legal values.
const (
	StatusProvisioning   = string(TenantIdentitiesStateProvisioning_State)
	StatusActive         = string(TenantIdentitiesStateActive_State)
	StatusFailed         = string(TenantIdentitiesStateFailed_State)
	StatusDecommissioned = string(TenantIdentitiesStateDecommissioned_State)

	// IsolationTierRow is the only tier expressible in the single-schema +
	// RLS deployment. Schema/database tiers require a new ADR.
	IsolationTierRow = "row"
)

// TenantIdentity is the stable tenant subject bound 1:1 to an organization.
// OrganizationID is the tenant_id: no second identifier source is introduced.
type TenantIdentity struct {
	OrganizationID   string
	Status           string
	IsolationTier    string
	IsBuiltIn        bool
	LastError        string
	ProvisionedAt    *time.Time
	DecommissionedAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Revision         int64
}

// Validate checks the identity fields against the lifecycle model.
func (t *TenantIdentity) Validate() error {
	if t.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if t.IsolationTier == "" {
		return apperrors.NewUnprocessableEntity("isolationTier is required")
	}
	if t.IsolationTier != IsolationTierRow {
		return apperrors.NewUnprocessableEntityf("isolationTier %q is not supported; only %q is available", t.IsolationTier, IsolationTierRow)
	}
	if !IsValid(t.Status) {
		return apperrors.NewUnprocessableEntityf("status %q is not a valid tenant identity state", t.Status)
	}
	return nil
}

// TransitionTo applies the generated state machine to the in-memory entity.
// Persistence layers must additionally guard the transition in SQL (WHERE
// status <> 'decommissioned') so concurrent decommissions stay idempotent.
func (t *TenantIdentity) TransitionTo(next string, at time.Time) error {
	if !IsValid(next) {
		return apperrors.NewUnprocessableEntityf("status %q is not a valid tenant identity state", next)
	}
	if !CanTransitionTo(TenantIdentitiesState(t.Status), TenantIdentitiesState(next)) {
		return apperrors.NewConflictf("tenant identity cannot transition from %q to %q", t.Status, next)
	}
	t.Status = next
	switch next {
	case StatusActive:
		t.ProvisionedAt = &at
	case StatusDecommissioned:
		t.DecommissionedAt = &at
	}
	t.UpdatedAt = at
	t.Revision++
	return nil
}

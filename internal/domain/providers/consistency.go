package providers

import (
	"context"
	"time"
)

// RegistrationConsistency is the control-plane row used by the phase-C
// read-only consistency checker. Status and DetailID come from the stage-A
// registry metadata; Classified reports whether all three V13 columns are set.
type RegistrationConsistency struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Family         Family
	Kind           Kind
	Variant        Variant
	Classified     bool
	Status         Status
	DetailID       string
	Enabled        bool
	DeletedAt      *time.Time
}

// DetailConsistency is a native family row. Fields not used by a family stay
// zero-valued; notification uses Enabled because it has no status/deleted_at.
type DetailConsistency struct {
	ID             string
	ProviderID     string
	OrganizationID string
	Name           string
	DisplayName    string
	Kind           string
	Variant        string
	Status         string
	Enabled        bool
	DeletedAt      *time.Time
}

// ConsistencyRepository is intentionally separate from Repository: it is a
// read-only reporting port and does not expand the CRUD contract or its mocks.
type ConsistencyRepository interface {
	ListConsistencyRegistrations(ctx context.Context) ([]RegistrationConsistency, error)
	ListConsistencyDetails(ctx context.Context, family Family) ([]DetailConsistency, error)
}

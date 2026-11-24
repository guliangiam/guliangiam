package applications

import (
	"context"
	"time"
)

// ClientJWKSURI points a private_key_jwt client at a remotely hosted JWKS.
// Kept in its own table so the 114-column application aggregate statements do
// not change and the URI remains optional (OPEN-API-61).
type ClientJWKSURI struct {
	ApplicationID  string
	OrganizationID string
	JWKSURI        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ClientJWKSURIRepository is implemented by the PostgreSQL applications
// repository; app-layer callers use a narrow capability assertion so other
// repositories/fakes do not have to implement it.
type ClientJWKSURIRepository interface {
	SetClientJWKSURI(ctx context.Context, organizationID, applicationID, jwksURI string, now time.Time) error
	GetClientJWKSURI(ctx context.Context, organizationID, applicationID string) (*ClientJWKSURI, error)
	DeleteClientJWKSURI(ctx context.Context, organizationID, applicationID string) error
}

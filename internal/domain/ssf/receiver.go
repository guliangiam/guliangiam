// Package ssf defines CAEP/SSF (Shared Signals Framework) receiver
// registration: endpoints that receive signed security event tokens.
package ssf

import (
	"context"
	"time"
)

// Receiver is one registered SSF event receiver.
type Receiver struct {
	ID             string
	OrganizationID string
	Name           string
	ReceiverURL    string
	AuthTokenHash  string
	EventTypes     []byte
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CreatedBy      *string
	Revision       int64
}

// Repository stores SSF receiver registrations.
type Repository interface {
	Create(ctx context.Context, receiver *Receiver) error
	GetByID(ctx context.Context, organizationID, id string) (*Receiver, error)
	ListByOrganization(ctx context.Context, organizationID string) ([]*Receiver, error)
	Delete(ctx context.Context, organizationID, id string) error
}

package idv

import (
	"context"
	"encoding/json"
	"time"
)

// Verification is the durable record of an identity-verification attempt.
// It outlives the outbound provider session and lets us surface audit trails
// and compliance evidence long after the provider session expires.
type Verification struct {
	ID             string
	OrganizationID string
	UserID         *string
	Provider       string
	Status         VerificationStatus
	RedirectURL    *string
	SessionToken   *string
	ProviderRef    *string // provider-side reference (session id / verification id)
	BizSeq         *string // merchant business sequence (unique; idempotency / anti-replay)
	Action         *string // FaceAction semantics; nil = legacy row
	ProductCode    *string // provider product used for this session
	SubjectRef     *string // salted SM3 hex of the claimed ID number (de-identified; never plaintext)
	FirstName      *string
	LastName       *string
	DateOfBirth    *string
	Nationality    *string
	IDType         *string
	IDNumber       *string
	Confidence     *float64
	Reason         *string
	RawResult      json.RawMessage
	InitiatedAt    time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ListQuery filters paginated verification records.
type ListQuery struct {
	OrganizationID string
	UserID         *string
	Provider       *string
	Status         *VerificationStatus
	Page           int
	PageSize       int
}

// CreateCommand is the contract for persisting a newly initiated verification.
type CreateCommand struct {
	OrganizationID string
	UserID         *string
	Provider       string
	Status         VerificationStatus
	RedirectURL    *string
	SessionToken   *string
	ProviderRef    *string
	BizSeq         *string
	Action         *string
	ProductCode    *string
	SubjectRef     *string
}

// CompleteCommand records the provider result against an existing verification.
type CompleteCommand struct {
	ID          string
	Status      VerificationStatus
	FirstName   *string
	LastName    *string
	DateOfBirth *string
	Nationality *string
	IDType      *string
	IDNumber    *string
	Confidence  *float64
	Reason      *string
	RawResult   json.RawMessage
	ProviderRef *string
}

// Repository is the persistence contract for identity-verification records.
// All methods enforce multi-tenancy — callers must supply organization_id
// explicitly on every write and on every query that needs filtering.
type Repository interface {
	Create(ctx context.Context, v *Verification) error
	Update(ctx context.Context, v *Verification) error
	GetByID(ctx context.Context, id, organizationID string) (*Verification, error)
	GetByProviderRef(ctx context.Context, provider, providerRef string) (*Verification, error)
	List(ctx context.Context, q ListQuery) ([]*Verification, int64, error)
	Delete(ctx context.Context, id, organizationID string) error
}

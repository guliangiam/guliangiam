package privacy

import (
	"context"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ConsentRecord is one immutable version of a data subject's consent for a
// purpose (guliangiam.consent_records, AC-PRIV-002). Versions are append-only:
// a new grant writes version N+1; withdrawal stamps withdrawn_at on the latest
// version instead of deleting history.
type ConsentRecord struct {
	ID             string
	OrganizationID string
	SubjectType    string
	SubjectID      string
	Purpose        string
	Version        int
	GrantedAt      time.Time
	WithdrawnAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// ValidateSubject checks the fields that must be present before the repository
// computes the next version (grant) or looks up the latest row (withdraw).
func (c *ConsentRecord) ValidateSubject() error {
	probe := *c
	probe.Version = 1
	return probe.Validate()
}

// Validate enforces the registry-compatible field set and CHECK constraints.
func (c *ConsentRecord) Validate() error {
	if c.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if c.SubjectType != SubjectTypeUser && c.SubjectType != SubjectTypeContact {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid subjectType",
			map[string]any{"field": "subjectType", "value": c.SubjectType, "allowed": subjectTypes})
	}
	if c.SubjectID == "" {
		return apperrors.NewInvalidArgument("subjectId is required")
	}
	if c.Purpose == "" {
		return apperrors.NewInvalidArgument("purpose is required")
	}
	if c.Version < 1 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "version must be >= 1",
			map[string]any{"field": "version", "value": c.Version})
	}
	if c.WithdrawnAt != nil && c.WithdrawnAt.Before(c.GrantedAt) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "withdrawnAt must not precede grantedAt",
			map[string]any{"field": "withdrawnAt"})
	}
	return nil
}

// ConsentQuery scopes a consent version-chain read.
type ConsentQuery struct {
	SubjectType string
	SubjectID   string
	Purpose     string
	Limit       int
}

// ConsentRepository is the transaction-scoped port for consent_records.
type ConsentRepository interface {
	List(ctx context.Context, organizationID string, query ConsentQuery) ([]*ConsentRecord, error)
	// LockSubject serializes concurrent version-chain writes for one
	// (organization, subject, purpose) within the enclosing transaction.
	// Grant must call it before Latest so two concurrent grants cannot read
	// the same latest version and both append N+1 (BE-D-22c).
	LockSubject(ctx context.Context, organizationID, subjectType, subjectID, purpose string) error
	Latest(ctx context.Context, organizationID, subjectType, subjectID, purpose string) (*ConsentRecord, error)
	Create(ctx context.Context, record *ConsentRecord) error
	WithdrawLatest(ctx context.Context, organizationID, subjectType, subjectID, purpose string, at time.Time) (*ConsentRecord, error)
}

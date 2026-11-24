package mfachallenges

import "time"

const (
	StatePending     = "pending"
	StatePushPending = "push_pending"
	StateApproved    = "approved"
	StateRejected    = "rejected"
)

// MetadataKeyFailedAttempts mirrors FailedAttempts into the JSONB metadata bag
// so the per-challenge failure count survives reloading the row.
const MetadataKeyFailedAttempts = "mfa_failed_attempts"

type Challenge struct {
	ID             string
	OrganizationID string
	UserID         string
	ApplicationID  string
	State          string
	MethodType     string
	FailedAttempts int
	// Metadata is the factor-specific JSONB state (TOTP window, WebAuthn cred
	// ID, failure counter). It maps to login_challenges.metadata.
	Metadata   map[string]any
	ExpiresAt  time.Time
	VerifiedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Revision   int64
}

// RecordFailure increments the persisted failure counter and stamps the
// transition time. The counter is mirrored into Metadata so callers that
// reload the challenge see the same value.
func (c *Challenge) RecordFailure(now time.Time) {
	c.FailedAttempts++
	c.UpdatedAt = now
	if c.Metadata == nil {
		c.Metadata = map[string]any{}
	}
	c.Metadata[MetadataKeyFailedAttempts] = c.FailedAttempts
}

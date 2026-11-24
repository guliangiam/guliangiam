package mfachallenges

import "time"

type challengeRow struct {
	ID             string
	OrganizationID string
	UserID         string
	ApplicationID  string
	State          string
	MethodType     string
	Metadata       []byte
	ExpiresAt      time.Time
	VerifiedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

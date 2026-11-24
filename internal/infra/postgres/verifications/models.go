package verifications

import "time"

type verificationRow struct {
	ID             string
	OrganizationID string
	Type           string
	Recipient      string
	Code           string
	Status         string
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

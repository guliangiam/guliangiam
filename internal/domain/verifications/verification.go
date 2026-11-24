package verifications

import "time"

type Verification struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Type           string    `json:"type"`
	Recipient      string    `json:"recipient"`
	Code           string    `json:"code"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expiresAt"`
	CreatedAt      time.Time `json:"createdAt"`
}

package pricings

import "time"

type Pricing struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Description    string    `json:"description,omitempty"`
	Plans          []string  `json:"plans,omitempty"`
	IsEnabled      bool      `json:"isEnabled"`
	TrialDuration  int       `json:"trialDuration"`
	Application    string    `json:"application,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Revision       int64     `json:"revision"`
}

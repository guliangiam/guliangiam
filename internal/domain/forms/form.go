package forms

import "time"

type Form struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Description    string    `json:"description,omitempty"`
	Type           string    `json:"type,omitempty"`
	Properties     string    `json:"properties,omitempty"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Revision       int64     `json:"revision"`
}

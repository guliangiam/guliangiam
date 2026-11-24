package models

import "time"

type ModelListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Description    string    `json:"description,omitempty"`
	Owner          string    `json:"owner,omitempty"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type ModelDetail struct {
	ModelListItem
	ModelText string `json:"modelText"`
	Revision  int64  `json:"revision"`
}

package enforcers

import "time"

type EnforcerListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	ModelID        string    `json:"modelId"`
	AdapterID      string    `json:"adapterId"`
	IsEnabled      bool      `json:"isEnabled"`
	Owner          string    `json:"owner,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type EnforcerDetail struct {
	EnforcerListItem
	Description string `json:"description,omitempty"`
	Revision    int64  `json:"revision"`
}

type EnforceResult struct {
	Allow bool `json:"allow"`
}

type BatchEnforceResult struct {
	Results []bool `json:"results"`
}

package roles

import "time"

type RoleListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Description    string    `json:"description"`
	GroupIDs       []string  `json:"groupIds"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type RoleDetail struct {
	RoleListItem
	Users    []string `json:"users,omitempty"`
	RoleIDs  []string `json:"roleIds"`
	Domains  []string `json:"domains"`
	Revision int64    `json:"revision"`
}

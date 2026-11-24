package groups

import "time"

type GroupListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Type           string    `json:"type"`
	ParentID       string    `json:"parentId"`
	IsTopGroup     bool      `json:"isTopGroup"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type GroupDetail struct {
	GroupListItem
	Manager      string   `json:"manager"`
	ContactEmail string   `json:"contactEmail"`
	Homepage     string   `json:"homepage,omitempty"`
	Users        []string `json:"users,omitempty"`
	Revision     int64    `json:"revision"`
}

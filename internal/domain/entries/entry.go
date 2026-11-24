package entries

import "time"

type Entry struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Provider       string    `json:"provider,omitempty"`
	Application    string    `json:"application,omitempty"`
	Type           string    `json:"type,omitempty"`
	ClientIP       string    `json:"clientIp,omitempty"`
	UserAgent      string    `json:"userAgent,omitempty"`
	Message        string    `json:"message,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Revision       int64     `json:"revision"`
}

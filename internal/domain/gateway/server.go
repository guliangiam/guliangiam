package gateway

import "time"

type Server struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	Name           string         `json:"name"`
	Endpoint       string         `json:"endpoint"`
	Protocol       string         `json:"protocol"`
	Status         string         `json:"status"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      *time.Time     `json:"deletedAt,omitempty"`
	Revision       int64          `json:"revision"`
}

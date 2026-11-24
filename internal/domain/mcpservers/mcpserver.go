package mcpservers

import "time"

type McpServer struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	URL            string    `json:"url,omitempty"`
	Token          string    `json:"token,omitempty"`
	Application    string    `json:"application,omitempty"`
	Tools          []Tool    `json:"tools,omitempty"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Revision       int64     `json:"revision"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
	IsAllowed   bool                   `json:"isAllowed"`
}

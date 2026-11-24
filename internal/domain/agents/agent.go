package agents

import "time"

type Agent struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Type           string // gateway-proxy, sync-agent, etc.
	Host           string
	Port           int
	Url            string // Agent service URL (e.g. for MCP proxying)
	Token          string // Agent authentication token (masked in API responses)
	ApplicationID  string // Associated application ID
	IsEnabled      bool
	LastPingAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

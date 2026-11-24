package gateway

import "context"

type ListServersQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
}

type ListSitesQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
}

type Repository interface {
	// GetServerByID is the unscoped, system-scoped lookup reserved for
	// infrastructure diagnostics and repair tooling. Request-scoped by-ID paths
	// must use GetServerByIDForOrganization so a caller-supplied id cannot
	// cross tenants.
	GetServerByID(ctx context.Context, id string) (*Server, error)
	// GetServerByIDForOrganization returns the server only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetServerByIDForOrganization(ctx context.Context, id, organizationID string) (*Server, error)
	ListServers(ctx context.Context, query ListServersQuery) ([]*Server, int64, error)
	CreateServer(ctx context.Context, server *Server) error
	UpdateServer(ctx context.Context, server *Server) error
	DeleteServer(ctx context.Context, id string) error

	// GetSiteByID is the unscoped, system-scoped lookup reserved for
	// infrastructure diagnostics and repair tooling. Request-scoped by-ID paths
	// must use GetSiteByIDForOrganization so a caller-supplied id cannot cross
	// tenants.
	GetSiteByID(ctx context.Context, id string) (*Site, error)
	// GetSiteByIDForOrganization returns the site only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetSiteByIDForOrganization(ctx context.Context, id, organizationID string) (*Site, error)
	ListSites(ctx context.Context, query ListSitesQuery) ([]*Site, int64, error)
	CreateSite(ctx context.Context, site *Site) error
	UpdateSite(ctx context.Context, site *Site) error
	DeleteSite(ctx context.Context, id string) error
}

type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type MCPProxy interface {
	ListTools(ctx context.Context) ([]MCPTool, error)
	CallTool(ctx context.Context, serverID, toolName string, arguments map[string]interface{}) (interface{}, error)
	SyncAllTools(ctx context.Context) error
}

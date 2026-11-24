package mcpservers

import (
	"context"

	appmcpservers "github.com/guliangiam/guliangiam/internal/app/mcpservers"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// McpTool mirrors the domain Tool type for transport-layer decoupling.
type McpTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
	IsAllowed   bool                   `json:"isAllowed"`
}

func (t McpTool) toDTO() appmcpservers.ToolDTO {
	return appmcpservers.ToolDTO{
		Name:        t.Name,
		Description: t.Description,
		InputSchema: t.InputSchema,
		IsAllowed:   t.IsAllowed,
	}
}

type Handler struct {
	Service appmcpservers.Service
}

func NewHandler(s appmcpservers.Service) Handler {
	return Handler{Service: s}
}

type ListMcpServersResponse = presenter.ListResponse[appmcpservers.McpServerListItem]
type McpServerResponse = appmcpservers.McpServerDetail

type ListMcpServersRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type GetMcpServerRequest struct {
	ID string
}

type CreateMcpServerRequest struct {
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	URL            string    `json:"url,omitempty"`
	Token          string    `json:"token,omitempty"`
	Application    string    `json:"application,omitempty"`
	Tools          []McpTool `json:"tools,omitempty"`
	IsEnabled      bool      `json:"isEnabled"`
}

type PatchMcpServerRequest struct {
	DisplayName *string   `json:"displayName,omitempty"`
	URL         *string   `json:"url,omitempty"`
	Token       *string   `json:"token,omitempty"`
	Application *string   `json:"application,omitempty"`
	Tools       []McpTool `json:"tools,omitempty"`
	IsEnabled   *bool     `json:"isEnabled,omitempty"`
	Revision    int64     `json:"revision,omitempty"`
}

func (r CreateMcpServerRequest) toCommand() appmcpservers.CreateMcpServerCommand {
	tools := make([]appmcpservers.ToolDTO, len(r.Tools))
	for i, t := range r.Tools {
		tools[i] = t.toDTO()
	}
	return appmcpservers.CreateMcpServerCommand{
		OrganizationID: r.OrganizationID, Name: r.Name, DisplayName: r.DisplayName,
		URL: r.URL, Token: r.Token, Application: r.Application,
		Tools: tools, IsEnabled: r.IsEnabled,
	}
}

func (r PatchMcpServerRequest) toCommand(id string) appmcpservers.PatchMcpServerCommand {
	tools := make([]appmcpservers.ToolDTO, len(r.Tools))
	for i, t := range r.Tools {
		tools[i] = t.toDTO()
	}
	return appmcpservers.PatchMcpServerCommand{
		ID: id, DisplayName: r.DisplayName, URL: r.URL, Token: r.Token,
		Application: r.Application, Tools: tools, IsEnabled: r.IsEnabled,
		Revision: r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListMcpServersRequest) (ListMcpServersResponse, error) {
	items, total, err := h.Service.ListMcpServers(ctx, appmcpservers.ListMcpServersQuery{Page: request.Page, PageSize: request.PageSize, OrganizationID: request.OrganizationID, Keyword: request.Keyword, IsEnabled: request.IsEnabled})
	if err != nil {
		return ListMcpServersResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetMcpServerRequest) (*McpServerResponse, error) {
	response, err := h.Service.GetMcpServer(ctx, organizationID, request.ID)
	if err != nil {
		return nil, err
	}
	typed := McpServerResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateMcpServerRequest) error {
	return h.Service.CreateMcpServer(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchMcpServerRequest) error {
	return h.Service.PatchMcpServer(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteMcpServer(ctx, organizationID, id)
}

// GetOnlineServers returns MCP servers from a remote registry.
func (h Handler) GetOnlineServers(ctx context.Context, registryURL string) ([]appmcpservers.OnlineServer, error) {
	return h.Service.GetOnlineServers(ctx, registryURL)
}

// SyncIntranetServers scans the intranet for MCP servers.
func (h Handler) SyncIntranetServers(ctx context.Context, req appmcpservers.SyncIntranetServersRequest) (*appmcpservers.SyncIntranetServersResult, error) {
	return h.Service.SyncIntranetServers(ctx, req)
}

// SyncTools syncs tools from an MCP server inside the trusted tenant.
func (h Handler) SyncTools(ctx context.Context, organizationID, serverID string) (*appmcpservers.SyncToolsResult, error) {
	return h.Service.SyncTools(ctx, organizationID, serverID)
}

// ProxyRequest proxies a tool invocation to an MCP server inside the trusted
// tenant. userID is the validated session user and is used for the tool-level
// decision; it is never sourced from the request body or query.
func (h Handler) ProxyRequest(ctx context.Context, organizationID, serverName, toolName string, arguments map[string]any, userID string) (*appmcpservers.ProxyRequestResult, error) {
	return h.Service.ProxyRequest(ctx, organizationID, serverName, toolName, arguments, userID)
}

// GetAccessToken issues a short-lived MCP access token for the authenticated
// session user of the trusted organization. The transport layer resolves userID
// from the validated session; callers cannot select another subject.
func (h Handler) GetAccessToken(ctx context.Context, organizationID, serverID, userID string) (*appmcpservers.AccessTokenResult, error) {
	return h.Service.GetAccessToken(ctx, organizationID, serverID, userID)
}

// SyncIntranetServersRequest is the HTTP request body for intranet scanning.
type SyncIntranetServersHTTPRequest struct {
	OrganizationID string   `json:"organizationId"`
	CIDRs          []string `json:"cidrs"`
	Ports          []int    `json:"ports"`
	Concurrency    int      `json:"concurrency,omitempty"`
}

// ProxyServerHTTPRequest is the HTTP request body for proxying tool calls.
type ProxyServerHTTPRequest struct {
	ToolName  string         `json:"toolName"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

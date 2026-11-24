package agents

import (
	"context"

	appagents "github.com/guliangiam/guliangiam/internal/app/agents"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appagents.Service
}

func NewHandler(s appagents.Service) Handler {
	return Handler{Service: s}
}

type ListAgentsResponse = presenter.ListResponse[appagents.AgentListItem]
type AgentResponse = appagents.AgentDetail

type ListAgentsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type CreateAgentRequest struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Type           string `json:"type"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Url            string `json:"url,omitempty"`
	Token          string `json:"token,omitempty"`
	ApplicationID  string `json:"applicationId,omitempty"`
	IsEnabled      *bool  `json:"isEnabled,omitempty"`
}

type PatchAgentRequest struct {
	DisplayName   *string `json:"displayName,omitempty"`
	Host          *string `json:"host,omitempty"`
	Port          *int    `json:"port,omitempty"`
	Url           *string `json:"url,omitempty"`
	Token         *string `json:"token,omitempty"`
	ApplicationID *string `json:"applicationId,omitempty"`
	IsEnabled     *bool   `json:"isEnabled,omitempty"`
	Revision      int64   `json:"revision,omitempty"`
}

func (r CreateAgentRequest) toCommand() appagents.CreateAgentCommand {
	return appagents.CreateAgentCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		DisplayName:    r.DisplayName,
		Type:           r.Type,
		Host:           r.Host,
		Port:           r.Port,
		Url:            r.Url,
		Token:          r.Token,
		ApplicationID:  r.ApplicationID,
		IsEnabled:      r.IsEnabled,
	}
}

func (r PatchAgentRequest) toCommand(id string) appagents.PatchAgentCommand {
	return appagents.PatchAgentCommand{
		ID:            id,
		DisplayName:   r.DisplayName,
		Host:          r.Host,
		Port:          r.Port,
		Url:           r.Url,
		Token:         r.Token,
		ApplicationID: r.ApplicationID,
		IsEnabled:     r.IsEnabled,
		Revision:      r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListAgentsRequest) (ListAgentsResponse, error) {
	items, total, err := h.Service.ListAgents(ctx, appagents.ListAgentsQuery(request))
	if err != nil {
		return ListAgentsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*AgentResponse, error) {
	response, err := h.Service.GetAgent(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	typed := AgentResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateAgentRequest) error {
	return h.Service.CreateAgent(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchAgentRequest) error {
	return h.Service.PatchAgent(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteAgent(ctx, organizationID, id)
}

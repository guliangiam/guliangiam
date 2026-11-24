package webhooks

import (
	"context"

	appwebhooks "github.com/guliangiam/guliangiam/internal/app/webhooks"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appwebhooks.Service
}

func NewHandler(s appwebhooks.Service) Handler {
	return Handler{Service: s}
}

type ListWebhooksResponse = presenter.ListResponse[appwebhooks.WebhookListItem]
type WebhookResponse = appwebhooks.WebhookDetail

type ListWebhooksRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
	Method         string
}

type GetWebhookRequest struct {
	ID string
}

type CreateWebhookRequest struct {
	OrganizationID        string   `json:"organizationId"`
	Name                  string   `json:"name"`
	DisplayName           string   `json:"displayName"`
	URL                   string   `json:"url"`
	Method                string   `json:"method"`
	Headers               string   `json:"headers,omitempty"`
	Events                []string `json:"events,omitempty"`
	IsEnabled             *bool    `json:"isEnabled,omitempty"`
	RetryCount            int      `json:"retryCount,omitempty"`
	TimeoutMs             int      `json:"timeoutMs,omitempty"`
	Organization          string   `json:"organization,omitempty"`
	ContentType           string   `json:"contentType,omitempty"`
	CustomBody            string   `json:"customBody,omitempty"`
	CustomHeaders         string   `json:"customHeaders,omitempty"`
	CertID                string   `json:"certId,omitempty"`
	IsError               bool     `json:"isError,omitempty"`
	ErrorText             string   `json:"errorText,omitempty"`
	TokenFields           []string `json:"tokenFields,omitempty"`
	ObjectFields          []string `json:"objectFields,omitempty"`
	IsUserExtended        bool     `json:"isUserExtended,omitempty"`
	SingleOrgOnly         bool     `json:"singleOrgOnly,omitempty"`
	MaxRetries            int      `json:"maxRetries,omitempty"`
	RetryInterval         int      `json:"retryInterval,omitempty"`
	UseExponentialBackoff bool     `json:"useExponentialBackoff,omitempty"`
}

type PatchWebhookRequest struct {
	DisplayName           *string  `json:"displayName,omitempty"`
	URL                   *string  `json:"url,omitempty"`
	Method                *string  `json:"method,omitempty"`
	Headers               *string  `json:"headers,omitempty"`
	Secret                *string  `json:"secret,omitempty"`
	Events                []string `json:"events,omitempty"`
	IsEnabled             *bool    `json:"isEnabled,omitempty"`
	RetryCount            *int     `json:"retryCount,omitempty"`
	TimeoutMs             *int     `json:"timeoutMs,omitempty"`
	Organization          *string  `json:"organization,omitempty"`
	ContentType           *string  `json:"contentType,omitempty"`
	CustomBody            *string  `json:"customBody,omitempty"`
	CustomHeaders         *string  `json:"customHeaders,omitempty"`
	CertID                *string  `json:"certId,omitempty"`
	IsError               *bool    `json:"isError,omitempty"`
	ErrorText             *string  `json:"errorText,omitempty"`
	TokenFields           []string `json:"tokenFields,omitempty"`
	ObjectFields          []string `json:"objectFields,omitempty"`
	IsUserExtended        *bool    `json:"isUserExtended,omitempty"`
	SingleOrgOnly         *bool    `json:"singleOrgOnly,omitempty"`
	MaxRetries            *int     `json:"maxRetries,omitempty"`
	RetryInterval         *int     `json:"retryInterval,omitempty"`
	UseExponentialBackoff *bool    `json:"useExponentialBackoff,omitempty"`
	Revision              int64    `json:"revision,omitempty"`
}

func (r CreateWebhookRequest) toCommand() appwebhooks.CreateWebhookCommand {
	return appwebhooks.CreateWebhookCommand{
		OrganizationID:        r.OrganizationID,
		Name:                  r.Name,
		DisplayName:           r.DisplayName,
		URL:                   r.URL,
		Method:                r.Method,
		Headers:               r.Headers,
		Events:                r.Events,
		IsEnabled:             r.IsEnabled,
		RetryCount:            r.RetryCount,
		TimeoutMs:             r.TimeoutMs,
		Organization:          r.Organization,
		ContentType:           r.ContentType,
		CustomBody:            r.CustomBody,
		CustomHeaders:         r.CustomHeaders,
		CertID:                r.CertID,
		IsError:               r.IsError,
		ErrorText:             r.ErrorText,
		TokenFields:           r.TokenFields,
		ObjectFields:          r.ObjectFields,
		IsUserExtended:        r.IsUserExtended,
		SingleOrgOnly:         r.SingleOrgOnly,
		MaxRetries:            r.MaxRetries,
		RetryInterval:         r.RetryInterval,
		UseExponentialBackoff: r.UseExponentialBackoff,
	}
}

func (r PatchWebhookRequest) toCommand(id string) appwebhooks.PatchWebhookCommand {
	command := appwebhooks.PatchWebhookCommand{
		ID:                    id,
		DisplayName:           r.DisplayName,
		URL:                   r.URL,
		Method:                r.Method,
		Headers:               r.Headers,
		Secret:                r.Secret,
		IsEnabled:             r.IsEnabled,
		RetryCount:            r.RetryCount,
		TimeoutMs:             r.TimeoutMs,
		Organization:          r.Organization,
		ContentType:           r.ContentType,
		CustomBody:            r.CustomBody,
		CustomHeaders:         r.CustomHeaders,
		CertID:                r.CertID,
		IsError:               r.IsError,
		ErrorText:             r.ErrorText,
		IsUserExtended:        r.IsUserExtended,
		SingleOrgOnly:         r.SingleOrgOnly,
		MaxRetries:            r.MaxRetries,
		RetryInterval:         r.RetryInterval,
		UseExponentialBackoff: r.UseExponentialBackoff,
		Revision:              r.Revision,
	}
	if r.Events != nil {
		command.Events = r.Events
	}
	if r.TokenFields != nil {
		command.TokenFields = r.TokenFields
	}
	if r.ObjectFields != nil {
		command.ObjectFields = r.ObjectFields
	}
	return command
}

func (h Handler) List(ctx context.Context, request ListWebhooksRequest) (ListWebhooksResponse, error) {
	items, total, err := h.Service.ListWebhooks(ctx, appwebhooks.ListWebhooksQuery(request))
	if err != nil {
		return ListWebhooksResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetWebhookRequest) (*WebhookResponse, error) {
	response, err := h.Service.GetWebhook(ctx, request.ID, organizationID)
	if err != nil {
		return nil, err
	}

	typed := WebhookResponse(*response)
	return &typed, nil
}

// Create returns the generated signing secret exactly once (secretPlain); the
// HTTP layer must put it in the 201 response and never expose it on reads.
func (h Handler) Create(ctx context.Context, request CreateWebhookRequest) (string, error) {
	return h.Service.CreateWebhook(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchWebhookRequest) error {
	return h.Service.PatchWebhook(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteWebhook(ctx, id, organizationID)
}

// Webhook event handler methods

type ListWebhookEventsResponse = presenter.ListResponse[appwebhooks.WebhookEventListItem]

func (h Handler) ListEvents(ctx context.Context, webhookID, organizationID string, page, pageSize int, eventType string, status *string) (ListWebhookEventsResponse, error) {
	items, total, err := h.Service.ListWebhookEvents(ctx, webhookID, organizationID, page, pageSize, eventType, status)
	if err != nil {
		return ListWebhookEventsResponse{}, err
	}
	return presenter.NewListResponse(items, page, pageSize, total), nil
}

func (h Handler) GetEvent(ctx context.Context, id, organizationID string) (*appwebhooks.WebhookEventDetail, error) {
	return h.Service.GetWebhookEvent(ctx, id, organizationID)
}

func (h Handler) ReplayEvent(ctx context.Context, id, organizationID string) error {
	return h.Service.ReplayWebhookEvent(ctx, id, organizationID)
}

func (h Handler) DeleteEvent(ctx context.Context, id, organizationID string) error {
	return h.Service.DeleteWebhookEvent(ctx, id, organizationID)
}

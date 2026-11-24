package subscriptions

import (
	"context"
	"time"

	appsubscriptions "github.com/guliangiam/guliangiam/internal/app/subscriptions"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appsubscriptions.Service
}

func NewHandler(s appsubscriptions.Service) Handler {
	return Handler{Service: s}
}

type ListSubscriptionsResponse = presenter.ListResponse[appsubscriptions.SubscriptionListItem]
type SubscriptionResponse = appsubscriptions.SubscriptionDetail

type ListSubscriptionsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	User           string
	State          *string
}

type GetSubscriptionRequest struct {
	ID string
}

type CreateSubscriptionRequest struct {
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	User           string     `json:"user,omitempty"`
	Plan           string     `json:"plan,omitempty"`
	Pricing        string     `json:"pricing,omitempty"`
	State          string     `json:"state,omitempty"`
	StartAt        *time.Time `json:"startAt,omitempty"`
	EndAt          *time.Time `json:"endAt,omitempty"`
	ExpireAt       *time.Time `json:"expireAt,omitempty"`
}

type PatchSubscriptionRequest struct {
	DisplayName *string    `json:"displayName,omitempty"`
	State       *string    `json:"state,omitempty"`
	StartAt     *time.Time `json:"startAt,omitempty"`
	EndAt       *time.Time `json:"endAt,omitempty"`
	ExpireAt    *time.Time `json:"expireAt,omitempty"`
	Revision    int64      `json:"revision,omitempty"`
}

func (r CreateSubscriptionRequest) toCommand() appsubscriptions.CreateSubscriptionCommand {
	return appsubscriptions.CreateSubscriptionCommand{
		OrganizationID: r.OrganizationID, Name: r.Name, DisplayName: r.DisplayName,
		User: r.User, Plan: r.Plan, Pricing: r.Pricing, State: r.State,
		StartAt: r.StartAt, EndAt: r.EndAt, ExpireAt: r.ExpireAt,
	}
}

func (r PatchSubscriptionRequest) toCommand(id string) appsubscriptions.PatchSubscriptionCommand {
	return appsubscriptions.PatchSubscriptionCommand{
		ID: id, DisplayName: r.DisplayName, State: r.State,
		StartAt: r.StartAt, EndAt: r.EndAt, ExpireAt: r.ExpireAt, Revision: r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListSubscriptionsRequest) (ListSubscriptionsResponse, error) {
	items, total, err := h.Service.ListSubscriptions(ctx, appsubscriptions.ListSubscriptionsQuery(request))
	if err != nil {
		return ListSubscriptionsResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetSubscriptionRequest) (*SubscriptionResponse, error) {
	response, err := h.Service.GetSubscription(ctx, organizationID, request.ID)
	if err != nil {
		return nil, err
	}
	typed := SubscriptionResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateSubscriptionRequest) error {
	return h.Service.CreateSubscription(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchSubscriptionRequest) error {
	return h.Service.PatchSubscription(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteSubscription(ctx, organizationID, id)
}

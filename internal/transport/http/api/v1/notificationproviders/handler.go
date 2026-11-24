package notificationproviders

import (
	"context"

	appnotificationproviders "github.com/guliangiam/guliangiam/internal/app/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appnotificationproviders.Service
}

func (h Handler) List(ctx context.Context, request ListNotificationProvidersRequest) (ListNotificationProvidersResponse, error) {
	items, total, err := h.Service.ListNotificationProviders(ctx, appnotificationproviders.ListNotificationProvidersQuery(request))
	if err != nil {
		return ListNotificationProvidersResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*NotificationProviderResponse, error) {
	response, err := h.Service.GetNotificationProvider(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	typed := NotificationProviderResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateNotificationProviderRequest) error {
	return h.Service.CreateNotificationProvider(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchNotificationProviderRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	return h.Service.PatchNotificationProvider(ctx, command)
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteNotificationProvider(ctx, organizationID, id)
}

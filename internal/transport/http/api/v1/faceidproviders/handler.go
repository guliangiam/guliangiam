package faceidproviders

import (
	"context"

	appfaceidproviders "github.com/guliangiam/guliangiam/internal/app/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appfaceidproviders.Service
}

func (h Handler) List(ctx context.Context, request ListFaceIdProvidersRequest) (ListFaceIdProvidersResponse, error) {
	items, total, err := h.Service.ListFaceIdProviders(ctx, appfaceidproviders.ListFaceIdProvidersQuery{
		Page:           request.Page,
		PageSize:       request.PageSize,
		OrganizationID: request.OrganizationID,
		IsEnabled:      request.IsEnabled,
	})
	if err != nil {
		return ListFaceIdProvidersResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*FaceIdProviderResponse, error) {
	response, err := h.Service.GetFaceIdProvider(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	typed := FaceIdProviderResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateFaceIdProviderRequest) error {
	return h.Service.CreateFaceIdProvider(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchFaceIdProviderRequest) error {
	return h.Service.PatchFaceIdProvider(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteFaceIdProvider(ctx, organizationID, id)
}

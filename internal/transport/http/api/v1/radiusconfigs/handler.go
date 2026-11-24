package radiusconfigs

import (
	"context"

	appradiusconfigs "github.com/guliangiam/guliangiam/internal/app/radiusconfigs"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appradiusconfigs.Service
}

func (h Handler) List(ctx context.Context, request ListRadiusConfigsRequest) (ListRadiusConfigsResponse, error) {
	items, total, err := h.Service.ListRadiusConfigs(ctx, appradiusconfigs.ListRadiusConfigsQuery{
		Page:           request.Page,
		PageSize:       request.PageSize,
		OrganizationID: request.OrganizationID,
		IsEnabled:      request.IsEnabled,
		AuthProtocol:   request.AuthProtocol,
	})
	if err != nil {
		return ListRadiusConfigsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*RadiusConfigResponse, error) {
	response, err := h.Service.GetRadiusConfig(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	typed := RadiusConfigResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateRadiusConfigRequest) error {
	return h.Service.CreateRadiusConfig(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchRadiusConfigRequest) error {
	return h.Service.PatchRadiusConfig(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteRadiusConfig(ctx, organizationID, id)
}

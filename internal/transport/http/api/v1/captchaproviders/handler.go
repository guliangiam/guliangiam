package captchaproviders

import (
	"context"

	appcaptchaproviders "github.com/guliangiam/guliangiam/internal/app/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appcaptchaproviders.Service
}

func (h Handler) List(ctx context.Context, request ListCaptchaProvidersRequest) (ListCaptchaProvidersResponse, error) {
	items, total, err := h.Service.ListCaptchaProviders(ctx, appcaptchaproviders.ListCaptchaProvidersQuery(request))
	if err != nil {
		return ListCaptchaProvidersResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*CaptchaProviderResponse, error) {
	response, err := h.Service.GetCaptchaProvider(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	typed := CaptchaProviderResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateCaptchaProviderRequest) error {
	return h.Service.CreateCaptchaProvider(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchCaptchaProviderRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	return h.Service.PatchCaptchaProvider(ctx, command)
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteCaptchaProvider(ctx, organizationID, id)
}

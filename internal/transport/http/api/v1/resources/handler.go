package resources

import (
	"context"

	appresources "github.com/guliangiam/guliangiam/internal/app/resources"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appresources.Service
}

func NewHandler(service appresources.Service) Handler {
	return Handler{Service: service}
}

func (h Handler) List(ctx context.Context, request ListResourcesRequest) (ListResourcesResponse, error) {
	items, total, err := h.Service.ListResources(ctx, appresources.ListResourcesQuery(request))
	if err != nil {
		return ListResourcesResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*ResourceResponse, error) {
	response, err := h.Service.GetResource(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	typed := ResourceResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateResourceRequest) error {
	return h.Service.CreateResource(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchResourceRequest) error {
	return h.Service.PatchResource(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteResource(ctx, organizationID, id)
}

// Upload handles a multipart file upload, stores the file, and returns the URL.
func (h Handler) Upload(ctx context.Context, command UploadResourceCommand) (string, error) {
	return h.Service.UploadResource(ctx, command.toServiceCommand())
}

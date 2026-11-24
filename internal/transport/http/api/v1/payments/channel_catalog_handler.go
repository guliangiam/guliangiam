package payments

import (
	"context"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// ChannelCatalogHandler exposes the adapter capability matrix as a read-only
// catalog. It depends on the app service, never on infra adapters directly.
type ChannelCatalogHandler struct {
	Svc apppayments.ChannelCatalogService
}

func NewChannelCatalogHandler(svc apppayments.ChannelCatalogService) ChannelCatalogHandler {
	return ChannelCatalogHandler{Svc: svc}
}

type ListPayChannelsResponse = presenter.ListResponse[apppayments.ChannelMetadata]

func (h ChannelCatalogHandler) List(ctx context.Context, page, pageSize int) ListPayChannelsResponse {
	items := h.Svc.List(ctx)
	return paginateList(items, page, pageSize)
}

package reconciliation

import (
	"context"

	appreconciliation "github.com/guliangiam/guliangiam/internal/app/reconciliation"
)

// ListRequest is the query DTO for reconciliation report listing.
type ListRequest struct {
	Page     int
	PageSize int
}

// Handler exposes reconciliation reports to the management surface.
type Handler struct {
	Service appreconciliation.Service
}

// NewHandler constructs the handler.
func NewHandler(service appreconciliation.Service) *Handler {
	return &Handler{Service: service}
}

// List returns one page of reports for the caller's organization.
func (h *Handler) List(ctx context.Context, request ListRequest) (*appreconciliation.ListResponse, error) {
	if h == nil || h.Service == nil {
		return nil, unavailableService()
	}
	return h.Service.List(ctx, appreconciliation.ListQuery{
		OrganizationID: organizationIDFromContext(ctx),
		Page:           request.Page,
		PageSize:       request.PageSize,
	})
}

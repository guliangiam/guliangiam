package orders

import (
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// requireOrganizationID keeps non-HTTP callers from degrading to a cross-tenant
// lookup when no trusted tenant is present.
func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

// ensureOrderTenantScope fails closed if a repository ever returns an unscoped row.
func ensureOrderTenantScope(order *domain.Order, organizationID string) error {
	if order == nil || order.OrganizationID == "" || order.OrganizationID != organizationID {
		return apperrors.NewNotFound("order not found")
	}
	return nil
}

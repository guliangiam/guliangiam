package transactions

import (
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
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

// ensureTransactionTenantScope fails closed if a repository ever returns an unscoped row.
func ensureTransactionTenantScope(transaction *domain.Transaction, organizationID string) error {
	if transaction == nil || transaction.OrganizationID == "" || transaction.OrganizationID != organizationID {
		return apperrors.NewNotFound("transaction not found")
	}
	return nil
}

package subscriptions

import (
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
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

// ensureSubscriptionTenantScope fails closed if a repository ever returns an unscoped row.
func ensureSubscriptionTenantScope(subscription *domain.Subscription, organizationID string) error {
	if subscription == nil || subscription.OrganizationID == "" || subscription.OrganizationID != organizationID {
		return apperrors.NewNotFound("subscription not found")
	}
	return nil
}

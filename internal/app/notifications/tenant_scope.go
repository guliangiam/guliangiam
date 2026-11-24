package notifications

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// requireOrganizationID fails fast when a caller reaches the app layer without
// a trusted tenant. Handlers resolve it via TrustedOrganizationID; the guard
// keeps non-HTTP callers from degrading to a cross-tenant lookup.
func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

package syncers

import (
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// requireOrganizationID fails fast when a caller reaches the app layer without
// a trusted tenant. HTTP handlers resolve this via TrustedOrganizationID; the
// guard keeps non-HTTP callers from degrading to a cross-tenant lookup.
func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

// ensureSyncerTenantScope is the application half of the syncer by-ID guard.
// The scoped repository query already filters by organization_id; this check
// keeps the ownership decision explicit and fails closed if a repository
// implementation ever returns an unscoped row.
func ensureSyncerTenantScope(syncer *domain.Syncer, organizationID string) error {
	if syncer == nil || syncer.OrganizationID == "" || syncer.OrganizationID != organizationID {
		return apperrors.NewNotFound("syncer not found")
	}
	return nil
}

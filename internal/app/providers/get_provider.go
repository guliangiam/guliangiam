package providers

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func (s Service) GetProvider(ctx context.Context, id, organizationID string) (*ProviderDetail, error) {
	return s.GetProviderByFamily(ctx, id, "", organizationID)
}

func (s Service) GetProviderByFamily(ctx context.Context, id, family, organizationID string) (*ProviderDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	unified := s.unified()
	provider, err := unified.Get(ctx, id, family, organizationID)
	if err != nil {
		return nil, err
	}
	// The repository read is scoped; this assertion only guards against a
	// future regression in the loader.
	if err := ensureProviderTenantScope(ctx, provider.Registration.OrganizationID, organizationID); err != nil {
		return nil, err
	}
	return toUnifiedDetail(unified, provider)
}

// ensureProviderTenantScope is a defence-in-depth assertion over a row that was
// already loaded through a tenant-scoped repository read. It keeps the
// not-found response shape for a cross-tenant row even if a loader regresses
// and returns a foreign entity. An empty organizationID is only accepted for
// an authenticated platform scope; routed by-ID calls require a tenant before
// reaching the repository.
func ensureProviderTenantScope(ctx context.Context, providerOrganizationID, organizationID string) error {
	if organizationID == "" {
		scope := tenantctx.TenantScopeFromContext(ctx)
		if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
			return nil
		}
		return apperrors.NewNotFound("provider not found")
	}
	if providerOrganizationID == "" || providerOrganizationID != organizationID {
		return apperrors.NewNotFound("provider not found")
	}
	return nil
}

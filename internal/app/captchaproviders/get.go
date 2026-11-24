package captchaproviders

import (
	"context"
	"strings"

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

func (s Service) GetCaptchaProvider(ctx context.Context, organizationID, id string) (*CaptchaProviderDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}

	provider, err := s.UOW.CaptchaProviders().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(provider)
	return &detail, nil
}

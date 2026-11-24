package certs

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

func (s Service) GetCert(ctx context.Context, organizationID, id string) (*CertDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}

	cert, err := s.UOW.Certs().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(cert)
	return &detail, nil
}

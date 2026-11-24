package applications

import (
	"context"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// GetApplication is the management-route read: the caller's trusted tenant is a
// required input, so a by-ID lookup cannot cross organizations. The transport
// layer passes an empty organizationID only for a super-admin/system scope that
// omitted organizationId; such a read is authorized to use the system-scoped
// lookup instead.
func (s *Service) GetApplication(ctx context.Context, organizationID, id string) (*ApplicationDetail, error) {
	var application *applications.Application
	var err error
	if organizationID == "" {
		application, err = s.UOW.Applications().GetByID(ctx, id)
	} else {
		application, err = s.UOW.Applications().GetByIDForOrganization(ctx, id, organizationID)
	}
	if err != nil {
		return nil, err
	}

	set, masked, err := s.clientSecretState(ctx, s.UOW.Applications(), application, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	detail := toDetail(application, ClientSecretState{Set: set, Masked: masked})
	keys, err := s.UOW.Applications().ListActiveClientJWKs(ctx, application.OrganizationID, application.ID)
	if err != nil {
		return nil, err
	}
	detail.ClientJWKS = projectClientJWKS(keys)
	uri, err := s.clientJWKSURI(ctx, application.OrganizationID, application.ID)
	if err != nil {
		return nil, err
	}
	detail.JWKSURI = uri
	return detail, nil
}

// clientJWKSURI reads the remote JWKS location for detail projections.
// NotFound is the normal "inline JWKS only" state; repositories without the
// capability (fakes/MySQL mirror) degrade to empty, mirroring the read-side
// precedent in auth/client_assertion.go.
func (s *Service) clientJWKSURI(ctx context.Context, organizationID, applicationID string) (string, error) {
	uriRepo, ok := s.UOW.Applications().(applications.ClientJWKSURIRepository)
	if !ok {
		return "", nil
	}
	uri, err := uriRepo.GetClientJWKSURI(ctx, organizationID, applicationID)
	if err != nil {
		if apperrors.FromError(err).Code == apperrors.CodeNotFound {
			return "", nil
		}
		return "", err
	}
	if uri == nil {
		return "", nil
	}
	return uri.JWKSURI, nil
}

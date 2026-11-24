package verifications

import (
	"context"
)

func (s Service) GetVerification(ctx context.Context, organizationID, id string) (*VerificationDTO, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// The management by-ID route is tenant-bound; recipient/code send-and-
	// verify flows keep using the system-scoped GetByID.
	v, err := s.UOW.Verifications().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	return toVerificationDTO(v), nil
}

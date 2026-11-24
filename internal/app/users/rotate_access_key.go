package users

import (
	"context"
)

func (s *Service) RotateUserAccessKey(ctx context.Context, organizationID, userID string) (UpdateUserCredentialsResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return UpdateUserCredentialsResult{}, err
	}
	return s.UpdateUserCredentials(ctx, UpdateUserCredentialsCommand{
		OrganizationID:     organizationID,
		UserID:             userID,
		RotateAccessSecret: true,
	})
}

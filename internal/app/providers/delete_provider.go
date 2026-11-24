package providers

import "context"

func (s Service) DeleteProvider(ctx context.Context, id, organizationID, actorID string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.unified().Delete(ctx, id, "", organizationID, actorID)
}

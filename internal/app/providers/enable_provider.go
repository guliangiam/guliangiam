package providers

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

// EnableProvider is the explicit activation step that follows a successful
// connection probe. Probing itself is read-only; this method is the only path
// that moves a provider to active from the management API.
func (s Service) EnableProvider(ctx context.Context, id, organizationID, actorID string) (*Provider, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	return s.unified().ChangeStatus(ctx, ChangeProviderStatusCommand{
		ID:             id,
		Status:         string(domain.StatusActive),
		OrganizationID: organizationID,
		ActorID:        actorID,
	})
}

package gateway

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s *Service) DeleteServer(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first inside the transaction so a foreign id is rejected before
		// the delete and the audit row carries the entity's real organization.
		server, err := tx.Gateway().GetServerByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Gateway().DeleteServer(ctx, id); err != nil {
			return err
		}
		return s.embedAuditEvent(
			tx,
			ctx,
			server.OrganizationID,
			"gateway.server.deleted",
			"server",
			id,
			"server",
			"gateway.server.deleted",
			map[string]any{keyServerID: id, keyOrgID: server.OrganizationID},
		)
	})
}

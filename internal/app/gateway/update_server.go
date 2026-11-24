package gateway

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s *Service) UpdateServer(ctx context.Context, organizationID string, command UpdateServerCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validateUpdateServerCommand(command); err != nil {
		return err
	}

	// command.ID comes from the URL path. Ownership is decided inside the
	// transaction before any mutation: a foreign or missing id returns NotFound
	// and leaves no write and no audit record behind.
	now := s.Clock.Now()
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		server, err := tx.Gateway().GetServerByIDForOrganization(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}
		server.Name = command.Name
		server.Endpoint = command.Endpoint
		server.Protocol = command.Protocol
		server.Status = command.Status
		server.Metadata = command.Metadata
		server.UpdatedAt = now
		server.Revision = command.Revision + 1

		if err := tx.Gateway().UpdateServer(ctx, server); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			server.OrganizationID,
			"gateway.server.updated",
			"server",
			server.ID,
			"server",
			"gateway.server.updated",
			map[string]any{

				keyServerID: server.ID,

				keyName: server.Name,

				keyOrgID: server.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}

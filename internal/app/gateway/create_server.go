package gateway

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
)

func (s *Service) CreateServer(ctx context.Context, command CreateServerCommand) (*ServerDTO, error) {
	if err := validateCreateServerCommand(command); err != nil {
		return nil, err
	}

	// Transport converges command.OrganizationID through TrustedOrganizationID
	// before this use case runs, so the insert can only target the tenant the
	// caller's scope authorizes.
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	server := &domain.Server{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		Endpoint:       command.Endpoint,
		Protocol:       command.Protocol,
		Status:         command.Status,
		Metadata:       command.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	return toServerDTO(server), s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

		if err := tx.Gateway().CreateServer(ctx, server); err != nil {
			return err

		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			server.OrganizationID,
			"gateway.server.created",
			"server",
			server.ID,
			"server",
			"gateway.server.created",
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

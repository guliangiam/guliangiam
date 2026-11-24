package providers

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

type UpdateProviderConfigCommand struct {
	ProviderID string
	// OrganizationID is the trusted tenant scope resolved by transport.
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID    string
	ConfigType string
	Config     map[string]any
}

func (s Service) UpdateProviderConfig(ctx context.Context, command UpdateProviderConfigCommand) error {
	if err := validateUpdateProviderConfigCommand(command); err != nil {
		return err
	}
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := tx.Providers().GetByIDForOrganization(ctx, command.ProviderID, command.OrganizationID)
		if err != nil {
			return err
		}
		// Ownership is already bound by the scoped read; the assertion keeps the
		// not-found shape before the config write and the audit/outbox embed.
		if err := ensureProviderTenantScope(ctx, provider.OrganizationID, command.OrganizationID); err != nil {
			return err
		}

		if err := tx.Providers().UpdateConfig(ctx, &domain.ProviderConfig{
			ProviderID: command.ProviderID,
			ConfigType: command.ConfigType,
			Config:     command.Config,
			UpdatedAt:  s.Clock.Now(),
		}); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			provider.OrganizationID, command.ActorID,
			"provider_config.updated", "provider_config", command.ProviderID,
			"provider", "provider_config.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

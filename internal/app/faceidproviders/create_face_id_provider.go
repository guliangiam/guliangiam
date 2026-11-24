package faceidproviders

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) CreateFaceIdProvider(ctx context.Context, command CreateFaceIdProviderCommand) error {
	if err := validateCreateFaceIdProviderCommand(command); err != nil {
		return err
	}
	provider, err := s.NewFaceIdProvider(command)
	if err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.FaceIdProviders().Create(ctx, provider); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			provider.OrganizationID, "",
			"faceidproviders.created", "faceidproviders", provider.ID,
			"faceidproviders", "faceidproviders.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

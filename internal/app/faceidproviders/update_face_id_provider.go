package faceidproviders

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchFaceIdProviderCommand struct {
	ID              string
	Name            *string
	ProviderType    *string
	AccessKeyID     *string
	AccessKeySecret *string
	Endpoint        *string
	Config          map[string]any
	IsEnabled       *bool
	Revision        int64
}

func (s Service) PatchFaceIdProvider(ctx context.Context, organizationID string, command PatchFaceIdProviderCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validatePatchFaceIdProviderCommand(command); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		provider, err := tx.FaceIdProviders().GetByID(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && provider.Revision != command.Revision {
			return apperrors.NewConflict("face id provider revision mismatch")
		}

		if command.Name != nil {
			provider.Name = *command.Name
		}
		if command.ProviderType != nil {
			provider.ProviderType = *command.ProviderType
		}
		if command.AccessKeyID != nil {
			provider.AccessKeyID = *command.AccessKeyID
		}
		if command.AccessKeySecret != nil {
			provider.AccessKeySecret = *command.AccessKeySecret
		}
		if command.Endpoint != nil {
			provider.Endpoint = *command.Endpoint
		}
		if command.Config != nil {
			provider.Config = make(map[string]any)
			for k, v := range command.Config {
				provider.Config[k] = v
			}
		}
		if command.IsEnabled != nil {
			provider.IsEnabled = *command.IsEnabled
		}

		provider.UpdatedAt = s.Clock.Now()
		provider.Revision++

		if err := tx.FaceIdProviders().Update(ctx, provider); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			provider.OrganizationID, "",
			"faceidproviders.updated", "faceidproviders", provider.ID,
			"faceidproviders", "faceidproviders.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

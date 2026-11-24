package applications

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ReplaceApplicationProviderBindingCommand struct {
	ProviderID string
}

func (s *Service) ReplaceProviderBindings(ctx context.Context, organizationID, applicationID, actorID string, commands []ReplaceApplicationProviderBindingCommand) error {
	_, err := s.replaceProviderBindings(ctx, organizationID, applicationID, actorID, commands)
	return err
}

// ReplaceProviderBindingsWithRevision 在绑定成功后返回落库后的 revision。
func (s *Service) ReplaceProviderBindingsWithRevision(ctx context.Context, organizationID, applicationID, actorID string, commands []ReplaceApplicationProviderBindingCommand) (int64, error) {
	return s.replaceProviderBindings(ctx, organizationID, applicationID, actorID, commands)
}

func (s *Service) replaceProviderBindings(ctx context.Context, organizationID, applicationID, actorID string, commands []ReplaceApplicationProviderBindingCommand) (int64, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return 0, err
	}
	if err := validateReplaceProviderBindingCommands(commands); err != nil {
		return 0, err
	}

	var savedRevision int64
	// 事务内先按租户读取；Update 以实体 OrganizationID 为谓词，审计使用实体组织。
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		application, err := tx.Applications().GetByIDForOrganization(ctx, applicationID, organizationID)
		if err != nil {
			return err
		}

		bindings := make([]domain.ApplicationProviderBinding, 0, len(commands))
		for index, command := range commands {
			// The binding row carries a foreign key to providers, so an unknown or
			// foreign provider would otherwise surface as a driver constraint error
			// (500). Resolve it here, inside the tenant-scoped transaction, and
			// answer with the same 422 family as the duplicate-providerId check.
			if _, err := tx.Providers().GetByIDForOrganization(ctx, command.ProviderID, organizationID); err != nil {
				if appErr, ok := err.(*apperrors.Error); ok && appErr.Code == apperrors.CodeNotFound {
					return apperrors.NewWithMeta(
						apperrors.CodeUnprocessableEntity,
						"provider binding providerId does not exist",
						map[string]any{metaKeyIndex: index, "providerId": command.ProviderID},
					)
				}
				return err
			}
			bindings = append(bindings, domain.ApplicationProviderBinding{
				ApplicationID: applicationID,
				ProviderID:    command.ProviderID,
			})
		}

		application.ProviderBindings = bindings
		application.UpdatedAt = s.Clock.Now()
		application.Revision++

		if err := tx.Applications().Update(ctx, application); err != nil {
			return err
		}
		savedRevision = application.Revision

		if err := tx.Applications().ReplaceProviderBindings(ctx, applicationID, bindings); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			application.OrganizationID,
			actorID,
			"application.provider_bindings.replaced",
			application.ID,
			"application.provider_bindings.replaced",
			map[string]any{},
		); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			application.OrganizationID,
			actorID,
			"application.provider_bindings.replaced",
			applicationID,
			"application.provider_bindings.replaced",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return savedRevision, nil
}

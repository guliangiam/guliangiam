package applications

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

type DeleteApplicationCommand struct {
	ID             string
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID string
}

func (s *Service) DeleteApplication(ctx context.Context, command DeleteApplicationCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// 先按租户读取实体：跨租户 id 统一 NotFound，审计使用的组织来自实体本身。
		application, err := tx.Applications().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}

		if err := tx.Applications().Delete(ctx, application.ID, application.OrganizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			application.OrganizationID,
			command.ActorID,
			"application.deleted",
			application.ID,
			"application.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

package rules

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) DeleteRule(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		rule, err := tx.Rules().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Rules().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			rule.OrganizationID, "",
			"rules.delete", "rules", id,
			"rules", "rules.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	return err
}

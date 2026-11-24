package rules

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) CreateRule(ctx context.Context, command CreateRuleCommand) error {
	if err := validateCreateRuleCommand(command); err != nil {
		return err
	}

	rule, err := s.NewRule(command)
	if err != nil {
		return err
	}

	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Rules().Create(ctx, rule); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			rule.OrganizationID, "",
			"rules.create", "rules", rule.ID,
			"rules", "rules.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

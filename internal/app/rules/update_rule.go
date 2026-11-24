package rules

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchRuleCommand struct {
	ID         string
	Name       *string
	Scope      *string
	RuleType   *string
	Conditions []map[string]any
	Action     *string
	Priority   *int
	IsEnabled  *bool
	Revision   int64
}

func (s Service) PatchRule(ctx context.Context, organizationID string, command PatchRuleCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validatePatchRuleCommand(command); err != nil {
		return err
	}

	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		rule, err := tx.Rules().GetByID(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && rule.Revision != command.Revision {
			return apperrors.NewConflict("rule revision mismatch")
		}

		if command.Name != nil {
			rule.Name = *command.Name
		}
		if command.Scope != nil {
			rule.Scope = *command.Scope
		}
		if command.RuleType != nil {
			rule.RuleType = *command.RuleType
		}
		if command.Conditions != nil {
			rule.Conditions = append([]map[string]any(nil), command.Conditions...)
		}
		if command.Action != nil {
			rule.Action = *command.Action
		}
		if command.Priority != nil {
			rule.Priority = *command.Priority
		}
		if command.IsEnabled != nil {
			rule.IsEnabled = *command.IsEnabled
		}

		rule.UpdatedAt = s.Clock.Now()
		rule.Revision++

		// BE-D-22：按 patch 后的生效类型+条件整体校验（改 ruleType 不带
		// conditions 时也要用新类型复核既有条件）。
		if err := validateRuleConditions(rule.RuleType, rule.Conditions); err != nil {
			return err
		}

		if err := tx.Rules().Update(ctx, rule); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			rule.OrganizationID, "",
			"rules.update", "rules", rule.ID,
			"rules", "rules.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	return err
}

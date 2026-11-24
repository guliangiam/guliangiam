package rules

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// pgerrorConstraints is the whitelist of constraint identifiers this repository
// may surface in error metadata; anything else stays in the server log.
var pgerrorConstraints = []string{
	"uq_rules_org_name",
	"fk_rules_organization_id_organizations",
}

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Rule, error) {
	var row ruleRow
	err := r.db.QueryRow(ctx, queryGetRuleByID, id, organizationID).Scan(&row.ID, &row.OrganizationID, &row.Name, &row.Scope, &row.RuleType, &row.Conditions, &row.Action, &row.Priority, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("rule not found")
		}
		return nil, err
	}

	rule, err := toRule(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode rule")
	}
	return rule, nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Rule, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListRules, query.OrganizationID, query.Scope, query.RuleType, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Rule, 0)
	for rows.Next() {
		var row ruleRow
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.Name, &row.Scope, &row.RuleType, &row.Conditions, &row.Action, &row.Priority, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision); err != nil {
			return nil, 0, err
		}
		rule, err := toRule(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode rule")
		}
		items = append(items, rule)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountRules, query.OrganizationID, query.Scope, query.RuleType, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, rule *domain.Rule) error {
	conditions, err := shared.MarshalJSON(rule.Conditions)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryInsertRule, rule.ID, rule.OrganizationID, rule.Name, rule.Scope, rule.RuleType, conditions, rule.Action, rule.Priority, rule.IsEnabled, rule.CreatedAt, rule.UpdatedAt, rule.Revision)
	return pgerror.Map(err, pgerrorConstraints...)
}

func (r Repository) Update(ctx context.Context, rule *domain.Rule) error {
	conditions, err := shared.MarshalJSON(rule.Conditions)
	if err != nil {
		return err
	}

	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateRule, rule.Name, rule.Scope, rule.RuleType, conditions, rule.Action, rule.Priority, rule.IsEnabled, rule.UpdatedAt, rule.Revision-1, rule.ID, rule.OrganizationID).Scan(&newRevision)
	if err != nil {
		// The app layer read the entity scoped first: ErrNoRows is only a
		// same-tenant revision conflict.
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("rule revision conflict")
		}
		return pgerror.Map(err, pgerrorConstraints...)
	}
	rule.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteRule, id, organizationID)
	if err != nil {
		return pgerror.Map(err, pgerrorConstraints...)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("rule not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

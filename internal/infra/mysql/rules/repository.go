package rules

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 rules 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 Rule（organization_id 谓词防止跨租户读取）。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Rule, error) {
	row := r.db.QueryRowContext(ctx, queryGetRuleByID, id, organizationID)
	rule, err := scanRule(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("rule not found")
		}
		return nil, err
	}
	return rule, nil
}

// List 分页列出 Rule。
// countArgs: org_id×2 + scope×2 + rule_type×2 + is_enabled×2 = 8
// listArgs: 同上 + limit + offset = 10
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Rule, int64, error) {
	var enabledArg interface{}
	if q.IsEnabled != nil {
		enabledArg = *q.IsEnabled
	}

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Scope, q.Scope,
		q.RuleType, q.RuleType,
		enabledArg, enabledArg,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountRules, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListRules, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Rule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, rule)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 Rule。
func (r Repository) Create(ctx context.Context, rule *domain.Rule) error {
	conditions, err := json.Marshal(rule.Conditions)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertRule,
		rule.ID, rule.OrganizationID, rule.Name, rule.Scope, rule.RuleType,
		conditions, rule.Action, rule.Priority, rule.IsEnabled,
		rule.CreatedAt, rule.UpdatedAt, rule.Revision,
	)
	return err
}

// Update 乐观锁更新 Rule。先 UPDATE（revision 原子 +1），再 SELECT 新 revision。
func (r Repository) Update(ctx context.Context, rule *domain.Rule) error {
	conditions, err := json.Marshal(rule.Conditions)
	if err != nil {
		return err
	}

	exec, err := r.db.ExecContext(ctx, queryUpdateRule,
		rule.Name, rule.Scope, rule.RuleType, conditions, rule.Action,
		rule.Priority, rule.IsEnabled, rule.UpdatedAt,
		rule.ID, rule.Revision, rule.OrganizationID,
	)
	if err != nil {
		return err
	}
	affected, err := exec.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// 实体已在 app 层按租户读取：0 affected 只代表 revision 冲突。
		return apperrors.NewConflict("rule revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRuleRevision, rule.ID, rule.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("rule not found")
		}
		return err
	}
	rule.Revision = newRevision
	return nil
}

// Delete 删除 Rule（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteRule, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("rule not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}

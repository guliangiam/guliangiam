package plans

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/plans"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 plans 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id + organization_id 查 Plan（跨租户 id 与不存在同为 NotFound）。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Plan, error) {
	row := r.db.QueryRowContext(ctx, queryGetPlanByID, id, organizationID)
	p, err := scanPlan(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("plan not found")
		}
		return nil, err
	}
	return p, nil
}

// List 分页列出 Plan。
// countArgs: org_id×2 + keyword×3 + state×2 + period×2 + is_exclusive×2 = 11
// listArgs: 同上 + limit + offset = 13
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Plan, int64, error) {
	var exclusiveArg interface{}
	if q.IsExclusive != nil {
		exclusiveArg = *q.IsExclusive
	} // 否则传 nil

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.State, q.State,
		q.Period, q.Period,
		exclusiveArg, exclusiveArg,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountPlans, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListPlans, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Plan, 0)
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 Plan。
func (r Repository) Create(ctx context.Context, p *domain.Plan) error {
	providers, _ := json.Marshal(p.PaymentProviders)
	_, err := r.db.ExecContext(ctx, queryInsertPlan,
		p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Description,
		p.Price, p.Currency, p.Period, p.Product, providers,
		p.State, p.IsExclusive, p.Role, p.CreatedAt, p.UpdatedAt, p.Revision,
	)
	return err
}

// Update 更新 Plan（organization_id 取自实体；0 行受影响 → NotFound）。
func (r Repository) Update(ctx context.Context, p *domain.Plan) error {
	providers, _ := json.Marshal(p.PaymentProviders)
	res, err := r.db.ExecContext(ctx, queryUpdatePlan,
		p.DisplayName, p.Description, p.Price, p.Currency, p.Period, p.Product, providers,
		p.State, p.IsExclusive, p.Role, p.UpdatedAt, p.Revision, p.ID, p.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// 实体已在 app 层按租户读取：0 行只可能是同一租户内的行已消失。
		return apperrors.NewNotFound("plan not found")
	}
	return nil
}

// Delete 只在 Plan 属于该组织时删除；0 行受影响 → NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeletePlan, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("plan not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}

package pricings

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 pricings 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id + organization_id 查 Pricing（跨租户 id 与不存在同为 NotFound）。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Pricing, error) {
	row := r.db.QueryRowContext(ctx, queryGetPricingByID, id, organizationID)
	p, err := scanPricing(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("pricing not found")
		}
		return nil, err
	}
	return p, nil
}

// List 分页列出 Pricing。
// countArgs: org_id×2 + keyword×3 + is_enabled×2 = 7
// listArgs: 同上 + limit + offset = 9
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Pricing, int64, error) {
	var enabledArg interface{}
	if q.IsEnabled != nil {
		enabledArg = *q.IsEnabled
	} // 否则传 nil

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		enabledArg, enabledArg,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountPricings, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListPricings, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Pricing, 0)
	for rows.Next() {
		p, err := scanPricing(rows)
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

// Create 插入 Pricing。
func (r Repository) Create(ctx context.Context, p *domain.Pricing) error {
	plans, _ := json.Marshal(p.Plans)
	_, err := r.db.ExecContext(ctx, queryInsertPricing,
		p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Description,
		plans, p.IsEnabled, p.TrialDuration, p.Application,
		p.CreatedAt, p.UpdatedAt, p.Revision,
	)
	return err
}

// Update 更新 Pricing（organization_id 取自实体；0 行受影响 → NotFound）。
func (r Repository) Update(ctx context.Context, p *domain.Pricing) error {
	plans, _ := json.Marshal(p.Plans)
	res, err := r.db.ExecContext(ctx, queryUpdatePricing,
		p.DisplayName, p.Description, plans, p.IsEnabled, p.TrialDuration,
		p.Application, p.UpdatedAt, p.Revision, p.ID, p.OrganizationID,
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
		return apperrors.NewNotFound("pricing not found")
	}
	return nil
}

// Delete 只在 Pricing 属于该组织时删除；0 行受影响 → NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeletePricing, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("pricing not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}

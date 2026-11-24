package subscriptions

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 subscriptions 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// subscriptionRow 是 subscriptions 表扫描目标。
// MySQL 的 NULL 时间字段用 sql.NullTime。
type subscriptionRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	User           string
	Plan           string
	Pricing        string
	State          string
	StartAt        sql.NullTime
	EndAt          sql.NullTime
	ExpireAt       sql.NullTime
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
	Revision       int64
}

// GetByIDForOrganization 只在 Subscription 属于该组织时返回；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Subscription, error) {
	var row subscriptionRow
	err := r.db.QueryRowContext(ctx, queryGetByIDForOrganization, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
		&row.User, &row.Plan, &row.Pricing, &row.State,
		&row.StartAt, &row.EndAt, &row.ExpireAt,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("subscription not found")
		}
		return nil, err
	}
	return toSubscription(row), nil
}

// List 分页列出 Subscription。
// countArgs: org_id×2 + keyword×3 + user×2 + state×2 = 9
// listArgs: countArgs + limit + offset = 11
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Subscription, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	stateVal := ""
	if q.State != nil {
		stateVal = *q.State
	}

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.User, q.User,
		stateVal, stateVal,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountSubscriptions, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, queryListSubscriptions, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Subscription, 0)
	for rows.Next() {
		var row subscriptionRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.User, &row.Plan, &row.Pricing, &row.State,
			&row.StartAt, &row.EndAt, &row.ExpireAt,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, toSubscription(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}
	return items, total, nil
}

// Create 插入 Subscription。
func (r Repository) Create(ctx context.Context, s *domain.Subscription) error {
	_, err := r.db.ExecContext(ctx, queryInsertSubscription,
		s.ID, s.OrganizationID, s.Name, s.DisplayName,
		s.User, s.Plan, s.Pricing, s.State,
		s.StartAt, s.EndAt, s.ExpireAt,
		s.CreatedAt, s.UpdatedAt, s.Revision,
	)
	return err
}

// Update 更新 Subscription（带租户谓词）。
func (r Repository) Update(ctx context.Context, s *domain.Subscription) error {
	res, err := r.db.ExecContext(ctx, queryUpdateSubscription,
		s.DisplayName, s.State, s.StartAt, s.EndAt, s.ExpireAt, s.UpdatedAt,
		s.ID, s.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("subscription not found")
	}
	return nil
}

// Delete 只在 Subscription 属于该组织时删除；0 行受影响统一 NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteSubscription, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("subscription not found")
	}
	return nil
}

// toSubscription 把 MySQL subscriptionRow 转成 domain.Subscription。
func toSubscription(row subscriptionRow) *domain.Subscription {
	s := &domain.Subscription{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		DisplayName:    row.DisplayName,
		User:           row.User,
		Plan:           row.Plan,
		Pricing:        row.Pricing,
		State:          row.State,
		Revision:       row.Revision,
	}
	if row.StartAt.Valid {
		s.StartAt = &row.StartAt.Time
	}
	if row.EndAt.Valid {
		s.EndAt = &row.EndAt.Time
	}
	if row.ExpireAt.Valid {
		s.ExpireAt = &row.ExpireAt.Time
	}
	if row.CreatedAt.Valid {
		s.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		s.UpdatedAt = row.UpdatedAt.Time
	}
	return s
}

// 编译期接口断言
var _ domain.Repository = Repository{}

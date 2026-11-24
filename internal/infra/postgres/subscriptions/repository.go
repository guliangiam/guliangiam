package subscriptions

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Subscription, error) {
	var s domain.Subscription
	var user, plan, pricing *string
	err := r.db.QueryRow(ctx, QueryGetByIDForOrganization, id, organizationID).Scan(&s.ID, &s.OrganizationID, &s.Name, &s.DisplayName, &user, &plan, &pricing, &s.State, &s.StartAt, &s.EndAt, &s.ExpireAt, &s.CreatedAt, &s.UpdatedAt, &s.Revision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("subscription not found")
		}
		return nil, err
	}
	s.User = stringOrEmpty(user)
	s.Plan = stringOrEmpty(plan)
	s.Pricing = stringOrEmpty(pricing)
	return &s, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Subscription, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.User, q.State, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Subscription, 0)
	for rows.Next() {
		var s domain.Subscription
		var user, plan, pricing *string
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.Name, &s.DisplayName, &user, &plan, &pricing, &s.State, &s.StartAt, &s.EndAt, &s.ExpireAt, &s.CreatedAt, &s.UpdatedAt, &s.Revision); err != nil {
			return nil, 0, err
		}
		s.User = stringOrEmpty(user)
		s.Plan = stringOrEmpty(plan)
		s.Pricing = stringOrEmpty(pricing)
		items = append(items, &s)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.User, q.State).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, s *domain.Subscription) error {
	_, err := r.db.Exec(ctx, QueryInsert, s.ID, s.OrganizationID, s.Name, s.DisplayName, nullableUUID(s.User), nullableUUID(s.Plan), nullableUUID(s.Pricing), s.State, s.StartAt, s.EndAt, s.ExpireAt, s.CreatedAt, s.UpdatedAt, s.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, s *domain.Subscription) error {
	tag, err := r.db.Exec(ctx, QueryUpdate, s.DisplayName, s.State, s.StartAt, s.EndAt, s.ExpireAt, s.UpdatedAt, s.Revision, s.ID, s.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("subscription not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("subscription not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// nullableUUID 把空字符串转成 NULL：user_id / plan_id / pricing_id 都是 uuid 列，
// 直接写 "" 会触发 SQLSTATE 22P02（invalid input syntax for type uuid）。
func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

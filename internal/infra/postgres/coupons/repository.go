package coupons

import (
	"context"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// pgerrorConstraints is the whitelist of constraint identifiers this repository
// may surface in error metadata; anything else stays in the server log.
var pgerrorConstraints = []string{
	"uq_coupons_code",
	"ck_coupons_status",
	"fk_coupons_organization_id_organizations",
}

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Coupon, error) {
	return r.getByIDQuery(ctx, QueryGetByID, id)
}

// GetByIDForOrganization returns the coupon only when it belongs to the given
// tenant; a foreign or missing row is reported as not found.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Coupon, error) {
	return r.getByIDQuery(ctx, QueryGetByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Coupon, error) {
	c, err := scanCoupon(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("coupon not found")
		}
		return nil, err
	}
	return c, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Coupon, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.State, q.DiscountType, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Coupon, 0)
	for rows.Next() {
		var c domain.Coupon
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.DisplayName, &c.Code, &c.DiscountType, &c.Discount, &c.State, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &c)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.State, q.DiscountType).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, c *domain.Coupon) error {
	products, _ := json.Marshal(c.Products)
	users, _ := json.Marshal(c.Users)
	_, err := r.db.Exec(ctx, QueryInsert, c.ID, c.OrganizationID, c.Name, c.DisplayName, c.Description, c.Code, c.DiscountType, c.Discount, c.MaxDiscount, c.Scope, products, users, c.Quantity, c.UsedCount, c.MaxUsagePerUser, c.StartTime, c.ExpireTime, c.MinOrderAmount, c.Currency, c.State, c.CreatedAt, c.UpdatedAt, c.Revision)
	return pgerror.Map(err, pgerrorConstraints...)
}

func (r Repository) Update(ctx context.Context, c *domain.Coupon) error {
	products, _ := json.Marshal(c.Products)
	users, _ := json.Marshal(c.Users)
	// Callers bump c.Revision before Update; the predicate pins the row to the
	// revision they read, so a concurrent writer cannot be silently overwritten.
	tag, err := r.db.Exec(ctx, QueryUpdate, c.DisplayName, c.Description, c.Discount, c.MaxDiscount, c.Scope, products, users, c.Quantity, c.MaxUsagePerUser, c.StartTime, c.ExpireTime, c.MinOrderAmount, c.Currency, c.State, c.UpdatedAt, c.Revision, c.ID, c.OrganizationID, c.Revision-1)
	if err != nil {
		return pgerror.Map(err, pgerrorConstraints...)
	}
	if tag.RowsAffected() == 0 {
		// Distinguish a vanished row from a concurrent modification: the scoped
		// re-read finds the row only when it still belongs to this tenant.
		if _, rerr := r.GetByIDForOrganization(ctx, c.ID, c.OrganizationID); rerr != nil {
			return apperrors.NewNotFound("coupon not found")
		}
		return apperrors.NewConflict("coupon revision mismatch")
	}
	return nil
}

// GetByCode is the exact-code, tenant-scoped checkout lookup (BE-C-07).
func (r Repository) GetByCode(ctx context.Context, organizationID, code string) (*domain.Coupon, error) {
	c, err := scanCoupon(r.db.QueryRow(ctx, QueryGetByCode, organizationID, code))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("coupon not found")
		}
		return nil, err
	}
	return c, nil
}

// IncrementUsage consumes one use atomically (BE-C-06). Zero affected rows
// means the coupon is inactive, exhausted, over the per-user limit, gone, or
// was concurrently modified — all rejected as Conflict so checkout never
// oversells and never loses an increment.
func (r Repository) IncrementUsage(ctx context.Context, id, userID string, now time.Time, expectedRevision int64) (*domain.Coupon, error) {
	c, err := scanCoupon(r.db.QueryRow(ctx, QueryIncrementUsage, id, userID, now, expectedRevision))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewConflict("coupon usage increment rejected: limit reached, inactive, or revision mismatch")
		}
		return nil, pgerror.Map(err, pgerrorConstraints...)
	}
	return c, nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return pgerror.Map(err, pgerrorConstraints...)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("coupon not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
var _ domain.CodeLookup = Repository{}
var _ domain.UsageIncrementer = Repository{}

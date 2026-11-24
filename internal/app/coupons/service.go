package coupons

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.CommerceUoW
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

type ListCouponsQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	State                   *string
	DiscountType            string
}
type CreateCouponCommand struct {
	OrganizationID, Name, DisplayName, Description, Code, DiscountType, Scope, StartTime, ExpireTime, Currency, State string
	Discount, MaxDiscount, MinOrderAmount                                                                             float64
	Products, Users                                                                                                   []string
	Quantity, UsedCount, MaxUsagePerUser                                                                              int
}
type PatchCouponCommand struct {
	ID                                                                      string
	DisplayName, Description, Scope, StartTime, ExpireTime, Currency, State *string
	Discount, MaxDiscount, MinOrderAmount                                   *float64
	Products, Users                                                         []string
	Quantity, MaxUsagePerUser                                               *int
	Revision                                                                int64
}

type CouponListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Code           string    `json:"code"`
	DiscountType   string    `json:"discountType"`
	Discount       float64   `json:"discount"`
	State          string    `json:"state"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type CouponDetail struct {
	CouponListItem
	Description     string   `json:"description,omitempty"`
	MaxDiscount     float64  `json:"maxDiscount"`
	Scope           string   `json:"scope,omitempty"`
	Products        []string `json:"products,omitempty"`
	Users           []string `json:"users,omitempty"`
	Quantity        int      `json:"quantity"`
	UsedCount       int      `json:"usedCount"`
	MaxUsagePerUser int      `json:"maxUsagePerUser"`
	StartTime       string   `json:"startTime,omitempty"`
	ExpireTime      string   `json:"expireTime,omitempty"`
	MinOrderAmount  float64  `json:"minOrderAmount"`
	Currency        string   `json:"currency,omitempty"`
	Revision        int64    `json:"revision"`
}

func toListItem(c *domain.Coupon) CouponListItem {
	return CouponListItem{ID: c.ID, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Code: c.Code, DiscountType: c.DiscountType, Discount: c.Discount, State: c.State,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}
func toDetail(c *domain.Coupon) CouponDetail {
	return CouponDetail{CouponListItem: toListItem(c), Description: c.Description, MaxDiscount: c.MaxDiscount,
		Scope: c.Scope, Products: append([]string(nil), c.Products...), Users: append([]string(nil), c.Users...),
		Quantity: c.Quantity, UsedCount: c.UsedCount, MaxUsagePerUser: c.MaxUsagePerUser,
		StartTime: c.StartTime, ExpireTime: c.ExpireTime, MinOrderAmount: c.MinOrderAmount,
		Currency: c.Currency, Revision: c.Revision}
}

func (s Service) ListCoupons(ctx context.Context, q ListCouponsQuery) ([]CouponListItem, int64, error) {
	items, total, err := s.UOW.Coupons().List(ctx, domain.ListQuery{Page: q.Page, PageSize: q.PageSize, OrganizationID: q.OrganizationID, Keyword: q.Keyword, State: q.State, DiscountType: q.DiscountType})
	if err != nil {
		return nil, 0, err
	}
	r := make([]CouponListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetCoupon(ctx context.Context, organizationID, id string) (*CouponDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	c, err := s.UOW.Coupons().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(c)
	return &d, nil
}
func (s Service) CreateCoupon(ctx context.Context, c CreateCouponCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	state := domain.StateActive
	if c.State != "" {
		state = c.State
	}
	coupon := &domain.Coupon{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Description: c.Description, Code: c.Code, DiscountType: c.DiscountType, Discount: c.Discount,
		MaxDiscount: c.MaxDiscount, Scope: c.Scope, Products: append([]string(nil), c.Products...),
		Users: append([]string(nil), c.Users...), Quantity: c.Quantity, UsedCount: 0,
		MaxUsagePerUser: c.MaxUsagePerUser, StartTime: c.StartTime, ExpireTime: c.ExpireTime,
		MinOrderAmount: c.MinOrderAmount, Currency: c.Currency, State: state,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	if err := coupon.Validate(); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Coupons().Create(ctx, coupon); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			coupon.OrganizationID,
			"coupon.created",
			"coupon",
			coupon.ID,
			"coupon",
			"coupon.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchCoupon(ctx context.Context, organizationID string, c PatchCouponCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		coupon, err := tx.Coupons().GetByIDForOrganization(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && coupon.Revision != c.Revision {
			return apperrors.NewConflict("coupon revision mismatch")
		}
		if c.DisplayName != nil {
			coupon.DisplayName = *c.DisplayName
		}
		if c.Description != nil {
			coupon.Description = *c.Description
		}
		if c.Scope != nil {
			coupon.Scope = *c.Scope
		}
		if c.StartTime != nil {
			coupon.StartTime = *c.StartTime
		}
		if c.ExpireTime != nil {
			coupon.ExpireTime = *c.ExpireTime
		}
		if c.Currency != nil {
			coupon.Currency = *c.Currency
		}
		if c.State != nil {
			if err := coupon.TransitionTo(*c.State); err != nil {
				return err
			}
		}
		if c.Discount != nil {
			coupon.Discount = *c.Discount
		}
		if c.MaxDiscount != nil {
			coupon.MaxDiscount = *c.MaxDiscount
		}
		if c.MinOrderAmount != nil {
			coupon.MinOrderAmount = *c.MinOrderAmount
		}
		if c.Products != nil {
			coupon.Products = append([]string(nil), c.Products...)
		}
		if c.Users != nil {
			coupon.Users = append([]string(nil), c.Users...)
		}
		if c.Quantity != nil {
			coupon.Quantity = *c.Quantity
		}
		if c.MaxUsagePerUser != nil {
			coupon.MaxUsagePerUser = *c.MaxUsagePerUser
		}
		if err := coupon.ValidateTimeWindow(); err != nil {
			return err
		}
		coupon.UpdatedAt = s.Clock.Now()
		coupon.Revision++
		if err := tx.Coupons().Update(ctx, coupon); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			coupon.OrganizationID,
			"coupon.updated",
			"coupon",
			coupon.ID,
			"coupon",
			"coupon.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// ValidateCouponResult holds the result of a coupon validation.
type ValidateCouponResult struct {
	Valid          bool    `json:"valid"`
	DiscountType   string  `json:"discountType,omitempty"`
	Discount       float64 `json:"discount"`
	MaxDiscount    float64 `json:"maxDiscount"`
	MinOrderAmount float64 `json:"minOrderAmount"`
	Currency       string  `json:"currency,omitempty"`
	Message        string  `json:"message,omitempty"`
}

// ValidateCoupon checks if a coupon code is valid for the given context.
// The lookup is an exact, tenant-scoped code match (BE-C-07): a keyword search
// could validate "SUM" against "SUMMER20" and hand out an unintended discount.
func (s Service) ValidateCoupon(ctx context.Context, orgID, code, user string, productNames []string, orderAmount float64) (*ValidateCouponResult, error) {
	if strings.TrimSpace(code) == "" {
		return &ValidateCouponResult{Valid: false, Message: "coupon code is required"}, nil
	}
	lookup, ok := s.UOW.Coupons().(domain.CodeLookup)
	if !ok {
		return nil, apperrors.NewUnimplemented("coupon exact-code lookup is not supported by this dialect")
	}
	c, err := lookup.GetByCode(ctx, orgID, code)
	if err != nil {
		if apperrors.FromError(err).Code == apperrors.CodeNotFound {
			return &ValidateCouponResult{Valid: false, Message: "coupon not found or inactive"}, nil
		}
		return nil, err
	}

	var userCount int64
	if c.MaxUsagePerUser > 0 && user != "" {
		userCount, err = s.UOW.CouponUsages().CountByCouponAndUser(ctx, c.ID, user)
		if err != nil {
			return nil, err
		}
	}
	if c.Scope == "User" && len(c.Users) > 0 && user != "" && !couponAppliesToUser(c.Users, user) {
		return &ValidateCouponResult{Valid: false, Message: "coupon is not applicable to this user"}, nil
	}
	if reason := couponRejectionReason(c, s.Clock.Now(), userCount, productNames, orderAmount); reason != "" {
		return &ValidateCouponResult{Valid: false, Message: reason}, nil
	}

	return &ValidateCouponResult{
		Valid:          true,
		DiscountType:   c.DiscountType,
		Discount:       c.Discount,
		MaxDiscount:    c.MaxDiscount,
		MinOrderAmount: c.MinOrderAmount,
		Currency:       c.Currency,
		Message:        "coupon is valid",
	}, nil
}

// couponAppliesToProducts reports whether any requested product is in the
// coupon's product scope.
func couponAppliesToProducts(products, productNames []string) bool {
	for _, p := range productNames {
		for _, cp := range products {
			if p == cp {
				return true
			}
		}
	}
	return false
}

// couponAppliesToUser reports whether the coupon is restricted to and applies
// to the given user.
func couponAppliesToUser(users []string, user string) bool {
	for _, u := range users {
		if u == user {
			return true
		}
	}
	return false
}

// IncrementCouponUsage increments the used count of a coupon after successful application.
// It also records the usage in the coupon_usage table for per-user tracking.
//
// Order settlement is an internal, system-scoped path outside the management
// by-ID tenant binding, so it keeps the unscoped GetByID lookup. The write is
// an atomic guarded increment (BE-C-06): used_count is bumped in SQL with the
// quantity/per-user guards and the revision predicate, never read-modify-write.
func (s Service) IncrementCouponUsage(ctx context.Context, id, userID, orderID string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		coupon, err := tx.Coupons().GetByID(ctx, id)
		if err != nil {
			return err
		}
		return RecordUsageInTx(ctx, tx, s.Clock, s.UUID, coupon, userID, orderID)
	})
}

// ListCouponUsages returns the usage records for a specific coupon.
func (s Service) ListCouponUsages(ctx context.Context, couponID string, page, pageSize int) ([]domain.CouponUsage, int64, error) {
	items, total, err := s.UOW.CouponUsages().ListByCoupon(ctx, couponID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	result := make([]domain.CouponUsage, 0, len(items))
	for _, item := range items {
		result = append(result, *item)
	}
	return result, total, nil
}

func (s Service) DeleteCoupon(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		coupon, err := tx.Coupons().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.Coupons().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			coupon.OrganizationID,
			"coupon.deleted",
			"coupon",
			id,
			"coupon",
			"coupon.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

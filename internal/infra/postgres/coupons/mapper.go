package coupons

import (
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
)

// scanCoupon scans a full coupon row into a domain Coupon.
func scanCoupon(sc interface{ Scan(...interface{}) error }) (*domain.Coupon, error) {
	var c domain.Coupon
	var products, users []byte
	err := sc.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.DisplayName, &c.Description, &c.Code, &c.DiscountType, &c.Discount, &c.MaxDiscount, &c.Scope, &products, &users, &c.Quantity, &c.UsedCount, &c.MaxUsagePerUser, &c.StartTime, &c.ExpireTime, &c.MinOrderAmount, &c.Currency, &c.State, &c.CreatedAt, &c.UpdatedAt, &c.Revision)
	if err != nil {
		return nil, err
	}
	if len(products) > 0 && string(products) != "null" {
		if err := json.Unmarshal(products, &c.Products); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal products")
		}
	}
	if len(users) > 0 && string(users) != "null" {
		if err := json.Unmarshal(users, &c.Users); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal users")
		}
	}
	if c.Products == nil {
		c.Products = []string{}
	}
	if c.Users == nil {
		c.Users = []string{}
	}
	return &c, nil
}

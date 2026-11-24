package orders

import (
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
)

// scanOrder scans a full order row into a domain Order.
func scanOrder(sc interface{ Scan(...interface{}) error }) (*domain.Order, error) {
	var o domain.Order
	var products, productInfos []byte
	var idempotencyKey *string
	var userID *string
	err := sc.Scan(&o.ID, &o.OrganizationID, &o.Name, &o.DisplayName, &userID, &products, &productInfos, &o.Payment, &o.Price, &o.Currency, &o.State, &o.Message, &o.PaymentURL, &o.TradeNo, &o.CouponName, &o.CouponDiscount, &idempotencyKey, &o.CreatedAt, &o.UpdatedAt, &o.PaidAt, &o.Revision)
	if err != nil {
		return nil, err
	}
	if userID != nil {
		o.User = *userID
	}
	if idempotencyKey != nil {
		o.IdempotencyKey = *idempotencyKey
	}
	if len(products) > 0 && string(products) != "null" {
		if err := json.Unmarshal(products, &o.Products); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal products")
		}
	}
	if len(productInfos) > 0 && string(productInfos) != "null" {
		if err := json.Unmarshal(productInfos, &o.ProductInfos); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal product_infos")
		}
	}
	if o.Products == nil {
		o.Products = []string{}
	}
	if o.ProductInfos == nil {
		o.ProductInfos = []domain.ProductInfo{}
	}
	return &o, nil
}

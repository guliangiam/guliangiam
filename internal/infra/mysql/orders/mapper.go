package orders

import (
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
)

// scanOrder 把 MySQL 表扫描结果转成 domain.Order。
// 列顺序必须与 sql.go queryGetOrderByID / queryListOrders 的 SELECT 严格一致（21 列）。
// products / product_infos JSON 列从 []byte 反序列化为 []string / []ProductInfo。
func scanOrder(sc interface{ Scan(...interface{}) error }) (*domain.Order, error) {
	var o domain.Order
	var productsJSON, productInfosJSON []byte
	var idempotencyKey *string
	err := sc.Scan(
		&o.ID, &o.OrganizationID, &o.Name, &o.DisplayName, &o.User,
		&productsJSON, &productInfosJSON,
		&o.Payment, &o.Price, &o.Currency, &o.State, &o.Message,
		&o.PaymentURL, &o.TradeNo, &o.CouponName, &o.CouponDiscount,
		&idempotencyKey,
		&o.CreatedAt, &o.UpdatedAt, &o.PaidAt, &o.Revision,
	)
	if err != nil {
		return nil, err
	}
	if idempotencyKey != nil {
		o.IdempotencyKey = *idempotencyKey
	}
	if len(productsJSON) > 0 && string(productsJSON) != "null" {
		if err := json.Unmarshal(productsJSON, &o.Products); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal products")
		}
	}
	if o.Products == nil {
		o.Products = []string{}
	}
	if len(productInfosJSON) > 0 && string(productInfosJSON) != "null" {
		if err := json.Unmarshal(productInfosJSON, &o.ProductInfos); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal product_infos")
		}
	}
	if o.ProductInfos == nil {
		o.ProductInfos = []domain.ProductInfo{}
	}
	return &o, nil
}

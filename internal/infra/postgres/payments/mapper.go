package payments

import (
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
)

// scanPayment scans a full payment row into a domain Payment. The projection
// is QueryPaymentColumns (sql.go): every read carries the settlement columns so a
// subsequent Update round-trips them instead of zeroing them (BE-C-08/10).
func scanPayment(sc interface{ Scan(...interface{}) error }) (*domain.Payment, error) {
	var p domain.Payment
	var products []byte
	var idempotencyKey *string
	var userID, orderID *string
	err := sc.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Provider, &p.Type, &products, &p.ProductName, &p.ProductDisplayName, &p.Detail, &p.Currency, &p.Price, &userID, &p.PersonName, &p.PersonIdCard, &p.PersonEmail, &p.PersonPhone, &p.InvoiceType, &p.InvoiceTitle, &p.InvoiceTaxId, &p.InvoiceRemark, &p.InvoiceUrl, &orderID, &p.OutOrderId, &p.PayUrl, &p.SuccessUrl, &p.State, &p.Message, &idempotencyKey, &p.ChannelOrderNo, &p.AttemptCount, &p.RefundedAmount, &p.RefundingAmount, &p.LastErrorCode, &p.LastErrorMessage, &p.CreatedAt, &p.UpdatedAt, &p.Revision)
	if err != nil {
		return nil, err
	}
	if userID != nil {
		p.User = *userID
	}
	if orderID != nil {
		p.Order = *orderID
	}
	if idempotencyKey != nil {
		p.IdempotencyKey = *idempotencyKey
	}
	if len(products) > 0 && string(products) != jsonNullLiteral {
		if err := json.Unmarshal(products, &p.Products); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal products")
		}
	}
	if p.Products == nil {
		p.Products = []string{}
	}
	return &p, nil
}

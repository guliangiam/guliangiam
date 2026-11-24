package payments

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockScanner struct {
	values []interface{}
	err    error
}

func (m *mockScanner) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	if len(dest) > len(m.values) {
		return errors.New("not enough values")
	}
	for i, d := range dest {
		switch v := m.values[i].(type) {
		case int64:
			if ptr, ok := d.(*int64); ok {
				*ptr = v
			}
		case int:
			if ptr, ok := d.(*int); ok {
				*ptr = v
			}
		case float64:
			if ptr, ok := d.(*float64); ok {
				*ptr = v
			}
		case string:
			if ptr, ok := d.(*string); ok {
				*ptr = v
			}
		case []byte:
			if ptr, ok := d.(*[]byte); ok {
				*ptr = v
			}
		case bool:
			if ptr, ok := d.(*bool); ok {
				*ptr = v
			}
		case time.Time:
			if ptr, ok := d.(*time.Time); ok {
				*ptr = v
			}
		case *string:
			if ptr, ok := d.(**string); ok {
				*ptr = v
			}
		case nil:
			// skip nil values
		}
	}
	return nil
}

func TestScanPayment(t *testing.T) {
	now := time.Now()
	products, _ := json.Marshal([]string{"product-1"})
	idempotencyKey := "idem-001"

	t.Run("full_payment", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"pay-001", "org-001", "payment-1", "Payment 1",
				"alipay", "recharge", products,
				"product-1", "Product 1", "detail",
				"CNY", 99.99, "user-001",
				"Zhang San", "110101199001011234", "test@example.com", "13800138000",
				"electronic", "Company Inc", "91110000MA01XXXXXX", "test invoice", "https://inv.url",
				"order-001", "OUT-001", "https://pay.url", "https://success.url",
				"pending", "waiting",
				&idempotencyKey,
				"CH-001", int(2), float64(10.5), float64(5.25), "E_TIMEOUT", "gateway timeout",
				now, now, int64(1),
			},
		}

		payment, err := scanPayment(sc)
		require.NoError(t, err)

		assert.Equal(t, "pay-001", payment.ID)
		assert.Equal(t, "org-001", payment.OrganizationID)
		assert.Equal(t, "alipay", payment.Provider)
		assert.Equal(t, []string{"product-1"}, payment.Products)
		assert.Equal(t, "pending", payment.State)
		assert.Equal(t, "idem-001", payment.IdempotencyKey)
		assert.Equal(t, "CH-001", payment.ChannelOrderNo)
		assert.Equal(t, 2, payment.AttemptCount)
		assert.Equal(t, 10.5, payment.RefundedAmount)
		assert.Equal(t, 5.25, payment.RefundingAmount)
		assert.Equal(t, "E_TIMEOUT", payment.LastErrorCode)
		assert.Equal(t, "gateway timeout", payment.LastErrorMessage)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanPayment(sc)
		assert.Error(t, err)
	})

	t.Run("null_fields", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"pay-002", "org-001", "payment-2", "Payment 2",
				"wechat", "recharge", []byte("null"),
				"product-2", "Product 2", "",
				"CNY", 49.99, "user-002",
				"", "", "", "",
				"", "", "", "",
				"", "order-002", "", "", "",
				"paid", "",
				nil, // nil idempotencyKey
				"", int(0), float64(0), float64(0), "", "",
				now, now, int64(1),
			},
		}

		payment, err := scanPayment(sc)
		require.NoError(t, err)
		assert.Equal(t, []string{}, payment.Products)
		assert.Equal(t, "", payment.IdempotencyKey)
	})
}

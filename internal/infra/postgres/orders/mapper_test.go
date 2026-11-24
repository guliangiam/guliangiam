package orders

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
)

// mockScanner implements the Scan interface for testing.
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
		case *time.Time:
			if ptr, ok := d.(**time.Time); ok {
				*ptr = v
			}
		case nil:
			// skip nil values
		}
	}
	return nil
}

func TestScanOrder(t *testing.T) {
	now := time.Now()
	paidAt := now.Add(-time.Minute)
	products, _ := json.Marshal([]string{"product-1", "product-2"})
	productInfos, _ := json.Marshal([]domain.ProductInfo{
		{Name: "product-1", DisplayName: "Product 1", Price: 9.99},
	})
	idempotencyKey := "idem-key-001"
	userID := "user-001"

	t.Run("full_order", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"order-001",       // ID
				"org-001",         // OrganizationID
				"test-order",      // Name
				"Test Order",      // DisplayName
				&userID,           // User (nullable uuid)
				products,          // Products (JSONB)
				productInfos,      // ProductInfos (JSONB)
				"alipay",          // Payment
				99.99,             // Price
				"CNY",             // Currency
				"paid",            // State
				"",                // Message
				"https://pay.url", // PaymentURL
				"TN123456",        // TradeNo
				"SUMMER10",        // CouponName
				10.0,              // CouponDiscount
				&idempotencyKey,   // IdempotencyKey
				now,               // CreatedAt
				now,               // UpdatedAt
				&paidAt,           // PaidAt
				int64(1),          // Revision
			},
		}

		order, err := scanOrder(sc)
		require.NoError(t, err)

		assert.Equal(t, "order-001", order.ID)
		assert.Equal(t, "org-001", order.OrganizationID)
		assert.Equal(t, "test-order", order.Name)
		assert.Equal(t, "user-001", order.User)
		assert.Equal(t, []string{"product-1", "product-2"}, order.Products)
		assert.Equal(t, "alipay", order.Payment)
		assert.Equal(t, 99.99, order.Price)
		assert.Equal(t, "CNY", order.Currency)
		assert.Equal(t, "paid", order.State)
		require.NotNil(t, order.PaidAt)
		assert.True(t, paidAt.Equal(*order.PaidAt))
		assert.Equal(t, "idem-key-001", order.IdempotencyKey)
		assert.Equal(t, int64(1), order.Revision)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanOrder(sc)
		assert.Error(t, err)
	})

	t.Run("null_products", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"order-002", "org-001", "order2", "Order 2", &userID,
				[]byte("null"), // null products
				[]byte("null"), // null productInfos
				"wechat", 49.99, "USD", "paid", "", "", "", "", 0.0,
				nil, // nil idempotencyKey
				now, now,
				nil, // nil paidAt
				int64(1),
			},
		}

		order, err := scanOrder(sc)
		require.NoError(t, err)
		assert.Equal(t, []string{}, order.Products)
		assert.Equal(t, []domain.ProductInfo{}, order.ProductInfos)
		assert.Equal(t, "", order.IdempotencyKey)
	})
}

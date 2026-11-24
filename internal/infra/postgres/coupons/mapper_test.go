package coupons

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
		case float64:
			if ptr, ok := d.(*float64); ok {
				*ptr = v
			}
		case int:
			if ptr, ok := d.(*int); ok {
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
		case nil:
			// skip nil values
		}
	}
	return nil
}

func TestScanCoupon(t *testing.T) {
	now := time.Now()
	products, _ := json.Marshal([]string{"product-1", "product-2"})
	users, _ := json.Marshal([]string{"user-1", "user-2"})

	t.Run("full_coupon", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"coupon-001", "org-001", "summer-sale", "Summer Sale", "10% off",
				"SUMMER10", "percentage", 10.0, 50.0, "product",
				products, users,
				100, 30, 3,
				"2024-06-01T00:00:00Z", "2024-09-01T00:00:00Z",
				99.99, "CNY", "active",
				now, now, int64(1),
			},
		}

		coupon, err := scanCoupon(sc)
		require.NoError(t, err)

		assert.Equal(t, "coupon-001", coupon.ID)
		assert.Equal(t, "org-001", coupon.OrganizationID)
		assert.Equal(t, "summer-sale", coupon.Name)
		assert.Equal(t, "SUMMER10", coupon.Code)
		assert.Equal(t, "percentage", coupon.DiscountType)
		assert.Equal(t, 10.0, coupon.Discount)
		assert.Equal(t, []string{"product-1", "product-2"}, coupon.Products)
		assert.Equal(t, []string{"user-1", "user-2"}, coupon.Users)
		assert.Equal(t, 100, coupon.Quantity)
		assert.Equal(t, 30, coupon.UsedCount)
		assert.Equal(t, "active", coupon.State)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanCoupon(sc)
		assert.Error(t, err)
	})

	t.Run("null_arrays", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"coupon-002", "org-001", "fixed-coupon", "Fixed Coupon", "",
				"FIXED5", "fixed", 5.0, 0.0, "",
				[]byte("null"), []byte("null"),
				0, 0, 0,
				"", "",
				0.0, "", "active",
				now, now, int64(1),
			},
		}

		coupon, err := scanCoupon(sc)
		require.NoError(t, err)
		assert.Equal(t, []string{}, coupon.Products)
		assert.Equal(t, []string{}, coupon.Users)
	})
}

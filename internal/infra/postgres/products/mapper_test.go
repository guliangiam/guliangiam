package products

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
		case int:
			if ptr, ok := d.(*int); ok {
				*ptr = v
			}
		case nil:
			// skip
		}
	}
	return nil
}

func TestScanProduct(t *testing.T) {
	now := time.Now()
	rechargeOpts, _ := json.Marshal([]float64{10.0, 20.0, 50.0})
	providers, _ := json.Marshal([]string{"alipay", "wechat"})

	t.Run("full_product", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"prod-001", "org-001", "vpn-plan", "VPN Plan",
				"https://img.url/vpn.png", "VPN access plan", "VPN subscription",
				"vpn", "CNY", 29.99, 100, 50,
				false, rechargeOpts, false, providers,
				"https://app.example.com/success", "active",
				now, now, int64(1),
			},
		}

		product, err := scanProduct(sc)
		require.NoError(t, err)

		assert.Equal(t, "prod-001", product.ID)
		assert.Equal(t, "org-001", product.OrganizationID)
		assert.Equal(t, "vpn-plan", product.Name)
		assert.Equal(t, 29.99, product.Price)
		assert.Equal(t, 100, product.Quantity)
		assert.Equal(t, 50, product.Sold)
		assert.Equal(t, []float64{10.0, 20.0, 50.0}, product.RechargeOptions)
		assert.Equal(t, []string{"alipay", "wechat"}, product.Providers)
		assert.Equal(t, "active", product.State)
	})

	t.Run("null_arrays", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"prod-002", "org-001", "simple", "Simple Product",
				"", "", "",
				"", "CNY", 9.99, 10, 0,
				false, []byte("null"), false, []byte("null"),
				"", "draft",
				now, now, int64(1),
			},
		}

		product, err := scanProduct(sc)
		require.NoError(t, err)
		assert.Equal(t, []float64{}, product.RechargeOptions)
		assert.Equal(t, []string{}, product.Providers)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanProduct(sc)
		assert.Error(t, err)
	})
}

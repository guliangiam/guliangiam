package pricings

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

func TestScanPricing(t *testing.T) {
	now := time.Now()
	plans, _ := json.Marshal([]string{"plan-001", "plan-002"})

	t.Run("full_pricing", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"pricing-001", "org-001", "standard", "Standard Pricing",
				"Standard pricing tier", plans,
				true, 14, "app-001",
				now, now, int64(1),
			},
		}

		pricing, err := scanPricing(sc)
		require.NoError(t, err)

		assert.Equal(t, "pricing-001", pricing.ID)
		assert.Equal(t, "org-001", pricing.OrganizationID)
		assert.Equal(t, "standard", pricing.Name)
		assert.Equal(t, []string{"plan-001", "plan-002"}, pricing.Plans)
		assert.True(t, pricing.IsEnabled)
		assert.Equal(t, 14, pricing.TrialDuration)
		assert.Equal(t, "app-001", pricing.Application)
	})

	t.Run("null_plans", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"pricing-002", "org-001", "free", "Free Pricing",
				"", []byte("null"),
				false, 0, "",
				now, now, int64(1),
			},
		}

		pricing, err := scanPricing(sc)
		require.NoError(t, err)
		assert.Equal(t, []string{}, pricing.Plans)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanPricing(sc)
		assert.Error(t, err)
	})
}

package plans

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
		case nil:
			// skip
		}
	}
	return nil
}

func TestScanPlan(t *testing.T) {
	now := time.Now()
	providers, _ := json.Marshal([]string{"alipay", "wechat"})

	t.Run("full_plan", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"plan-001", "org-001", "monthly", "Monthly Plan",
				"Monthly subscription", 29.99, "CNY", "monthly",
				"product-001", providers,
				"active", false, "role-member",
				now, now, int64(1),
			},
		}

		plan, err := scanPlan(sc)
		require.NoError(t, err)

		assert.Equal(t, "plan-001", plan.ID)
		assert.Equal(t, "org-001", plan.OrganizationID)
		assert.Equal(t, "monthly", plan.Name)
		assert.Equal(t, 29.99, plan.Price)
		assert.Equal(t, "CNY", plan.Currency)
		assert.Equal(t, "monthly", plan.Period)
		assert.Equal(t, []string{"alipay", "wechat"}, plan.PaymentProviders)
		assert.Equal(t, "active", plan.State)
		assert.False(t, plan.IsExclusive)
		assert.Equal(t, "role-member", plan.Role)
	})

	t.Run("null_providers", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"plan-002", "org-001", "free", "Free Plan",
				"", 0.0, "", "lifetime",
				"", []byte("null"),
				"draft", false, "",
				now, now, int64(1),
			},
		}

		plan, err := scanPlan(sc)
		require.NoError(t, err)
		assert.Equal(t, []string{}, plan.PaymentProviders)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanPlan(sc)
		assert.Error(t, err)
	})
}

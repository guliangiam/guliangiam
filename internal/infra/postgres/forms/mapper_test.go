package forms

import (
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

func TestScanForm(t *testing.T) {
	now := time.Now()

	t.Run("full_form", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"form-001", "org-001", "registration", "Registration Form",
				"User registration form", "registration",
				`{"fields":[{"name":"email","type":"text"}]}`,
				true, now, now, int64(1),
			},
		}

		form, err := scanForm(sc)
		require.NoError(t, err)

		assert.Equal(t, "form-001", form.ID)
		assert.Equal(t, "org-001", form.OrganizationID)
		assert.Equal(t, "registration", form.Name)
		assert.Equal(t, "Registration Form", form.DisplayName)
		assert.Equal(t, "User registration form", form.Description)
		assert.Equal(t, "registration", form.Type)
		assert.True(t, form.IsEnabled)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanForm(sc)
		assert.Error(t, err)
	})
}

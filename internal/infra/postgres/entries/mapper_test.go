package entries

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
		case float64:
			if ptr, ok := d.(*float64); ok {
				*ptr = v
			}
		case string:
			if ptr, ok := d.(*string); ok {
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

func TestScanEntry(t *testing.T) {
	now := time.Now()

	t.Run("full_entry", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"entry-001", "org-001", "login-entry", "Login Entry",
				"github", "app-001", "login",
				"192.168.1.1", "Mozilla/5.0", "Login successful",
				now, now, int64(1),
			},
		}

		entry, err := scanEntry(sc)
		require.NoError(t, err)

		assert.Equal(t, "entry-001", entry.ID)
		assert.Equal(t, "org-001", entry.OrganizationID)
		assert.Equal(t, "login-entry", entry.Name)
		assert.Equal(t, "github", entry.Provider)
		assert.Equal(t, "app-001", entry.Application)
		assert.Equal(t, "login", entry.Type)
		assert.Equal(t, "192.168.1.1", entry.ClientIP)
		assert.Equal(t, "Mozilla/5.0", entry.UserAgent)
		assert.Equal(t, "Login successful", entry.Message)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanEntry(sc)
		assert.Error(t, err)
	})
}

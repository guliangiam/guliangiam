package adapters

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAdapter_Creation(t *testing.T) {
	now := time.Now()
	port := 5432
	a := Adapter{
		ID:             "adapter-001",
		OrganizationID: "org-001",
		Name:           "postgres-adapter",
		DisplayName:    "PostgreSQL Adapter",
		Type:           "database",
		Host:           "localhost",
		Port:           &port,
		DatabaseName:   "guliangiam",
		DbUser:         "guliangiam",
		IsEnabled:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	assert.Equal(t, "adapter-001", a.ID)
	assert.Equal(t, "database", a.Type)
	assert.True(t, a.IsEnabled)
	assert.NotNil(t, a.Port)
	assert.Equal(t, 5432, *a.Port)
}

func TestAdapter_Types(t *testing.T) {
	validTypes := []string{"database", "file"}

	for _, adapterType := range validTypes {
		t.Run("type_"+adapterType, func(t *testing.T) {
			a := Adapter{Type: adapterType}
			assert.Equal(t, adapterType, a.Type)
		})
	}
}

func TestAdapter_Disabled(t *testing.T) {
	a := Adapter{Name: "disabled-adapter", IsEnabled: false}
	assert.False(t, a.IsEnabled)
}

package gateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToServer(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		deletedAt := now.Add(-time.Hour)
		row := serverRow{
			ID:             "srv-001",
			OrganizationID: "org-001",
			Name:           "main-server",
			Endpoint:       "https://gw.example.com",
			Protocol:       "https",
			Status:         "active",
			Metadata:       map[string]any{"version": "1.0"},
			CreatedAt:      now,
			UpdatedAt:      now,
			DeletedAt:      &deletedAt,
			Revision:       1,
		}

		result := toServer(row)

		assert.Equal(t, "srv-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "main-server", result.Name)
		assert.Equal(t, "https://gw.example.com", result.Endpoint)
		assert.Equal(t, "https", result.Protocol)
		assert.Equal(t, "active", result.Status)
		assert.Equal(t, "1.0", result.Metadata["version"])
		assert.Equal(t, &deletedAt, result.DeletedAt)
	})

	t.Run("nil_deleted_at", func(t *testing.T) {
		now := time.Now()
		row := serverRow{
			ID:        "srv-002",
			DeletedAt: nil,
			CreatedAt: now,
			UpdatedAt: now,
		}

		result := toServer(row)

		assert.Nil(t, result.DeletedAt)
	})
}

func TestToSite(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		lastHealthAt := now.Add(-5 * time.Minute)
		row := siteRow{
			ID:                  "site-001",
			OrganizationID:      "org-001",
			Name:                "main-site",
			Host:                "app.example.com",
			Status:              "active",
			SslMode:             "full",
			CertID:              "cert-001",
			ForceHttps:          true,
			EnableHealthCheck:   true,
			HealthCheckInterval: 30,
			LastHealthStatus:    "healthy",
			LastHealthAt:        &lastHealthAt,
			EnableAlert:         true,
			AlertInterval:       60,
			AlertTryTimes:       3,
			AlertProviderIDs:    []string{"prov-001", "prov-002"},
			Metadata:            map[string]any{"region": "us-east-1"},
			CreatedAt:           now,
			UpdatedAt:           now,
			DeletedAt:           nil,
			Revision:            1,
		}

		result := toSite(row)

		assert.Equal(t, "site-001", result.ID)
		assert.Equal(t, "main-site", result.Name)
		assert.Equal(t, "app.example.com", result.Host)
		assert.Equal(t, "full", result.SslMode)
		assert.True(t, result.ForceHttps)
		assert.True(t, result.EnableHealthCheck)
		assert.Equal(t, 30, result.HealthCheckInterval)
		assert.Equal(t, "healthy", result.LastHealthStatus)
		assert.Equal(t, &lastHealthAt, result.LastHealthAt)
		assert.Equal(t, []string{"prov-001", "prov-002"}, result.AlertProviderIDs)
		assert.Nil(t, result.DeletedAt)
	})
}

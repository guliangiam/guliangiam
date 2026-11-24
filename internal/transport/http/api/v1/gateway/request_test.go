package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListServersRequest_ToQuery(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := ListServersRequest{
			Page:           1,
			PageSize:       20,
			OrganizationID: "org-001",
			Keyword:        "main",
			Status:         "active",
		}

		q := req.toQuery()

		assert.Equal(t, 1, q.Page)
		assert.Equal(t, 20, q.PageSize)
		assert.Equal(t, "org-001", q.OrganizationID)
		assert.Equal(t, "main", q.Keyword)
		assert.Equal(t, "active", q.Status)
	})
}

func TestCreateServerRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := CreateServerRequest{
			OrganizationID: "org-001",
			Name:           "main-server",
			Endpoint:       "https://gw.example.com",
			Protocol:       "https",
			Status:         "active",
			Metadata:       map[string]any{"version": "1.0"},
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "main-server", cmd.Name)
		assert.Equal(t, "https", cmd.Protocol)
		assert.Equal(t, "1.0", cmd.Metadata["version"])
	})
}

func TestUpdateServerRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := UpdateServerRequest{
			ID:       "srv-001",
			Name:     "updated-server",
			Endpoint: "https://gw2.example.com",
			Protocol: "https",
			Status:   "inactive",
			Revision: 2,
		}

		cmd := req.toCommand()

		assert.Equal(t, "srv-001", cmd.ID)
		assert.Equal(t, "updated-server", cmd.Name)
		assert.Equal(t, int64(2), cmd.Revision)
	})
}

func TestListSitesRequest_ToQuery(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := ListSitesRequest{
			Page:           1,
			PageSize:       10,
			OrganizationID: "org-001",
			Keyword:        "app",
			Status:         "active",
		}

		q := req.toQuery()

		assert.Equal(t, 1, q.Page)
		assert.Equal(t, "org-001", q.OrganizationID)
	})
}

func TestCreateSiteRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := CreateSiteRequest{
			OrganizationID:      "org-001",
			Name:                "main-site",
			Host:                "app.example.com",
			Status:              "active",
			SslMode:             "full",
			ForceHttps:          true,
			EnableHealthCheck:   true,
			HealthCheckInterval: 30,
			EnableAlert:         true,
			AlertInterval:       60,
			AlertTryTimes:       3,
			AlertProviderIDs:    []string{"prov-001"},
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "main-site", cmd.Name)
		assert.True(t, cmd.ForceHttps)
		assert.Equal(t, []string{"prov-001"}, cmd.AlertProviderIDs)
	})
}

func TestUpdateSiteRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := UpdateSiteRequest{
			ID:       "site-001",
			Name:     "updated-site",
			Host:     "new.example.com",
			Revision: 2,
		}

		cmd := req.toCommand()

		assert.Equal(t, "site-001", cmd.ID)
		assert.Equal(t, "updated-site", cmd.Name)
		assert.Equal(t, int64(2), cmd.Revision)
	})
}

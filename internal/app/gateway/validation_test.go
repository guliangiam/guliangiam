package gateway

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─── CreateServer ──────────────────────────────────────────────────────────

func TestValidateCreateServerCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateServerCommand{
			OrganizationID: "org-001",
			Name:           "api-server",
			Endpoint:       "https://api.example.com",
			Protocol:       "https",
			Status:         "active",
		}
		assert.NoError(t, validateCreateServerCommand(cmd))
	})

	t.Run("missing_organization_id", func(t *testing.T) {
		cmd := CreateServerCommand{Name: "s", Endpoint: "https://x"}
		assert.Error(t, validateCreateServerCommand(cmd))
	})

	t.Run("missing_name", func(t *testing.T) {
		cmd := CreateServerCommand{OrganizationID: "org-1", Endpoint: "https://x"}
		assert.Error(t, validateCreateServerCommand(cmd))
	})

	t.Run("missing_endpoint", func(t *testing.T) {
		cmd := CreateServerCommand{OrganizationID: "org-1", Name: "s"}
		assert.Error(t, validateCreateServerCommand(cmd))
	})

	t.Run("blank_name_with_whitespace", func(t *testing.T) {
		cmd := CreateServerCommand{
			OrganizationID: "org-1",
			Name:           strings.Repeat(" ", 5),
			Endpoint:       "https://x",
		}
		assert.Error(t, validateCreateServerCommand(cmd))
	})

	t.Run("blank_endpoint_with_whitespace", func(t *testing.T) {
		cmd := CreateServerCommand{
			OrganizationID: "org-1",
			Name:           "s",
			Endpoint:       strings.Repeat(" ", 3),
		}
		assert.Error(t, validateCreateServerCommand(cmd))
	})

	t.Run("protocol_status_not_enforced_here", func(t *testing.T) {
		// See design note at top of validation.go — enum closure is intentionally
		// not enforced here to avoid false-positive rejects while the schema
		// remains TEXT-without-CHECK.
		cmd := CreateServerCommand{
			OrganizationID: "org-1",
			Name:           "s",
			Endpoint:       "https://x",
			Protocol:       "some-future-transport",
			Status:         "weird-state",
		}
		assert.NoError(t, validateCreateServerCommand(cmd))
	})
}

// ─── UpdateServer ──────────────────────────────────────────────────────────

func TestValidateUpdateServerCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := UpdateServerCommand{ID: "srv-001", Name: "new-name"}
		assert.NoError(t, validateUpdateServerCommand(cmd))
	})

	t.Run("missing_id", func(t *testing.T) {
		cmd := UpdateServerCommand{Name: "new-name"}
		assert.Error(t, validateUpdateServerCommand(cmd))
	})

	t.Run("blank_name_rejected", func(t *testing.T) {
		cmd := UpdateServerCommand{ID: "srv-001", Name: "   "}
		assert.Error(t, validateUpdateServerCommand(cmd))
	})
}

// ─── CreateSite ────────────────────────────────────────────────────────────

func TestValidateCreateSiteCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateSiteCommand{
			OrganizationID: "org-001",
			Name:           "main-site",
			Host:           "example.com",
			SslMode:        "auto",
			Status:         "active",
		}
		assert.NoError(t, validateCreateSiteCommand(cmd))
	})

	t.Run("missing_organization_id", func(t *testing.T) {
		cmd := CreateSiteCommand{Name: "s", Host: "example.com"}
		assert.Error(t, validateCreateSiteCommand(cmd))
	})

	t.Run("missing_name", func(t *testing.T) {
		cmd := CreateSiteCommand{OrganizationID: "org-1", Host: "example.com"}
		assert.Error(t, validateCreateSiteCommand(cmd))
	})

	t.Run("missing_host", func(t *testing.T) {
		cmd := CreateSiteCommand{OrganizationID: "org-1", Name: "s"}
		assert.Error(t, validateCreateSiteCommand(cmd))
	})

	t.Run("blank_host_with_whitespace", func(t *testing.T) {
		cmd := CreateSiteCommand{
			OrganizationID: "org-1", Name: "s", Host: strings.Repeat(" ", 5),
		}
		assert.Error(t, validateCreateSiteCommand(cmd))
	})
}

// ─── UpdateSite ────────────────────────────────────────────────────────────

func TestValidateUpdateSiteCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := UpdateSiteCommand{ID: "site-001", Name: "new-name"}
		assert.NoError(t, validateUpdateSiteCommand(cmd))
	})

	t.Run("missing_id", func(t *testing.T) {
		cmd := UpdateSiteCommand{Name: "new-name"}
		assert.Error(t, validateUpdateSiteCommand(cmd))
	})

	t.Run("blank_name_rejected", func(t *testing.T) {
		cmd := UpdateSiteCommand{ID: "site-001", Name: "   "}
		assert.Error(t, validateUpdateSiteCommand(cmd))
	})
}

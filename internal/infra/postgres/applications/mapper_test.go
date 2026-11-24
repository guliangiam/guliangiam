package applications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDerefString(t *testing.T) {
	t.Run("nil_pointer", func(t *testing.T) {
		assert.Equal(t, "", derefString(nil))
	})

	t.Run("non_nil_pointer", func(t *testing.T) {
		val := "test-value"
		assert.Equal(t, "test-value", derefString(&val))
	})
}

func TestToApplication_BasicFields(t *testing.T) {
	t.Run("basic_mapping", func(t *testing.T) {
		desc := "My Application"
		homepage := "https://myapp.example.com"
		domain := "myapp.example.com"
		clientID := "gl_client_abc123"
		clientSecretHash := "sha256hash"

		row := applicationRow{
			ID:               "app-001",
			OrganizationID:   "org-001",
			Slug:             "my-app",
			DisplayName:      "My App",
			Description:      &desc,
			HomepageURL:      &homepage,
			Domain:           &domain,
			Status:           "active",
			ClientID:         &clientID,
			ClientSecretHash: &clientSecretHash,
			RedirectURIs:     []string{"https://myapp.example.com/callback"},
			GrantTypes:       []string{"authorization_code", "refresh_token"},
		}

		app, err := toApplication(row)
		assert.NoError(t, err)
		assert.Equal(t, "app-001", app.ID)
		assert.Equal(t, "org-001", app.OrganizationID)
		assert.Equal(t, "My App", app.DisplayName)
		assert.Equal(t, "My Application", app.Description)
		assert.Equal(t, "https://myapp.example.com", app.HomepageURL)
		assert.Equal(t, "gl_client_abc123", app.ClientID)
		assert.Equal(t, []string{"authorization_code", "refresh_token"}, app.GrantTypes)
	})

	t.Run("nil_optional_fields", func(t *testing.T) {
		row := applicationRow{
			ID:               "app-002",
			OrganizationID:   "org-001",
			Slug:             "minimal",
			DisplayName:      "Minimal App",
			Description:      nil,
			HomepageURL:      nil,
			ClientID:         nil,
			ClientSecretHash: nil,
		}

		app, err := toApplication(row)
		assert.NoError(t, err)
		assert.Equal(t, "", app.Description)
		assert.Equal(t, "", app.HomepageURL)
		assert.Equal(t, "", app.ClientID)
		assert.Equal(t, "", app.ClientSecretHash)
	})
}

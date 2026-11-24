package applications

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/stretchr/testify/assert"
)

func TestToCustomScopeItems(t *testing.T) {
	scopes := []domain.ApplicationCustomScope{
		{Scope: "read", DisplayName: "Read Access", Description: "Read permission"},
		{Scope: "write", DisplayName: "Write Access", Description: "Write permission"},
	}
	items := toCustomScopeItems(scopes)
	assert.Len(t, items, 2)
	assert.Equal(t, "read", items[0].Scope)
	assert.Equal(t, "Read Access", items[0].DisplayName)
	assert.Equal(t, "Read permission", items[0].Description)
	assert.Equal(t, "write", items[1].Scope)
}

func TestToCustomScopeItems_Empty(t *testing.T) {
	items := toCustomScopeItems(nil)
	assert.Empty(t, items)
	assert.NotNil(t, items)
}

func TestToExportDTO(t *testing.T) {
	app := &domain.Application{
		Slug:         "my-app",
		DisplayName:  "My Application",
		Category:     "web",
		AppType:      "native",
		ClientID:     "client-123",
		IsPortal:     true,
		IsVisible:    false,
		Status:       "active",
		RedirectURIs: []string{"https://example.com/callback"},
		GrantTypes:   []string{"authorization_code", "refresh_token"},
		CustomScopes: []domain.ApplicationCustomScope{
			{Scope: "custom:read", DisplayName: "Custom Read", Description: "Custom read scope"},
		},
		EnableSignup:      true,
		ConsentMode:       "explicit",
		FailedSigninLimit: 5,
	}

	dto := toExportDTO(app)
	assert.Equal(t, "my-app", dto.Slug)
	assert.Equal(t, "My Application", dto.DisplayName)
	assert.Equal(t, "web", dto.Category)
	assert.Equal(t, "native", dto.AppType)
	assert.Equal(t, "client-123", dto.ClientID)
	assert.True(t, dto.IsPortal)
	assert.False(t, dto.IsVisible)
	assert.Equal(t, "active", dto.Status)
	assert.Equal(t, []string{"https://example.com/callback"}, dto.RedirectURIs)
	assert.Equal(t, []string{"authorization_code", "refresh_token"}, dto.GrantTypes)
	assert.Len(t, dto.CustomScopes, 1)
	assert.Equal(t, "custom:read", dto.CustomScopes[0].Scope)
	assert.True(t, dto.EnableSignup)
	assert.Equal(t, "explicit", dto.ConsentMode)
	assert.Equal(t, 5, dto.FailedSigninLimit)
}

func TestToExportDTO_Minimal(t *testing.T) {
	app := &domain.Application{
		Slug:        "minimal",
		DisplayName: "Minimal App",
		ClientID:    "client-min",
	}

	dto := toExportDTO(app)
	assert.Equal(t, "minimal", dto.Slug)
	assert.Equal(t, "Minimal App", dto.DisplayName)
	assert.Equal(t, "client-min", dto.ClientID)
	assert.Empty(t, dto.RedirectURIs)
	assert.Empty(t, dto.CustomScopes)
}

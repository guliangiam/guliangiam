package applications

import (
	"testing"

	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

func TestCloneApplication_Nil(t *testing.T) {
	result := cloneApplication(nil)
	assert.Nil(t, result)
}

func TestCloneApplication_Basic(t *testing.T) {
	app := &domain.Application{
		ID:             "app1",
		OrganizationID: "org1",
		Slug:           "test-app",
		DisplayName:    "Test App",
		RedirectURIs:   []string{"https://example.com/callback"},
		GrantTypes:     []string{"authorization_code"},
		CustomScopes: []domain.ApplicationCustomScope{
			{Scope: "scope1", DisplayName: "Scope 1"},
		},
		ProviderBindings: []domain.ApplicationProviderBinding{
			{ProviderID: "prov1"},
		},
		TokenAttributes: []domain.TokenAttribute{
			{Name: "attr1"},
		},
		SamlAttributes: []domain.SamlAttribute{
			{Name: "saml1"},
		},
		OtherDomains:  []string{"other.example.com"},
		SigninMethods: []string{"password"},
		SigninItems:   []domain.SigninItemConfig{{Name: "item1"}},
	}

	cloned := cloneApplication(app)

	// Verify it's a different pointer
	assert.NotSame(t, app, cloned)

	// Verify scalar fields are copied
	assert.Equal(t, "app1", cloned.ID)
	assert.Equal(t, "org1", cloned.OrganizationID)
	assert.Equal(t, "test-app", cloned.Slug)
	assert.Equal(t, "Test App", cloned.DisplayName)

	// Verify slices are independent
	cloned.RedirectURIs[0] = "modified"
	assert.Equal(t, "https://example.com/callback", app.RedirectURIs[0])

	cloned.GrantTypes[0] = "modified"
	assert.Equal(t, "authorization_code", app.GrantTypes[0])

	cloned.CustomScopes[0].Scope = "modified"
	assert.Equal(t, "scope1", app.CustomScopes[0].Scope)

	cloned.ProviderBindings[0].ProviderID = "modified"
	assert.Equal(t, "prov1", app.ProviderBindings[0].ProviderID)

	cloned.OtherDomains[0] = "modified"
	assert.Equal(t, "other.example.com", app.OtherDomains[0])
}

func TestCloneApplication_EmptySlices(t *testing.T) {
	app := &domain.Application{
		ID:           "app1",
		RedirectURIs: []string{},
		GrantTypes:   nil,
	}

	cloned := cloneApplication(app)
	assert.Equal(t, "app1", cloned.ID)
	assert.Empty(t, cloned.RedirectURIs)
	assert.Nil(t, cloned.GrantTypes)
}

func TestDerefString_Nil(t *testing.T) {
	result := derefString(nil)
	assert.Equal(t, "", result)
}

func TestDerefString_NonNil(t *testing.T) {
	s := "hello"
	result := derefString(&s)
	assert.Equal(t, "hello", result)
}

func TestDerefString_Empty(t *testing.T) {
	s := ""
	result := derefString(&s)
	assert.Equal(t, "", result)
}

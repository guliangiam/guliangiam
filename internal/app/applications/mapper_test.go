package applications

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

func TestToListItem(t *testing.T) {
	now := time.Now()
	app := &domain.Application{
		ID: "a1", OrganizationID: "o1", Slug: "my-app", DisplayName: "My App",
		Category: "web", AppType: "spa", Domain: "example.com", Status: "active",
		Logo: "logo.png", HomepageURL: "https://myapp.example.com", IsVisible: true,
		ProviderBindings: []domain.ApplicationProviderBinding{
			{ProviderID: "p1"}, {ProviderID: "p2"},
		},
		CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(app)
	if item.ID != "a1" || item.Slug != "my-app" || item.Name != "my-app" {
		t.Fatalf("unexpected: %+v", item)
	}
	if item.HomepageURL != "https://myapp.example.com" {
		t.Fatalf("HomepageURL = %q", item.HomepageURL)
	}
	if len(item.ProviderIDs) != 2 {
		t.Fatalf("got %d", len(item.ProviderIDs))
	}
}

func TestToDetail(t *testing.T) {
	now := time.Now()
	app := &domain.Application{
		ID: "a1", OrganizationID: "o1", Slug: "my-app", DisplayName: "My App",
		Category: "web", AppType: "spa", Status: "active",
		ClientID: "cid", RedirectURIs: []string{"https://example.com/cb"},
		GrantTypes:  []string{"authorization_code"},
		HomepageURL: "https://my-app.example.com",
		CustomScopes: []domain.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read", Description: "Read access"},
		},
		CreatedAt: now, UpdatedAt: now,
	}
	detail := toDetail(app)
	if detail.ClientID != "cid" {
		t.Fatalf("got %s", detail.ClientID)
	}
	if detail.HomepageURL != "https://my-app.example.com" {
		t.Fatalf("HomepageURL = %q", detail.HomepageURL)
	}
	if len(detail.RedirectURIs) != 1 || detail.RedirectURIs[0] != "https://example.com/cb" {
		t.Fatalf("got %v", detail.RedirectURIs)
	}
	if len(detail.CustomScopes) != 1 || detail.CustomScopes[0].Scope != "read" {
		t.Fatalf("got %v", detail.CustomScopes)
	}
}

func TestToProviderBindingItems(t *testing.T) {
	bindings := []domain.ApplicationProviderBinding{
		{ProviderID: "p1"}, {ProviderID: "p2"},
	}
	items := toProviderBindingItems(bindings)
	if len(items) != 2 || items[0].ProviderID != "p1" {
		t.Fatalf("got %v", items)
	}
}

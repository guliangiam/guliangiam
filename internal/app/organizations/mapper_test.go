package organizations

import (
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

func TestOrgToListItem(t *testing.T) {
	now := time.Now()
	org := &orgdomain.Organization{
		ID: "o1", Slug: "myorg", DisplayName: "My Org",
		WebsiteURL: "https://example.com", LogoURL: "logo.png", Status: "active",
		Balance: 100.0, Currency: "USD",
		CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(org)
	if item.ID != "o1" || item.Slug != "myorg" || item.DisplayName != "My Org" {
		t.Fatalf("unexpected: %+v", item)
	}
	if item.Balance != 100.0 || item.Currency != "USD" {
		t.Fatalf("unexpected: %+v", item)
	}
}

func TestOrgToDetail(t *testing.T) {
	now := time.Now()
	org := &orgdomain.Organization{
		ID: "o1", Slug: "myorg", DisplayName: "My Org",
		Status: "active", LogoDarkURL: "dark.png", FaviconURL: "fav.ico",
		InitScore: 100, DefaultAvatar: "avatar.png",
		CreatedAt: now, UpdatedAt: now,
	}
	detail := toDetail(org)
	if detail.ID != "o1" || detail.LogoDarkURL != "dark.png" {
		t.Fatalf("unexpected: %+v", detail)
	}
	if detail.InitScore != 100 {
		t.Fatalf("got %d", detail.InitScore)
	}
	if detail.Settings == nil {
		t.Fatal("settings nil")
	}
}

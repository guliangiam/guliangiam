package notificationproviders

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	"github.com/stretchr/testify/assert"
)

func TestToListItem(t *testing.T) {
	now := time.Now()
	p := &domain.NotificationProvider{
		ID: "np-1", OrganizationID: "org-1", Name: "webhook",
		ProviderType: "webhook", WebhookURL: "https://example.com/hook",
		IsEnabled: true, CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(p)
	assert.Equal(t, "np-1", item.ID)
	assert.Equal(t, "webhook", item.Name)
	assert.Equal(t, "https://example.com/hook", item.WebhookURL)
}

func TestToDetail(t *testing.T) {
	p := &domain.NotificationProvider{
		ID: "np-1", OrganizationID: "org-1", Name: "webhook",
		AccessToken: "token123", Config: map[string]any{"key": "val"}, Revision: 1,
	}
	detail := toDetail(p)
	assert.Equal(t, "np-1", detail.ID)
	assert.True(t, detail.AccessTokenSet)
	assert.Equal(t, "to***23", detail.AccessTokenMasked)
	assert.Equal(t, int64(1), detail.Revision)
}

func TestToDetail_NoAccessToken(t *testing.T) {
	p := &domain.NotificationProvider{ID: "np-2", OrganizationID: "org-1"}
	detail := toDetail(p)
	assert.False(t, detail.AccessTokenSet)
	assert.Empty(t, detail.AccessTokenMasked)
}

package notificationproviders

import domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"

func toListItem(provider *domain.NotificationProvider) NotificationProviderListItem {
	return NotificationProviderListItem{
		ID:             provider.ID,
		OrganizationID: provider.OrganizationID,
		Name:           provider.Name,
		ProviderType:   provider.ProviderType,
		WebhookURL:     provider.WebhookURL,
		IsEnabled:      provider.IsEnabled,
		CreatedAt:      provider.CreatedAt,
		UpdatedAt:      provider.UpdatedAt,
	}
}

func toDetail(provider *domain.NotificationProvider) NotificationProviderDetail {
	return NotificationProviderDetail{
		NotificationProviderListItem: toListItem(provider),
		AccessTokenSet:               provider.AccessToken != "",
		AccessTokenMasked:            maskSecret(provider.AccessToken),
		Config:                       provider.Config,
		Revision:                     provider.Revision,
	}
}

// maskSecret returns first 2 chars + *** + last 2 chars, or empty string.
// Short secrets (< 6 chars) return fully masked "******".
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) < 6 {
		return "******"
	}
	return s[:2] + "***" + s[len(s)-2:]
}

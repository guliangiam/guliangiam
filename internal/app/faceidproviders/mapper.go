package faceidproviders

import domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"

func toListItem(provider *domain.FaceIdProvider) FaceIdProviderListItem {
	return FaceIdProviderListItem{
		ID:                           provider.ID,
		OrganizationID:               provider.OrganizationID,
		Name:                         provider.Name,
		ProviderType:                 provider.ProviderType,
		AccessKeyID:                  provider.AccessKeyID,
		Endpoint:                     provider.Endpoint,
		IsEnabled:                    provider.IsEnabled,
		CreatedAt:                    provider.CreatedAt,
		UpdatedAt:                    provider.UpdatedAt,
		AccessKeySecretNeedsRotation: provider.SecretNeedsRotation,
	}
}

func toDetail(provider *domain.FaceIdProvider) FaceIdProviderDetail {
	config := make(map[string]any)
	for k, v := range provider.Config {
		config[k] = v
	}
	return FaceIdProviderDetail{
		FaceIdProviderListItem: toListItem(provider),
		AccessKeySecretSet:     provider.AccessKeySecret != "",
		AccessKeySecretMasked:  maskSecret(provider.AccessKeySecret),
		Config:                 config,
		Revision:               provider.Revision,
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

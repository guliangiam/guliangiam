package radiusconfigs

import domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"

func toListItem(config *domain.RadiusConfig) RadiusConfigListItem {
	return RadiusConfigListItem{
		ID:             config.ID,
		OrganizationID: config.OrganizationID,
		Name:           config.Name,
		Host:           config.Host,
		Port:           config.Port,
		AuthProtocol:   config.AuthProtocol,
		IsEnabled:      config.IsEnabled,
		CreatedAt:      config.CreatedAt,
		UpdatedAt:      config.UpdatedAt,
	}
}

func toDetail(config *domain.RadiusConfig) RadiusConfigDetail {
	return RadiusConfigDetail{
		RadiusConfigListItem: toListItem(config),
		SharedSecretSet:      config.SharedSecret != "",
		SharedSecretMasked:   maskSecret(config.SharedSecret),
		Revision:             config.Revision,
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

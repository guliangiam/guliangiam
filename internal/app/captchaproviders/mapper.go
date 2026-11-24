package captchaproviders

import (
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
)

func toListItem(provider *domain.CaptchaProvider) CaptchaProviderListItem {
	return CaptchaProviderListItem{
		ID:             provider.ID,
		OrganizationID: provider.OrganizationID,
		Name:           provider.Name,
		ProviderType:   provider.ProviderType,
		SiteKey:        provider.SiteKey,
		IsEnabled:      provider.IsEnabled,
		CreatedAt:      provider.CreatedAt,
		UpdatedAt:      provider.UpdatedAt,
	}
}

func toDetail(provider *domain.CaptchaProvider) CaptchaProviderDetail {
	return CaptchaProviderDetail{
		CaptchaProviderListItem: toListItem(provider),
		SecretKeySet:            provider.SecretKey != "",
		SecretKeyMasked:         maskSecret(provider.SecretKey),
		Config:                  redactConfig(provider.Config),
		Revision:                provider.Revision,
	}
}

// redactConfig masks secret-like config values so credentials carried in the
// JSONB config (Tencent CAM SecretId/SecretKey, Yidun secretId, Enterprise API
// key …) never leave the read API in plaintext (AGENTS §2.5).
func redactConfig(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	out := make(map[string]any, len(config))
	for k, v := range config {
		if s, ok := v.(string); ok && s != "" && isSecretConfigKey(k) {
			out[k] = maskSecret(s)
			continue
		}
		out[k] = v
	}
	return out
}

// mergeConfig applies an incoming config over the stored one, preserving
// secrets the client could only echo back: the read API redacts them, so a
// PATCH value equal to the masked sentinel (or empty) means "keep current".
func mergeConfig(existing, incoming map[string]any) map[string]any {
	out := make(map[string]any, len(incoming))
	for k, v := range incoming {
		s, isStr := v.(string)
		if isStr && isSecretConfigKey(k) {
			if cur, has := existing[k]; has {
				if cs, ok := cur.(string); ok && (s == "" || s == maskSecret(cs)) {
					out[k] = cs
					continue
				}
			}
			if s == "" {
				continue
			}
		}
		out[k] = v
	}
	return out
}

// isSecretConfigKey flags config keys whose values are credentials.
func isSecretConfigKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "secret") || strings.Contains(k, "password") ||
		strings.HasSuffix(k, "apikey") || strings.HasSuffix(k, "privatekey")
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

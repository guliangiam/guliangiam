package captchaproviders

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	"github.com/stretchr/testify/assert"
)

func TestToListItem(t *testing.T) {
	p := &domain.CaptchaProvider{
		ID:             "cp-1",
		OrganizationID: "org-1",
		Name:           "recaptcha",
		ProviderType:   "recaptcha",
		IsEnabled:      true,
	}
	item := toListItem(p)
	assert.Equal(t, "cp-1", item.ID)
	assert.Equal(t, "org-1", item.OrganizationID)
	assert.Equal(t, "recaptcha", item.Name)
	assert.Equal(t, "recaptcha", item.ProviderType)
	assert.True(t, item.IsEnabled)
}

func TestToDetail(t *testing.T) {
	now := time.Now()
	p := &domain.CaptchaProvider{
		ID:             "cp-1",
		OrganizationID: "org-1",
		Name:           "recaptcha",
		ProviderType:   "recaptcha",
		SecretKey:      "secret",
		SiteKey:        "site",
		IsEnabled:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	detail := toDetail(p)
	assert.Equal(t, "cp-1", detail.ID)
	assert.True(t, detail.SecretKeySet)
	assert.NotEmpty(t, detail.SecretKeyMasked)
	assert.NotEqual(t, "secret", detail.SecretKeyMasked)
	assert.Equal(t, "site", detail.SiteKey)
}

func TestToDetail_RedactsConfigSecrets(t *testing.T) {
	p := &domain.CaptchaProvider{
		ID: "cp-2", ProviderType: "tencent_captcha",
		Config: map[string]any{
			"camSecretId":  "AKIDzabcdefghij",
			"camSecretKey": "camsecretvalue99",
			"sceneId":      "scene-visible",
			"appName":      "my-app",
		},
	}
	detail := toDetail(p)
	assert.Equal(t, maskSecret("AKIDzabcdefghij"), detail.Config["camSecretId"])
	assert.Equal(t, maskSecret("camsecretvalue99"), detail.Config["camSecretKey"])
	// Non-secret values stay readable — SceneId/AppName are not credentials.
	assert.Equal(t, "scene-visible", detail.Config["sceneId"])
	assert.Equal(t, "my-app", detail.Config["appName"])
}

func TestMergeConfig_PreservesMaskedEchoAndEmptySecrets(t *testing.T) {
	existing := map[string]any{
		"camSecretKey": "topsecretvalue123",
		"secretId":     "sid-keep",
		"sceneId":      "scene-old",
	}

	// Echoing the masked sentinel (untouched field in the console form) keeps
	// the stored secret; a new plaintext value replaces it; empty means keep.
	merged := mergeConfig(existing, map[string]any{
		"camSecretKey": maskSecret("topsecretvalue123"),
		"secretId":     "",
		"sceneId":      "scene-new",
		"appName":      "app-added",
	})
	assert.Equal(t, "topsecretvalue123", merged["camSecretKey"])
	assert.Equal(t, "sid-keep", merged["secretId"])
	assert.Equal(t, "scene-new", merged["sceneId"])
	assert.Equal(t, "app-added", merged["appName"])

	// A genuinely new secret value overwrites.
	merged = mergeConfig(existing, map[string]any{"camSecretKey": "brand-new-secret"})
	assert.Equal(t, "brand-new-secret", merged["camSecretKey"])

	// Empty secret without a stored value is dropped, not stored as "".
	merged = mergeConfig(nil, map[string]any{"camSecretKey": ""})
	assert.NotContains(t, merged, "camSecretKey")
}

func TestIsSecretConfigKey(t *testing.T) {
	for _, k := range []string{"camSecretKey", "camSecretId", "secretKey", "secretId", "apiKey", "privateKey", "accessKeySecret", "appSecretKey", "DB_PASSWORD"} {
		assert.True(t, isSecretConfigKey(k), k)
	}
	for _, k := range []string{"sceneId", "appName", "projectId", "scoreThreshold", "action", "endpoint"} {
		assert.False(t, isSecretConfigKey(k), k)
	}
}

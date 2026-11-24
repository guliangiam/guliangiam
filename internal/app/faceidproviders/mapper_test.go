package faceidproviders

import (
	"encoding/json"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToListItem(t *testing.T) {
	now := time.Now()
	p := &domain.FaceIdProvider{
		ID: "fp-1", OrganizationID: "org-1", Name: "aliyun",
		ProviderType: "aliyun", AccessKeyID: "key123", Endpoint: "https://faceid.aliyun.com",
		IsEnabled: true, CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(p)
	assert.Equal(t, "fp-1", item.ID)
	assert.Equal(t, "aliyun", item.Name)
	assert.Equal(t, "key123", item.AccessKeyID)
}

func TestToDetail(t *testing.T) {
	p := &domain.FaceIdProvider{
		ID: "fp-1", OrganizationID: "org-1", Name: "aliyun",
		ProviderType: "aliyun", AccessKeySecret: "secret123",
		Config: map[string]any{"region": "cn-hangzhou"}, Revision: 1,
		SecretNeedsRotation: true,
	}
	detail := toDetail(p)
	assert.Equal(t, "fp-1", detail.ID)
	assert.True(t, detail.AccessKeySecretSet)
	assert.NotEmpty(t, detail.AccessKeySecretMasked)
	assert.NotEqual(t, "secret123", detail.AccessKeySecretMasked)
	assert.True(t, detail.AccessKeySecretNeedsRotation)
	assert.Equal(t, "cn-hangzhou", detail.Config["region"])
	assert.Equal(t, int64(1), detail.Revision)
}

func TestToDetailJSONNeverContainsPlaintextSecret(t *testing.T) {
	p := &domain.FaceIdProvider{
		ID: "fp-1", OrganizationID: "org-1", Name: "aliyun",
		ProviderType: "aliyun", AccessKeySecret: "secret123",
		Config: map[string]any{}, Revision: 1,
	}
	encoded, err := json.Marshal(toDetail(p))
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "secret123")
	assert.Contains(t, string(encoded), `"accessKeySecretSet":true`)
	assert.Contains(t, string(encoded), `"accessKeySecretNeedsRotation":false`)
}

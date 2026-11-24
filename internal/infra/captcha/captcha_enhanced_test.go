package captcha

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.NotNil(t, factory.providers)
}

func TestFactory_RegisterAndGetProvider(t *testing.T) {
	factory := NewFactory()
	store := NewMemoryStore(5 * time.Minute)
	provider := NewDefaultProvider(store)
	factory.Register("default", provider)

	got, err := factory.GetProvider("default")
	require.NoError(t, err)
	assert.Equal(t, provider, got)
}

func TestFactory_GetProvider_NotFound(t *testing.T) {
	factory := NewFactory()
	_, err := factory.GetProvider("nonexistent")
	assert.Error(t, err)
}

func TestNewProvider_Default(t *testing.T) {
	provider, err := NewProvider("default", "", "")
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestNewProvider_GeeTest(t *testing.T) {
	provider, err := NewProvider("geetest", "secret", "site")
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestNewProvider_HCaptcha(t *testing.T) {
	provider, err := NewProvider("hcaptcha", "secret", "")
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestNewProvider_ReCAPTCHA(t *testing.T) {
	provider, err := NewProvider("recaptcha", "secret", "")
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestNewProvider_Turnstile(t *testing.T) {
	provider, err := NewProvider("turnstile", "secret", "")
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestNewProvider_AliyunCaptcha_MapsAccessKeyPair(t *testing.T) {
	// OPEN-API-75 gap ①: the two-slot convenience mapping must route
	// siteKey→AccessKeyID and secretKey→AccessKeySecret; previously the secret
	// slot was dropped and secretKey was mis-mapped to AccessKeyID.
	provider, err := NewProvider("aliyun_captcha", "sk-secret", "ak-site")
	require.NoError(t, err)
	aliyun, ok := provider.(*AliyunCaptchaProvider)
	require.True(t, ok)
	assert.Equal(t, "ak-site", aliyun.AccessKeyID)
	assert.Equal(t, "sk-secret", aliyun.AccessKeySecret)
}

func TestNewProvider_TencentCaptcha(t *testing.T) {
	provider, err := NewProvider("tencent_captcha", "app-secret", "204099123")
	require.NoError(t, err)
	tencent, ok := provider.(*TencentCaptchaProvider)
	require.True(t, ok)
	assert.Equal(t, "204099123", tencent.CaptchaAppID)
	assert.Equal(t, "app-secret", tencent.AppSecretKey)
}

func TestNewProvider_UnsupportedType(t *testing.T) {
	_, err := NewProvider("unsupported", "", "")
	assert.Error(t, err)
}

func TestMemoryStore_SetGet(t *testing.T) {
	store := NewMemoryStore(5 * time.Minute)
	err := store.Set(context.Background(), "captcha-1", "ABC123")
	require.NoError(t, err)

	code, err := store.Get(context.Background(), "captcha-1")
	require.NoError(t, err)
	assert.Equal(t, "ABC123", code)
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	store := NewMemoryStore(5 * time.Minute)
	_, err := store.Get(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore(5 * time.Minute)
	require.NoError(t, store.Set(context.Background(), "captcha-1", "ABC123"))
	require.NoError(t, store.Delete(context.Background(), "captcha-1"))

	_, err := store.Get(context.Background(), "captcha-1")
	assert.Error(t, err)
}

func TestMemoryStore_Overwrite(t *testing.T) {
	store := NewMemoryStore(5 * time.Minute)
	require.NoError(t, store.Set(context.Background(), "captcha-1", "ABC123"))
	require.NoError(t, store.Set(context.Background(), "captcha-1", "XYZ789"))

	code, err := store.Get(context.Background(), "captcha-1")
	require.NoError(t, err)
	assert.Equal(t, "XYZ789", code)
}

func TestMemoryStore_Expired(t *testing.T) {
	store := NewMemoryStore(-time.Second)
	require.NoError(t, store.Set(context.Background(), "captcha-1", "ABC123"))

	_, err := store.Get(context.Background(), "captcha-1")
	assert.Error(t, err)
}

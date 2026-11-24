package idp

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/providers"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	if f == nil {
		t.Fatal("nil factory")
	}
}

func TestFactory_GetProvider_Nil(t *testing.T) {
	f := NewFactory()
	_, err := f.GetProvider(nil)
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestFactory_GetProvider_UnsupportedCategory(t *testing.T) {
	f := NewFactory()
	_, err := f.GetProvider(&providers.Provider{
		Category:     "unknown",
		ProviderType: "github",
	})
	if err == nil {
		t.Fatal("expected error for unsupported category")
	}
}

func TestFactory_GetProvider_OAuth(t *testing.T) {
	f := NewFactory()
	oauthTypes := []string{"github", "google", "wechat", "dingtalk", "lark", "gitlab", "facebook", "linkedin", "azuread", "custom", "alipay", "twitter", "slack", "discord"}
	for _, pt := range oauthTypes {
		t.Run(pt, func(t *testing.T) {
			p, err := f.GetProvider(&providers.Provider{
				Category:     "oauth",
				ProviderType: providers.ProviderType(pt),
			})
			if err != nil {
				t.Fatal(err)
			}
			if p == nil {
				t.Fatal("nil provider")
			}
		})
	}
}

func TestFactory_GetProvider_SAML(t *testing.T) {
	f := NewFactory()
	p, err := f.GetProvider(&providers.Provider{
		Category:     "saml",
		ProviderType: "saml",
		ProviderURL:  "https://sso.example.com/saml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("nil provider")
	}
}

func TestGenerateState(t *testing.T) {
	state, err := GenerateState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state) != 32 {
		t.Fatalf("expected 32-char hex, got len=%d", len(state))
	}
	state2, _ := GenerateState()
	if state == state2 {
		t.Fatal("two generated states should differ")
	}
}

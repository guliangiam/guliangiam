package payment

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
)

// verifyConfigTestCase is one input → want error (nil = should pass).
type verifyConfigTestCase struct {
	name    string
	channel string
	cfg     domain.ChannelConfig
	wantErr bool // true = expect error, false = expect nil
}

func TestChannelAdapter_VerifyConfig(t *testing.T) {
	goodAlipay := domain.ChannelConfig{
		AppID:      "2021000000000001",
		PrivateKey: "-----BEGIN PRIVATE KEY-----\nfake\n-----END PRIVATE KEY-----",
		NotifyURL:  "https://shop.example.com/api/v1/callbacks/alipay",
	}
	goodWechat := domain.ChannelConfig{
		AppID:      "wx1234567890abcdef",
		MerchantID: "1600000001",
		PrivateKey: "-----BEGIN PRIVATE KEY-----\nfake\n-----END PRIVATE KEY-----",
		NotifyURL:  "https://shop.example.com/api/v1/callbacks/wechat",
	}
	goodStripe := domain.ChannelConfig{
		WebhookSecret: "whsec_abc123def456",
	}
	goodPayPal := domain.ChannelConfig{
		AppID:      "AXXXXX-CLIENT-ID",
		PrivateKey: "CLIENT-SECRET-XXXXX",
	}
	goodCustom := domain.ChannelConfig{
		Extra: map[string]any{"token": "xyz"},
	}

	cases := []verifyConfigTestCase{
		// ── Alipay ────────────────────────────────────────────────
		{name: "alipay: good", channel: "alipay", cfg: goodAlipay, wantErr: false},
		{name: "alipay: missing AppID", channel: "alipay", cfg: mustCopy(goodAlipay, func(c *domain.ChannelConfig) { c.AppID = "" }), wantErr: true},
		{name: "alipay: missing PrivateKey", channel: "alipay", cfg: mustCopy(goodAlipay, func(c *domain.ChannelConfig) { c.PrivateKey = "" }), wantErr: true},
		{name: "alipay: missing NotifyURL", channel: "alipay", cfg: mustCopy(goodAlipay, func(c *domain.ChannelConfig) { c.NotifyURL = "" }), wantErr: true},
		{name: "alipay: http:// notify_url rejected", channel: "alipay",
			cfg: mustCopy(goodAlipay, func(c *domain.ChannelConfig) { c.NotifyURL = "http://shop.example.com/cb" }), wantErr: true},
		{name: "alipay: malformed notify_url rejected", channel: "alipay",
			cfg: mustCopy(goodAlipay, func(c *domain.ChannelConfig) { c.NotifyURL = "not-a-url" }), wantErr: true},

		// ── WeChat ────────────────────────────────────────────────
		{name: "wechat_pay: good", channel: "wechat_pay", cfg: goodWechat, wantErr: false},
		{name: "wechat_pay: alias wechat accepted", channel: "wechat", cfg: goodWechat, wantErr: false},
		{name: "wechat_pay: missing MerchantID", channel: "wechat_pay",
			cfg: mustCopy(goodWechat, func(c *domain.ChannelConfig) { c.MerchantID = "" }), wantErr: true},
		{name: "wechat_pay: missing AppID + PrivateKey", channel: "wechat_pay",
			cfg: mustCopy(goodWechat, func(c *domain.ChannelConfig) { c.AppID = ""; c.PrivateKey = "" }), wantErr: true},
		{name: "wechat_pay: http notify_url rejected", channel: "wechat_pay",
			cfg: mustCopy(goodWechat, func(c *domain.ChannelConfig) { c.NotifyURL = "http://cb.example" }), wantErr: true},

		// ── Stripe ─────────────────────────────────────────────────
		{name: "stripe: good whsec_", channel: "stripe", cfg: goodStripe, wantErr: false},
		{name: "stripe: missing secret", channel: "stripe",
			cfg: mustCopy(goodStripe, func(c *domain.ChannelConfig) { c.WebhookSecret = "" }), wantErr: true},
		{name: "stripe: wrong prefix", channel: "stripe",
			cfg: mustCopy(goodStripe, func(c *domain.ChannelConfig) { c.WebhookSecret = "sk_live_xxx" }), wantErr: true},

		// ── PayPal ────────────────────────────────────────────────
		{name: "paypal: good", channel: "paypal", cfg: goodPayPal, wantErr: false},
		{name: "paypal: missing AppID", channel: "paypal",
			cfg: mustCopy(goodPayPal, func(c *domain.ChannelConfig) { c.AppID = "" }), wantErr: true},

		// ── Braintree ──────────────────────────────────────────────
		{name: "braintree: good", channel: "braintree", cfg: domain.ChannelConfig{
			AppID:      "merchant123",
			PrivateKey: "private_key_abc",
		}, wantErr: false},
		{name: "braintree: missing both", channel: "braintree", cfg: domain.ChannelConfig{}, wantErr: true},

		// ── Sandbox ────────────────────────────────────────────────
		{name: "sandbox: empty config passes", channel: "sandbox", cfg: domain.ChannelConfig{}, wantErr: false},
		{name: "sandbox: any junk passes", channel: "sandbox", cfg: domain.ChannelConfig{PrivateKey: "whatever"}, wantErr: false},

		// ── Custom ─────────────────────────────────────────────────
		{name: "custom: good with Extra", channel: "custom", cfg: goodCustom, wantErr: false},
		{name: "custom: nil Extra rejected", channel: "custom", cfg: domain.ChannelConfig{}, wantErr: true},

		// ── Unknown ────────────────────────────────────────────────
		{name: "unknown channel rejected", channel: "brand_new_channel", cfg: goodAlipay, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := NewChannelAdapter(tc.channel, nil)
			err := a.VerifyConfig(context.Background(), tc.cfg)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected nil but got error: %v", err)
			}
		})
	}
}

// mustCopy shallow-copies a ChannelConfig and applies fn. Pure helper.
func mustCopy(src domain.ChannelConfig, fn func(*domain.ChannelConfig)) domain.ChannelConfig {
	cp := src
	fn(&cp)
	return cp
}

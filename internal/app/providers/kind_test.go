package providers

import (
	"strings"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

func TestCanonicalIdentityKind(t *testing.T) {
	tests := []struct {
		providerType string
		want         domain.Kind
	}{
		// 闭集内精确匹配
		{"GitHub", "github"},
		{"google", "google"},
		{"LinkedIn", "linkedin"},
		{"Custom", "custom"},
		{"SMTP", "smtp"},
		{"FaceId", "faceid"},
		{"MetaMask", "metamask"},
		// 别名：厂商名与 kind 命名不同
		{"AzureADB2C", "azure_ad_b2c"},
		{"WeCom", "wecom_internal"},
		{"Infoflow", "infoflow_internal"},
		{"SAML 2.0", "saml"},
		{"reCAPTCHAv3", "recaptcha_v3"},
		{"TencentCaptcha", "tencent_captcha"},
		{"AzureACSSMS", "azure_acs_sms"},
		{"WeChatScan", "wechat_scan"},
		{"DingTalkScan", "dingtalk_scan"},
		// 驼峰/分隔符归一
		{"AzureAD", "azure_ad"},
		{"AlibabaSMS", "alibaba_sms"},
		{"TwilioSMS", "twilio_sms"},
		{"UCloudSMS", "ucloud_sms"},
		{"AWS S3", "aws_s3"},
		{"CustomHTTP", "custom_http"},
		{"AliyunFaceBody", "aliyun_face_body"},
		// C2 messaging vendors
		{"Twilio Verify", "twilio_verify"},
		{"Alibaba Cloud PNVS SMS", "alibaba_pnvs"},
		{"SmsBao SMS", "smsbao"},
		{"Msg91 SMS", "msg91"},
		{"SUBMAIL Email", "submail_email"},
		{"Mailtrap", "mailtrap_email"},
		{"Custom HTTP SMS", "custom_http_sms"},
		// SMS family batch (domestic + international)
		{"UnismsSMS", "unisms_sms"},
		{"YunpianSMS", "yunpian_sms"},
		{"JDCloudSMS", "jdcloud_sms"},
		{"VonageSMS", "vonage_sms"},
		{"PlivoSMS", "plivo_sms"},
		{"TelnyxSMS", "telnyx_sms"},
		{"SinchSMS", "sinch_sms"},
		{"BirdSMS", "bird_sms"},
		{"ClickatellSMS", "clickatell_sms"},
		{"AfricastalkingSMS", "africastalking_sms"},
		// Email family batch (domestic + international)
		{"DirectMail", "directmail"},
		{"TencentSES", "tencent_ses"},
		{"SendCloud", "sendcloud"},
		{"Mailgun", "mailgun"},
		{"AmazonSES", "amazon_ses"},
		{"Postmark", "postmark"},
		{"MailerSend", "mailersend"},
		{"Brevo", "brevo"},
		{"ZohoMail", "zoho_mail"},
		// 闭集外仍兜底 custom（厂商信息保留在 provider_type）
		{"UnknownVendor", "custom"},
		{"", "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.providerType, func(t *testing.T) {
			if got := canonicalIdentityKind(tt.providerType); got != tt.want {
				t.Fatalf("canonicalIdentityKind(%q) = %q, want %q", tt.providerType, got, tt.want)
			}
		})
	}
}

// 前端「类型」下拉的全部取值都必须映射到闭集内的 kind（custom 除外，它是兜底），
// 否则创建请求会被 ck_providers_kind 或 strategy 拒绝。
func TestCanonicalIdentityKindCoversConsoleProviderTypes(t *testing.T) {
	consoleProviderTypes := []string{
		// OAuth
		"GitHub", "Google", "AzureAD", "AzureADB2C", "WeChat", "DingTalk", "Lark", "Facebook",
		"Twitter", "LinkedIn", "GitLab", "Slack", "Discord", "Apple", "Okta", "ADFS", "Alipay",
		"Baidu", "Douyin", "QQ", "Weibo", "Steam", "Telegram", "Amazon", "Bilibili", "Gitee",
		"Infoflow", "Kwai", "WeCom", "Custom",
		// SAML / OIDC
		"SAML 2.0", "Keycloak", "OIDC",
		// Email
		"SMTP", "SendGrid", "AzureACS", "Resend", "CustomHTTP",
		// SMS
		"TwilioSMS", "AlibabaSMS", "TencentSMS", "VolcengineSMS", "HuaweiSMS", "AmazonSNS",
		"AzureACSSMS", "Infobip", "UCloudSMS", "EmaySMS", "BaiduSMS", "Submail", "MailtrapSMS",
		"OsonSMS", "MockSMS",
		// Storage
		"AWS S3", "AzureBlob", "GCS", "MinIO", "AliOSS", "TencentCOS", "Qiniu", "CUCloud",
		"HuaweiOBS", "Synology", "Local",
		// Captcha
		"Default", "reCAPTCHA", "reCAPTCHAv3", "hCaptcha", "Turnstile", "GeeTest", "AliyunCaptcha",
		"TencentCaptcha",
		// MFA
		"TOTP", "RADIUS", "Push",
		// FaceId / Web3 / Scan / IDV / Log
		"AliyunFaceBody", "WeChatScan", "DingTalkScan", "Jumio", "AliyunIdv", "Elasticsearch", "Loki",
	}
	for _, providerType := range consoleProviderTypes {
		kind := canonicalIdentityKind(providerType)
		if kind == domain.Kind(domain.ProviderCustom) {
			// 兜底值本身就是 Custom：厂商写法为 Custom 时命中预期，不算退化
			if providerType == "Custom" {
				continue
			}
			t.Fatalf("console provider type %q still falls back to custom", providerType)
		}
		if !identityClosedKinds[kind] {
			t.Fatalf("console provider type %q maps to %q outside the closed kind set", providerType, kind)
		}
	}
}

func TestCanonicalIdentityKindMapsThirdPartyCatalogToGoth(t *testing.T) {
	catalog := append(append([]string{}, gothProviderTypes...), nativeThirdPartyOAuthTypes...)
	for _, providerType := range catalog {
		for _, spelling := range []string{providerType, strings.ToUpper(providerType)} {
			if got := canonicalIdentityKind(spelling); got != domain.Kind(domain.ProviderGoth) {
				t.Fatalf("canonicalIdentityKind(%q) = %q, want goth", spelling, got)
			}
		}
	}
}

func TestGothProviderCatalogHasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range gothProviderTypes {
		if seen[name] {
			t.Fatalf("duplicate goth provider type: %s", name)
		}
		seen[name] = true
	}
}

// Types with a dedicated kind keep it; the goth catalog must not hijack them.
func TestDedicatedKindsAreNotRoutedToGoth(t *testing.T) {
	for _, providerType := range []string{"GitHub", "Google", "Steam", "Amazon", "Slack", "Discord", "WeChat", "SAML 2.0", "OIDC", "Custom"} {
		if got := canonicalIdentityKind(providerType); got == domain.Kind(domain.ProviderGoth) {
			t.Fatalf("canonicalIdentityKind(%q) unexpectedly routed to goth", providerType)
		}
	}
}

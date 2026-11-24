package providers

import (
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TestProvider_Validate_CategoryMatrix is the regression matrix for the
// kind -> category closure. Before the domain fix every non-legacy kind was
// resolved by GetProviderCategory's default branch to OAuth, so a provider
// carrying its real category (SMS/Email/SAML/MFA/Scan/IDVerification/Log)
// failed activation with "category does not match type (expected OAuth)".
func TestProvider_Validate_CategoryMatrix(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  bool
		contains string
	}{
		{
			name: "sms alibaba_sms matches",
			provider: Provider{
				Name: "aliyun-sms", Category: ProviderCategory("SMS"),
				ProviderType: ProviderType("alibaba_sms"), Kind: Kind("alibaba_sms"),
				Admin: "ak", Password: "sk", Sender: "sign", Template: "SMS_1",
			},
		},
		{
			name: "email sendgrid matches",
			provider: Provider{
				Name: "sendgrid", Category: ProviderCategory("Email"),
				ProviderType: ProviderType("sendgrid"), Kind: Kind("sendgrid"),
			},
		},
		{
			name: "saml matches",
			provider: Provider{
				Name: "corp-saml", Category: ProviderCategory("SAML"),
				ProviderType: ProviderType("saml"), Kind: Kind("saml"),
				IdPEntityId: "https://idp.example.com/entity",
			},
		},
		{
			name: "mfa totp matches",
			provider: Provider{
				Name: "totp", Category: ProviderCategory("MFA"),
				ProviderType: ProviderType("totp"), Kind: Kind("totp"),
			},
		},
		{
			name: "scan wechat_scan matches",
			provider: Provider{
				Name: "wechat-scan", Category: ProviderCategory("Scan"),
				ProviderType: ProviderType("wechat_scan"), Kind: Kind("wechat_scan"),
			},
		},
		{
			name: "idv jumio matches",
			provider: Provider{
				Name: "jumio", Category: ProviderCategory("IDVerification"),
				ProviderType: ProviderType("jumio"), Kind: Kind("jumio"),
			},
		},
		{
			name: "log elasticsearch matches",
			provider: Provider{
				Name: "es", Category: ProviderCategory("Log"),
				ProviderType: ProviderType("elasticsearch"), Kind: Kind("elasticsearch"),
			},
		},
		{
			name: "audit syslog matches",
			provider: Provider{
				Name: "syslog", Category: ProviderCategory("Audit"),
				ProviderType: ProviderType("syslog"), Kind: Kind("syslog"),
				Host: "log.example.com", Port: 514,
			},
		},
		{
			name: "sms kind rejects email category",
			provider: Provider{
				Name: "aliyun-sms", Category: ProviderCategory("Email"),
				ProviderType: ProviderType("alibaba_sms"), Kind: Kind("alibaba_sms"),
			},
			wantErr: true, contains: "does not match",
		},
		{
			name: "email kind rejects sms category",
			provider: Provider{
				Name: "sendgrid", Category: ProviderCategory("SMS"),
				ProviderType: ProviderType("sendgrid"), Kind: Kind("sendgrid"),
			},
			wantErr: true, contains: "does not match",
		},
		{
			name: "saml kind rejects oauth category",
			provider: Provider{
				Name: "corp-saml", Category: ProviderCategory("OAuth"),
				ProviderType: ProviderType("saml"), Kind: Kind("saml"),
				IdPEntityId: "https://idp.example.com/entity",
			},
			wantErr: true, contains: "does not match",
		},
		{
			name: "saml still requires idpEntityId",
			provider: Provider{
				Name: "corp-saml", Category: ProviderCategory("SAML"),
				ProviderType: ProviderType("saml"), Kind: Kind("saml"),
			},
			wantErr: true, contains: "idpEntityId is required",
		},
		{
			name: "mfa kind rejects log category",
			provider: Provider{
				Name: "totp", Category: ProviderCategory("Log"),
				ProviderType: ProviderType("totp"), Kind: Kind("totp"),
			},
			wantErr: true, contains: "does not match",
		},
		{
			name: "scan kind rejects audit category",
			provider: Provider{
				Name: "wechat-scan", Category: ProviderCategory("Audit"),
				ProviderType: ProviderType("wechat_scan"), Kind: Kind("wechat_scan"),
			},
			wantErr: true, contains: "does not match",
		},
		{
			name: "idv kind rejects scan category",
			provider: Provider{
				Name: "jumio", Category: ProviderCategory("Scan"),
				ProviderType: ProviderType("jumio"), Kind: Kind("jumio"),
			},
			wantErr: true, contains: "does not match",
		},
		{
			name: "log kind rejects audit category",
			provider: Provider{
				Name: "es", Category: ProviderCategory("Audit"),
				ProviderType: ProviderType("elasticsearch"), Kind: Kind("elasticsearch"),
			},
			wantErr: true, contains: "does not match",
		},
		{
			name: "unknown kind is rejected instead of defaulting to OAuth",
			provider: Provider{
				Name: "unknown", Category: ProviderCategory("OAuth"),
				ProviderType: ProviderType("not_registered"), Kind: Kind("not_registered"),
			},
			wantErr: true, contains: "unknown provider kind",
		},
		{
			name: "unknown category is rejected",
			provider: Provider{
				Name: "bogus", Category: ProviderCategory("Banana"),
				ProviderType: ProviderType("github"), Kind: Kind("github"),
			},
			wantErr: true, contains: "unknown provider category",
		},
		{
			name: "oauth control still matches",
			provider: Provider{
				Name: "github", Category: CategoryOAuth,
				ProviderType: ProviderGitHub, Kind: Kind("github"),
				ClientID: "id", AuthUrl: "https://auth", TokenUrl: "https://token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.contains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.contains) {
					t.Fatalf("Validate() error = %v, want it to contain %q", err, tt.contains)
				}
			}
			if err != nil && tt.wantErr {
				if appErr := apperrors.FromError(err); appErr.Code != apperrors.CodeUnprocessableEntity && tt.contains != "idpEntityId is required" {
					t.Fatalf("Validate() code = %v, want 422", appErr.Code)
				}
			}
		})
	}
}

func TestGetProviderCategory_KindClosure(t *testing.T) {
	tests := []struct {
		kind Kind
		want ProviderCategory
	}{
		{Kind("alibaba_sms"), CategorySMS},
		{Kind("sendgrid"), CategoryEmail},
		{Kind("saml"), CategorySAML},
		{Kind("totp"), CategoryMFA},
		{Kind("wechat_scan"), CategoryScan},
		{Kind("jumio"), CategoryIDVerification},
		{Kind("elasticsearch"), CategoryLog},
		{Kind("syslog"), CategoryAudit},
		{Kind("github"), CategoryOAuth},
		{Kind("aws_s3"), CategoryStorage},
		{Kind("ali_oss"), CategoryStorage},
		{Kind("cloudflare_r2"), CategoryStorage},
		{Kind("backblaze_b2"), CategoryStorage},
		{Kind("digitalocean_spaces"), CategoryStorage},
		{Kind("recaptcha"), CategoryCaptcha},
		{Kind("faceid"), CategoryFaceID},
		{Kind("loki"), CategoryLog},
	}

	for _, tt := range tests {
		got, err := GetProviderCategory(tt.kind)
		if err != nil {
			t.Fatalf("GetProviderCategory(%q) error = %v", tt.kind, err)
		}
		if got != tt.want {
			t.Fatalf("GetProviderCategory(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}

	if _, err := GetProviderCategory(Kind("not_registered")); err == nil {
		t.Fatal("GetProviderCategory(unknown) should return an error, not default to OAuth")
	}
}

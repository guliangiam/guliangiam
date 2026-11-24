package redact

import (
	"reflect"
	"testing"
)

func TestIsSensitiveField(t *testing.T) {
	sensitive := []string{
		"password",
		"Password",
		"PASSWORD_HASH",
		"password-hash",
		"passwordSalt",
		"access_token",
		"accessToken",
		"access_secret",
		"refresh_token",
		"client_secret",
		"clientSecret",
		"bindPassword",
		"totpSecret",
		"recoveryCodes",
		"mfaPushReceiver",
		"masterPassword",
		"kerberosKeytab",
		"privateKey",
		"token",
		"tokens",
		"idCard",
		"passportNo",
		"driverLicenseNo",
		"cookie",
		"set-cookie",
		"Set-Cookie",
		"Authorization",
		"X-Api-Key",
		"credential",
		"credentials",
		"session_secret",
		"secret",
		// Every token run of the name is examined, so a prefixed or nested
		// spelling resolves to the canonical field instead of needing its own
		// entry.
		"http.request.header.authorization",
		"db.user.password",
	}
	for _, key := range sensitive {
		t.Run(key, func(t *testing.T) {
			if !IsSensitiveField(key) {
				t.Errorf("IsSensitiveField(%q) = false, want true", key)
			}
		})
	}
}

func TestIsSensitiveField_LeavesOrdinaryFieldsAlone(t *testing.T) {
	// The list is shared by every redaction boundary, so a false positive hides
	// real data from operators. These are the field names the observability read
	// surface actually carries.
	ordinary := []string{
		"",
		"   ",
		"username",
		"email",
		"user_agent",
		"http.method",
		"http.status_code",
		"db.statement",
		"service.name",
		"resourceSpanCount",
		"spanId",
		"traceId",
		"secretary",
		"tokenizer",
		"accountSecretary",
		"sessionId",
		"connectionString",
	}
	for _, key := range ordinary {
		t.Run(key, func(t *testing.T) {
			if IsSensitiveField(key) {
				t.Errorf("IsSensitiveField(%q) = true, want false", key)
			}
		})
	}
}

func TestIsSensitiveField_RedactsCredentialsWithASecondWord(t *testing.T) {
	// These read like ordinary fields but carry a credential, so the boundary
	// must close on the credential token rather than on the whole name.
	for _, key := range []string{"authorization_code", "cookieConsent", "apiKeyValue"} {
		t.Run(key, func(t *testing.T) {
			if !IsSensitiveField(key) {
				t.Errorf("IsSensitiveField(%q) = false, want true", key)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{"accessToken", []string{"access", "token"}},
		{"APIToken", []string{"api", "token"}},
		{"PASSWORD_HASH", []string{"password", "hash"}},
		{"X-Api-Key", []string{"x", "api", "key"}},
		{"http.request.header.cookie", []string{"http", "request", "header", "cookie"}},
		{"spanID", []string{"span", "id"}},
		{"", []string{}},
		{"---", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenize(tt.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tokenize(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestFields_ReturnsACopy(t *testing.T) {
	fields := Fields()
	if len(fields) == 0 {
		t.Fatal("Fields() returned no entries")
	}

	fields[0] = "mutated"

	if Fields()[0] == "mutated" {
		t.Fatal("Fields() returned the backing slice; callers can corrupt the rule")
	}
}

package organizations

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

// Sentinels planted on every credential-bearing organization column. They are
// distinct per column so a read projection that echoes any of them can be
// attributed to the exact field that leaked.
const (
	sentinelPasswordSalt     = "SENTINEL-password-salt"
	sentinelObfuscatorKey    = "SENTINEL-password-obfuscator-key"
	sentinelMasterPassword   = "SENTINEL-master-password"
	sentinelVerificationCode = "SENTINEL-master-verification-code"
	sentinelDefaultPassword  = "SENTINEL-default-password"
	sentinelKerberosKeytab   = "SENTINEL-kerberos-keytab"
)

// plaintextCredentialJSONKeys are the json tags the management read path used to
// emit. Their absence is part of the contract, not just the values' absence:
// re-adding a plaintext field would otherwise pass a value-only assertion as
// soon as the stored secret is empty.
var plaintextCredentialJSONKeys = []string{
	`"passwordSalt"`,
	`"passwordObfuscatorKey"`,
	`"masterPassword"`,
	`"masterVerificationCode"`,
	`"defaultPassword"`,
	`"kerberosKeytab"`,
}

func credentialFixture() *orgdomain.Organization {
	return &orgdomain.Organization{
		ID:                     "org-cred-1",
		Slug:                   "acme",
		DisplayName:            "Acme",
		Status:                 "active",
		PasswordSalt:           sentinelPasswordSalt,
		PasswordObfuscatorKey:  sentinelObfuscatorKey,
		MasterPassword:         sentinelMasterPassword,
		MasterVerificationCode: sentinelVerificationCode,
		DefaultPassword:        sentinelDefaultPassword,
		KerberosKeytab:         sentinelKerberosKeytab,
	}
}

type credentialPair struct {
	column   string
	sentinel string
	set      bool
	masked   string
}

func credentialPairs(detail OrganizationDetail) []credentialPair {
	return []credentialPair{
		{"passwordSalt", sentinelPasswordSalt, detail.PasswordSaltSet, detail.PasswordSaltMasked},
		{"passwordObfuscatorKey", sentinelObfuscatorKey, detail.PasswordObfuscatorKeySet, detail.PasswordObfuscatorKeyMasked},
		{"masterPassword", sentinelMasterPassword, detail.MasterPasswordSet, detail.MasterPasswordMasked},
		{"masterVerificationCode", sentinelVerificationCode, detail.MasterVerificationCodeSet, detail.MasterVerificationCodeMasked},
		{"defaultPassword", sentinelDefaultPassword, detail.DefaultPasswordSet, detail.DefaultPasswordMasked},
		{"kerberosKeytab", sentinelKerberosKeytab, detail.KerberosKeytabSet, detail.KerberosKeytabMasked},
	}
}

// TestToDetail_MasksCredentialColumns is the P0 regression for the management
// read path: GET /organizations/{id} (and the by-slug variant) serves
// OrganizationDetail, so a plaintext field there leaks the stored secret on
// every admin page load.
func TestToDetail_MasksCredentialColumns(t *testing.T) {
	detail := toDetail(credentialFixture())
	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("json.Marshal(detail) error = %v", err)
	}
	body := string(encoded)

	for _, pair := range credentialPairs(detail) {
		if strings.Contains(body, pair.sentinel) {
			t.Errorf("marshalled detail still contains the %s plaintext", pair.column)
		}
	}
	for _, key := range plaintextCredentialJSONKeys {
		if strings.Contains(body, key) {
			t.Errorf("marshalled detail still exposes the plaintext json key %s", key)
		}
	}

	for _, pair := range credentialPairs(detail) {
		if !pair.set {
			t.Errorf("%sSet = false, want true for a non-empty column", pair.column)
		}
		if pair.masked == "" {
			t.Errorf("%sMasked is empty, want a masked display value", pair.column)
		}
		if strings.Contains(pair.masked, pair.sentinel) {
			t.Errorf("%sMasked contains the plaintext", pair.column)
		}
		// Convention (certs/providers): first two + "***" + last two.
		if len(pair.sentinel) >= 6 {
			want := pair.sentinel[:2] + "***" + pair.sentinel[len(pair.sentinel)-2:]
			if pair.masked != want {
				t.Errorf("%sMasked = %q, want %q", pair.column, pair.masked, want)
			}
		}
	}
}

// TestToDetail_EmptyCredentialsStayUnset pins the empty-value half of the
// contract: no "******" placeholder for an unset column, and Set=false so the
// console can distinguish "unset" from "set but short".
func TestToDetail_EmptyCredentialsStayUnset(t *testing.T) {
	detail := toDetail(&orgdomain.Organization{
		ID: "org-empty-1", Slug: "empty", DisplayName: "Empty", Status: "active",
	})

	pairs := []struct {
		column string
		set    bool
		masked string
	}{
		{"passwordSalt", detail.PasswordSaltSet, detail.PasswordSaltMasked},
		{"passwordObfuscatorKey", detail.PasswordObfuscatorKeySet, detail.PasswordObfuscatorKeyMasked},
		{"masterPassword", detail.MasterPasswordSet, detail.MasterPasswordMasked},
		{"masterVerificationCode", detail.MasterVerificationCodeSet, detail.MasterVerificationCodeMasked},
		{"defaultPassword", detail.DefaultPasswordSet, detail.DefaultPasswordMasked},
		{"kerberosKeytab", detail.KerberosKeytabSet, detail.KerberosKeytabMasked},
	}
	for _, pair := range pairs {
		if pair.set {
			t.Errorf("%sSet = true, want false for an empty column", pair.column)
		}
		if pair.masked != "" {
			t.Errorf("%sMasked = %q, want empty for an empty column", pair.column, pair.masked)
		}
	}
}

func TestMaskCredential(t *testing.T) {
	cases := []struct {
		name   string
		secret string
		want   string
	}{
		{"empty", "", ""},
		{"short", "abcde", "******"},
		{"boundary", "abcdef", "ab***ef"},
		{"long", "SENTINEL-value", "SE***ue"},
		{"multibyte", "口令口令口令", "口令***口令"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := maskCredential(testCase.secret); got != testCase.want {
				t.Errorf("maskCredential(%q) = %q, want %q", testCase.secret, got, testCase.want)
			}
		})
	}
}

// TestPatchOrganization_NilCredentialFieldsKeepCurrent is the write-path
// counterpart of the masking fix: the console now omits untouched credential
// keys, which the transport decodes to nil. nil must mean "keep the stored
// value" — if it meant "clear", every org save would wipe the credentials the
// operator cannot see.
func TestPatchOrganization_NilCredentialFieldsKeepCurrent(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	created, err := svc.CreateOrganization(ctx, CreateOrganizationCommand{
		Slug:                   "cred-org",
		DisplayName:            "Cred Org",
		Status:                 "active",
		PasswordSalt:           sentinelPasswordSalt,
		PasswordObfuscatorKey:  sentinelObfuscatorKey,
		MasterPassword:         sentinelMasterPassword,
		MasterVerificationCode: sentinelVerificationCode,
		DefaultPassword:        sentinelDefaultPassword,
	})
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}

	// KerberosKeytab has no create-path input; plant it directly on the stored
	// aggregate the way a Kerberos sync would.
	stored, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("repo.GetByID() error = %v", err)
	}
	stored.KerberosKeytab = sentinelKerberosKeytab

	renamed := "Cred Org Renamed"
	if err := svc.PatchOrganization(ctx, PatchOrganizationCommand{
		ID:          created.ID,
		DisplayName: &renamed,
		Revision:    stored.Revision,
	}); err != nil {
		t.Fatalf("PatchOrganization() error = %v", err)
	}

	patched, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("repo.GetByID() error = %v", err)
	}
	if patched.PasswordSalt != sentinelPasswordSalt {
		t.Errorf("PasswordSalt = %q, want the stored value to survive a nil patch", patched.PasswordSalt)
	}
	if patched.PasswordObfuscatorKey != sentinelObfuscatorKey {
		t.Errorf("PasswordObfuscatorKey = %q, want the stored value to survive a nil patch", patched.PasswordObfuscatorKey)
	}
	if patched.MasterPassword != sentinelMasterPassword {
		t.Errorf("MasterPassword = %q, want the stored value to survive a nil patch", patched.MasterPassword)
	}
	if patched.MasterVerificationCode != sentinelVerificationCode {
		t.Errorf("MasterVerificationCode = %q, want the stored value to survive a nil patch", patched.MasterVerificationCode)
	}
	if patched.DefaultPassword != sentinelDefaultPassword {
		t.Errorf("DefaultPassword = %q, want the stored value to survive a nil patch", patched.DefaultPassword)
	}
	if patched.KerberosKeytab != sentinelKerberosKeytab {
		t.Errorf("KerberosKeytab = %q, want the stored value to survive a nil patch", patched.KerberosKeytab)
	}
	if patched.DisplayName != renamed {
		t.Errorf("DisplayName = %q, want %q", patched.DisplayName, renamed)
	}
}

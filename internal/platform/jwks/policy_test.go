package jwks

import (
	"strings"
	"testing"
)

func TestResolveEphemeralSigningPolicy(t *testing.T) {
	cases := []struct {
		name           string
		production     bool
		keyRingPath    string
		allowEphemeral string
		wantAllowed    bool
		wantEscaped    bool
		wantErr        string
	}{
		{
			name:       "production without key ring fails closed",
			production: true,
			wantErr:    "JWT_SIGNING_KEYS_FILE",
		},
		{
			name:           "production with blank key ring fails closed",
			production:     true,
			keyRingPath:    "   ",
			allowEphemeral: "",
			wantErr:        "JWT_SIGNING_KEYS_FILE",
		},
		{
			name:           "production escape hatch true",
			production:     true,
			allowEphemeral: "true",
			wantAllowed:    true,
			wantEscaped:    true,
		},
		{
			name:           "production escape hatch on",
			production:     true,
			allowEphemeral: " ON ",
			wantAllowed:    true,
			wantEscaped:    true,
		},
		{
			name:           "production escape hatch one",
			production:     true,
			allowEphemeral: "1",
			wantAllowed:    true,
			wantEscaped:    true,
		},
		{
			name:           "production escape hatch yes",
			production:     true,
			allowEphemeral: "yes",
			wantAllowed:    true,
			wantEscaped:    true,
		},
		{
			name:           "production escape hatch false fails closed",
			production:     true,
			allowEphemeral: "false",
			wantErr:        "JWT_SIGNING_KEYS_FILE",
		},
		{
			name:        "development keeps ephemeral keys",
			production:  false,
			wantAllowed: true,
		},
		{
			name:           "development ignores escape hatch",
			production:     false,
			allowEphemeral: "false",
			wantAllowed:    true,
		},
		{
			name:           "key ring wins over production escape hatch",
			production:     true,
			keyRingPath:    "keys.json",
			allowEphemeral: "true",
			wantAllowed:    false,
			wantEscaped:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := ResolveEphemeralSigningPolicy(tc.production, tc.keyRingPath, tc.allowEphemeral)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("ResolveEphemeralSigningPolicy() error = nil, want %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveEphemeralSigningPolicy() unexpected error = %v", err)
			}
			if policy.Allowed != tc.wantAllowed || policy.Escaped != tc.wantEscaped {
				t.Fatalf("policy = %+v, want allowed=%v escaped=%v", policy, tc.wantAllowed, tc.wantEscaped)
			}
		})
	}
}

func TestResolveEphemeralSigningPolicyErrorNamesBothControls(t *testing.T) {
	_, err := ResolveEphemeralSigningPolicy(true, "", "")
	if err == nil {
		t.Fatal("expected production without a key ring to fail closed")
	}
	for _, want := range []string{"JWT_SIGNING_KEYS_FILE", "ALLOW_EPHEMERAL_SIGNING_KEYS", "replicas", "restart"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not explain %q", err, want)
		}
	}
}

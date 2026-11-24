package ldap

import (
	"testing"

	domainldaps "github.com/guliangiam/guliangiam/internal/domain/ldaps"
)

// TestSyncerUseTLS pins the TLS decision for the sync/preview path: the
// operator's explicit UseTLS (ldaps.enable_ssl) wins, and well-known-port
// inference is only the fallback for legacy configs without the flag.
func TestSyncerUseTLS(t *testing.T) {
	cases := []struct {
		name   string
		useTLS bool
		port   int
		want   bool
	}{
		{"explicit ssl on plaintext port", true, 389, true},
		{"explicit ssl on custom port", true, 10636, true},
		{"no flag, ldaps well-known port", false, 636, true},
		{"no flag, global catalog ssl port", false, 3269, true},
		{"no flag, plaintext port", false, 389, false},
		{"no flag, custom plaintext port", false, 10389, false},
		{"no flag, non-well-known port stays plaintext", false, 6360, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &domainldaps.SyncerConfig{UseTLS: tc.useTLS, Port: tc.port}
			if got := syncerUseTLS(cfg); got != tc.want {
				t.Errorf("syncerUseTLS(UseTLS=%v, Port=%d) = %v, want %v", tc.useTLS, tc.port, got, tc.want)
			}
		})
	}
}

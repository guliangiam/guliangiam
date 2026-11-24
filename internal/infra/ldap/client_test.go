package ldap

import (
	"testing"
	"time"
)

func TestLDAPURL(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		port   int
		useTLS bool
		want   string
	}{
		{"plain", "ldap.example.com", 389, false, "ldap://ldap.example.com:389"},
		{"tls", "ldap.example.com", 636, true, "ldaps://ldap.example.com:636"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ldapURL(tc.host, tc.port, tc.useTLS); got != tc.want {
				t.Errorf("ldapURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestShouldUseTLS pins the LDAPS port set: 636 (LDAPS well-known) and
// 3269 (AD Global Catalog over SSL) use TLS; 389/3268 stay plain.
func TestShouldUseTLS(t *testing.T) {
	tests := []struct {
		port int
		want bool
	}{
		{389, false},
		{636, true},
		{3268, false},
		{3269, true},
		{0, false},
	}
	for _, tc := range tests {
		if got := ShouldUseTLS(tc.port); got != tc.want {
			t.Errorf("ShouldUseTLS(%d) = %v, want %v", tc.port, got, tc.want)
		}
	}
}

// TestOperationTimeoutPinned guards the OPEN-API-75 P9 deviation fix:
// ldap.Conn.SetTimeout takes a time.Duration; 30*1000 would be 30µs.
func TestOperationTimeoutPinned(t *testing.T) {
	if ldapOperationTimeout != 30*time.Second {
		t.Errorf("ldapOperationTimeout = %v, want 30s", ldapOperationTimeout)
	}
}

// BE-B-18 pins: the dial itself must be bounded (SetTimeout only applies after
// the connection is established, so an unreachable host would otherwise hang
// until the OS TCP timeout), and a search must never run with SizeLimit=0
// (unlimited result set pulled into memory).
func TestDialTimeoutPinned(t *testing.T) {
	if ldapDialTimeout != 10*time.Second {
		t.Errorf("ldapDialTimeout = %v, want 10s", ldapDialTimeout)
	}
}

func TestSearchSizeLimitPinned(t *testing.T) {
	if ldapSearchSizeLimit <= 0 {
		t.Fatalf("ldapSearchSizeLimit = %d, must be positive (0 means unlimited)", ldapSearchSizeLimit)
	}
	if ldapSearchSizeLimit != 10000 {
		t.Errorf("ldapSearchSizeLimit = %d, want 10000", ldapSearchSizeLimit)
	}
}

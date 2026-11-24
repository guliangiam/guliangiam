package sessions

import (
	"testing"
	"time"
)

// BenchmarkSessionCreation benchmarks Session struct creation (in-memory only).
// Spec §4.1: Session恢复 < 20ms → session creation must be sub-millisecond.
func BenchmarkSessionCreation(b *testing.B) {
	now := time.Now()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = &Session{
			ID:               "sess-01912345-6789-7abc-def0-123456789012",
			UserID:           "user-01912345-6789-7abc-def0-123456789012",
			Username:         "admin",
			OrganizationID:   "org-01912345-6789-7abc-def0-123456789012",
			OrganizationSlug: "default",
			ApplicationID:    "app-01912345-6789-7abc-def0-123456789012",
			ApplicationSlug:  "app-built-in",
			SessionState:     "active",
			DeviceInfo:       map[string]any{"ua": "chrome/120"},
			IPAddress:        "192.168.1.100",
			UserAgent:        "Mozilla/5.0",
			ExpiresAt:        now.Add(24 * time.Hour),
			RevokedAt:        nil,
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
	}
}

// BenchmarkSessionExpiryCheck benchmarks session expiry validation.
func BenchmarkSessionExpiryCheck(b *testing.B) {
	now := time.Now()
	sess := &Session{
		ID:        "sess-benchmark",
		ExpiresAt: now.Add(24 * time.Hour),
		RevokedAt: nil,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = sess.ExpiresAt.After(now) && sess.RevokedAt == nil
	}
}

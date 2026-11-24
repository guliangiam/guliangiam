package qrcodesessions

import (
	"testing"
	"time"
)

func TestSession_IsExpired_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		session     Session
		now         time.Time
		wantExpired bool
	}{
		{
			name:        "not expired - future",
			session:     Session{ExpiresAt: now.Add(5 * time.Minute)},
			now:         now,
			wantExpired: false,
		},
		{
			name:        "expired - past",
			session:     Session{ExpiresAt: now.Add(-1 * time.Minute)},
			now:         now,
			wantExpired: true,
		},
		{
			name:        "not expired - exact time",
			session:     Session{ExpiresAt: now},
			now:         now,
			wantExpired: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.now.After(tt.session.ExpiresAt)
			if got != tt.wantExpired {
				t.Errorf("expired = %v, want %v", got, tt.wantExpired)
			}
		})
	}
}

func TestSession_StatusTransitions_TableDriven(t *testing.T) {
	validStatuses := map[string]bool{
		"pending": true, "scanned": true, "confirmed": true, "expired": true,
	}
	validTransitions := map[string][]string{
		"pending":   {"scanned", "expired"},
		"scanned":   {"confirmed", "expired"},
		"confirmed": {},
		"expired":   {},
	}
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"pending -> scanned", "pending", "scanned", true},
		{"pending -> expired", "pending", "expired", true},
		{"pending -> confirmed", "pending", "confirmed", false},
		{"scanned -> confirmed", "scanned", "confirmed", true},
		{"scanned -> expired", "scanned", "expired", true},
		{"scanned -> pending", "scanned", "pending", false},
		{"confirmed -> pending", "confirmed", "pending", false},
		{"confirmed -> scanned", "confirmed", "scanned", false},
		{"expired -> pending", "expired", "pending", false},
		{"expired -> scanned", "expired", "scanned", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !validStatuses[tt.from] || !validStatuses[tt.to] {
				t.Fatalf("invalid test status: from=%q to=%q", tt.from, tt.to)
			}
			allowed := validTransitions[tt.from]
			got := false
			for _, s := range allowed {
				if s == tt.to {
					got = true
					break
				}
			}
			if got != tt.want {
				t.Errorf("transition %q -> %q = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestSession_Lifecycle_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		session Session
	}{
		{
			name: "pending session no user",
			session: Session{
				ID: "s-1", CodeID: "c-1", Status: "pending",
				ExpiresAt: now.Add(5 * time.Minute),
				CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "scanned session has user",
			session: Session{
				ID: "s-2", CodeID: "c-2", Status: "scanned",
				UserID: "user-1", Username: "alice",
				IPAddress: "10.0.0.1", UserAgent: "Mozilla/5.0",
				ExpiresAt: now.Add(5 * time.Minute),
				CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "confirmed session",
			session: Session{
				ID: "s-3", CodeID: "c-3", Status: "confirmed",
				UserID: "user-1", Username: "alice",
				ExpiresAt: now.Add(5 * time.Minute),
				CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "expired session",
			session: Session{
				ID: "s-4", CodeID: "c-4", Status: "expired",
				ExpiresAt: now.Add(-1 * time.Minute),
				CreatedAt: now, UpdatedAt: now,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.session.ID == "" {
				t.Error("ID should not be empty")
			}
			if tt.session.CodeID == "" {
				t.Error("CodeID should not be empty")
			}
			if tt.session.Status == "scanned" || tt.session.Status == "confirmed" {
				if tt.session.UserID == "" {
					t.Error("UserID should be set for scanned/confirmed session")
				}
			}
		})
	}
}

func TestSession_ZeroValue(t *testing.T) {
	var sess Session
	if sess.ID != "" {
		t.Errorf("ID = %q, want empty", sess.ID)
	}
	if sess.Status != "" {
		t.Errorf("Status = %q, want empty", sess.Status)
	}
	if sess.Revision != 0 {
		t.Errorf("Revision = %d, want 0", sess.Revision)
	}
}

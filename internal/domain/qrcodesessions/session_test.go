package qrcodesessions

import (
	"testing"
	"time"
)

func TestSession_Fields(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	sess := Session{
		ID:             "qrs-001",
		CodeID:         "code-abc",
		Code:           "https://iam.example.com/qr/code-abc",
		OrganizationID: "org-1",
		ApplicationID:  "app-1",
		ExpiresAt:      expiresAt,
		Status:         "pending",
		UserID:         "",
		Username:       "",
		IPAddress:      "192.168.1.100",
		UserAgent:      "Mozilla/5.0",
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	if sess.ID != "qrs-001" {
		t.Errorf("ID = %q, want %q", sess.ID, "qrs-001")
	}
	if sess.CodeID != "code-abc" {
		t.Errorf("CodeID = %q, want %q", sess.CodeID, "code-abc")
	}
	if sess.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", sess.OrganizationID, "org-1")
	}
	if sess.ApplicationID != "app-1" {
		t.Errorf("ApplicationID = %q, want %q", sess.ApplicationID, "app-1")
	}
	if sess.Status != "pending" {
		t.Errorf("Status = %q, want %q", sess.Status, "pending")
	}
}

func TestSession_StatusValues(t *testing.T) {
	validStatuses := []string{"pending", "scanned", "confirmed", "expired"}
	for _, status := range validStatuses {
		sess := Session{Status: status}
		if sess.Status != status {
			t.Errorf("Status = %q, want %q", sess.Status, status)
		}
	}
}

func TestSession_InitialState(t *testing.T) {
	sess := Session{
		ID:        "qrs-002",
		CodeID:    "code-xyz",
		Status:    "pending",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Revision:  1,
	}

	if sess.UserID != "" {
		t.Errorf("UserID = %q, want empty for initial state", sess.UserID)
	}
	if sess.Username != "" {
		t.Errorf("Username = %q, want empty for initial state", sess.Username)
	}
	if sess.Status != "pending" {
		t.Errorf("Status = %q, want %q for initial state", sess.Status, "pending")
	}
}

func TestSession_ScannedState(t *testing.T) {
	sess := Session{
		ID:        "qrs-003",
		CodeID:    "code-scan",
		Status:    "scanned",
		UserID:    "user-1",
		Username:  "scanner",
		IPAddress: "10.0.0.1",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	if sess.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", sess.UserID, "user-1")
	}
	if sess.Username != "scanner" {
		t.Errorf("Username = %q, want %q", sess.Username, "scanner")
	}
	if sess.Status != "scanned" {
		t.Errorf("Status = %q, want %q", sess.Status, "scanned")
	}
}

func TestSession_ExpiredState(t *testing.T) {
	sess := Session{
		ID:        "qrs-004",
		CodeID:    "code-exp",
		Status:    "expired",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}

	if sess.Status != "expired" {
		t.Errorf("Status = %q, want %q", sess.Status, "expired")
	}
	if !time.Now().After(sess.ExpiresAt) {
		t.Error("expired session should have ExpiresAt in the past")
	}
}

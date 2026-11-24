package agents

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/agents"
)

func TestToAgent(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	lastPing := now.Add(-5 * time.Minute)
	// DB-058: the confidential column is persisted as ciphertext.
	encToken, err := agentEncryptedMapper.EncryptColumn(agentAAD("agent-1"), "secret-token")
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	row := agentRow{
		ID:             "agent-1",
		OrganizationID: "org-1",
		Name:           "policy-agent",
		DisplayName:    "Policy Agent",
		Type:           "casbin",
		Host:           "localhost",
		Port:           8080,
		Url:            "http://localhost:8080",
		Token:          encToken,
		ApplicationID:  "app-1",
		IsEnabled:      true,
		LastPingAt:     &lastPing,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	result, err := toAgent(row)
	if err != nil {
		t.Fatalf("toAgent() error: %v", err)
	}
	if result == nil {
		t.Fatal("toAgent() returned nil")
	}
	if result.Token != "secret-token" {
		t.Errorf("Token = %q, want secret-token", result.Token)
	}
	if result.ID != "agent-1" {
		t.Errorf("ID = %q, want agent-1", result.ID)
	}
	if result.Name != "policy-agent" {
		t.Errorf("Name = %q, want policy-agent", result.Name)
	}
	if result.Port != 8080 {
		t.Errorf("Port = %d, want 8080", result.Port)
	}
	if !result.IsEnabled {
		t.Error("IsEnabled = false, want true")
	}
	if result.LastPingAt == nil {
		t.Error("LastPingAt = nil, want non-nil")
	}
}

func TestToAgent_NilLastPing(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	row := agentRow{
		ID:        "agent-2",
		Name:      "offline-agent",
		IsEnabled: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	result, err := toAgent(row)
	if err != nil {
		t.Fatalf("toAgent() error: %v", err)
	}
	if result.LastPingAt != nil {
		t.Error("LastPingAt should be nil for nil lastPingAt")
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		value, fallback, want int
	}{
		{0, 10, 10},
		{-1, 5, 5},
		{20, 10, 20},
	}
	for _, tt := range tests {
		if got := maxInt(tt.value, tt.fallback); got != tt.want {
			t.Errorf("maxInt(%d, %d) = %d, want %d", tt.value, tt.fallback, got, tt.want)
		}
	}
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	if repo.db != nil {
		t.Error("expected nil db")
	}
}

func TestDomainAgentFields(t *testing.T) {
	a := &domain.Agent{
		ID:        "test",
		Name:      "test-agent",
		IsEnabled: true,
	}
	if a.ID != "test" {
		t.Errorf("ID = %q, want test", a.ID)
	}
}

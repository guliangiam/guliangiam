package sessions

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	platformssf "github.com/guliangiam/guliangiam/internal/platform/ssf"
)

func TestService_RevokeSession_EmitsSSFSessionRevoked(t *testing.T) {
	t.Setenv("ISSUER_URL", "https://op.example.com")
	now := time.Now()
	repo := &mockSessionRepo{sessions: []*domain.Session{{ID: "sess-1", UserID: "user-1", OrganizationID: "org-1"}}}
	outboxRepo := &mockOutboxRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{
		sessionRepo: repo,
		recordsRepo: &mockRecordsRepo{},
		outboxRepo:  outboxRepo,
	}}

	if err := svc.RevokeSession(context.Background(), "sess-1"); err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
	if outboxRepo.enqueueCalls != 2 {
		t.Fatalf("outbox Enqueue calls = %d, want 2 (audit + ssf)", outboxRepo.enqueueCalls)
	}
	var ssfEvent *struct {
		AggregateType string
		Payload       []byte
	}
	for _, event := range outboxRepo.events {
		if event.AggregateType == "ssf_event" {
			ssfEvent = &struct {
				AggregateType string
				Payload       []byte
			}{event.AggregateType, event.Payload}
		}
	}
	if ssfEvent == nil {
		t.Fatal("ssf_event not enqueued")
	}
	var payload struct {
		Type    string         `json:"type"`
		Issuer  string         `json:"issuer"`
		Subject map[string]any `json:"subject"`
	}
	if err := json.Unmarshal(ssfEvent.Payload, &payload); err != nil {
		t.Fatalf("decode ssf payload: %v", err)
	}
	if payload.Type != platformssf.EventSessionRevoked {
		t.Errorf("event type = %q", payload.Type)
	}
	if payload.Issuer != "https://op.example.com" {
		t.Errorf("issuer = %q", payload.Issuer)
	}
	if payload.Subject["session_id"] != "sess-1" || payload.Subject["sub"] != "user-1" {
		t.Errorf("subject = %#v", payload.Subject)
	}
}

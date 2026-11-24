package messages

import (
	"context"
	"testing"

	domainnotificationproviders "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-B-12: a resolved provider row without a usable ProviderType is a client
// error (InvalidArgument), not a factory 503; the factory must never see the
// raw row ID.
func TestSendNotificationProviderWithoutTypeIsInvalidArgument(t *testing.T) {
	f := &recordingFactory{}
	lookup := &fakeProviderLookup{provider: &domainnotificationproviders.NotificationProvider{
		ID: "11111111-2222-3333-4444-555555555555", OrganizationID: "org-1", ProviderType: "  ",
	}}
	svc := NewServiceWithProviderLookup(f, lookup)
	err := svc.SendNotification(context.Background(), "org-1", "11111111-2222-3333-4444-555555555555", "title", "content", []string{"a@example.com"})
	if err == nil {
		t.Fatal("expected blank providerType to be rejected")
	}
	if apperrors.FromError(err).Code != apperrors.CodeInvalidArgument {
		t.Fatalf("error code = %v, want InvalidArgument", apperrors.FromError(err).Code)
	}
	if len(f.calls) != 0 {
		t.Fatalf("factory must not be called when resolution fails, calls = %d", len(f.calls))
	}
}

// A nil provider row (repository returned no entity without an error) must
// fail explicitly instead of dereferencing nil or reaching the factory.
func TestSendNotificationNilProviderRowIsInvalidArgument(t *testing.T) {
	f := &recordingFactory{}
	lookup := &fakeProviderLookup{provider: nil}
	svc := NewServiceWithProviderLookup(f, lookup)
	err := svc.SendNotification(context.Background(), "org-1", "missing-id", "title", "content", []string{"a@example.com"})
	if err == nil {
		t.Fatal("expected nil provider row to be rejected")
	}
	if apperrors.FromError(err).Code != apperrors.CodeInvalidArgument {
		t.Fatalf("error code = %v, want InvalidArgument", apperrors.FromError(err).Code)
	}
	if len(f.calls) != 0 {
		t.Fatalf("factory must not be called, calls = %d", len(f.calls))
	}
}

package messages

import (
	"context"
	"errors"
	"testing"

	domainnotificationproviders "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domainnotifications "github.com/guliangiam/guliangiam/internal/domain/notifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type recordedCall struct {
	providerType string
	payload      *domainnotifications.SendNotification
}

type recordingFactory struct {
	calls  []recordedCall
	failOn int // 1-based call index that should fail; 0 = never
}

func (f *recordingFactory) SendNotification(_ context.Context, providerType string, n *domainnotifications.SendNotification) error {
	f.calls = append(f.calls, recordedCall{providerType: providerType, payload: n})
	if f.failOn > 0 && len(f.calls) == f.failOn {
		return errors.New("provider failure")
	}
	return nil
}

func TestSendEmailMapsRecipientAndContent(t *testing.T) {
	f := &recordingFactory{}
	svc := NewService(f)
	if err := svc.SendEmail(context.Background(), "to@example.com", "subject", "body"); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.calls[0].providerType != "email" {
		t.Fatalf("calls = %+v", f.calls)
	}
	if got := f.calls[0].payload; got.Recipient != "to@example.com" || got.Subject != "subject" || got.Content != "body" || got.Type != "email" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestSendSmsMapsPhoneAndCode(t *testing.T) {
	f := &recordingFactory{}
	svc := NewService(f)
	if err := svc.SendSms(context.Background(), "+8613800000000", "123456"); err != nil {
		t.Fatal(err)
	}
	got := f.calls[0].payload
	if f.calls[0].providerType != "sms" || got.Recipient != "+8613800000000" || got.Content != "123456" {
		t.Fatalf("call = %+v payload = %+v", f.calls[0], got)
	}
}

func TestSendNotificationFansOutAndStopsOnFailure(t *testing.T) {
	f := &recordingFactory{failOn: 2}
	svc := NewService(f)
	err := svc.SendNotification(context.Background(), "org-1", "", "title", "content", []string{"a@example.com", "b@example.com", "c@example.com"})
	if err == nil {
		t.Fatal("expected second target to fail")
	}
	if len(f.calls) != 2 {
		t.Fatalf("expected fan-out to stop after failure, calls = %d", len(f.calls))
	}
	if f.calls[0].providerType != "notification" || f.calls[0].payload.OrganizationID != "org-1" {
		t.Fatalf("call = %+v", f.calls[0])
	}
}

func TestSendNotificationResolvesProviderIDToType(t *testing.T) {
	f := &recordingFactory{}
	lookup := &fakeProviderLookup{provider: &domainnotificationproviders.NotificationProvider{
		ID: "11111111-2222-3333-4444-555555555555", OrganizationID: "org-1", ProviderType: "smtp",
	}}
	svc := NewServiceWithProviderLookup(f, lookup)
	if err := svc.SendNotification(context.Background(), "org-1", "11111111-2222-3333-4444-555555555555", "title", "content", []string{"a@example.com"}); err != nil {
		t.Fatal(err)
	}
	if f.calls[0].providerType != "smtp" {
		t.Fatalf("providerType = %q, want smtp (resolved from the provider row)", f.calls[0].providerType)
	}
	if lookup.gotID != "11111111-2222-3333-4444-555555555555" || lookup.gotOrg != "org-1" {
		t.Fatalf("lookup called with id=%q org=%q, want tenant-scoped resolution", lookup.gotID, lookup.gotOrg)
	}
}

func TestSendNotificationProviderLookupErrorPropagates(t *testing.T) {
	f := &recordingFactory{}
	lookup := &fakeProviderLookup{err: apperrors.NewNotFound("notification provider not found")}
	svc := NewServiceWithProviderLookup(f, lookup)
	err := svc.SendNotification(context.Background(), "org-1", "foreign-id", "title", "content", []string{"a@example.com"})
	if err == nil {
		t.Fatal("expected cross-tenant/missing provider to fail")
	}
	if apperrors.FromError(err).Code != apperrors.CodeNotFound {
		t.Fatalf("error code = %v, want NotFound", apperrors.FromError(err).Code)
	}
	if len(f.calls) != 0 {
		t.Fatalf("factory must not be called when resolution fails, calls = %d", len(f.calls))
	}
}

func TestSendNotificationProviderIDWithoutLookupFailsExplicitly(t *testing.T) {
	f := &recordingFactory{}
	svc := NewService(f)
	err := svc.SendNotification(context.Background(), "org-1", "some-provider-id", "title", "content", []string{"a@example.com"})
	if err == nil {
		t.Fatal("expected providerID path to fail without a provider lookup")
	}
	if len(f.calls) != 0 {
		t.Fatalf("factory must not receive the raw providerID, calls = %d", len(f.calls))
	}
}

func TestSendEmailRejectsInvalidRecipient(t *testing.T) {
	for _, to := range []string{"", "not-an-email", "a@", "@b.com"} {
		f := &recordingFactory{}
		svc := NewService(f)
		err := svc.SendEmail(context.Background(), to, "subject", "body")
		if err == nil {
			t.Fatalf("expected invalid recipient %q to be rejected", to)
		}
		// 空收件人是缺字段（400）；非空但格式非法是语义校验（422，手册 4.14.1）。
		wantCode := apperrors.CodeUnprocessableEntity
		if to == "" {
			wantCode = apperrors.CodeInvalidArgument
		}
		if apperrors.FromError(err).Code != wantCode {
			t.Fatalf("recipient %q: code = %v, want %v", to, apperrors.FromError(err).Code, wantCode)
		}
		if len(f.calls) != 0 {
			t.Fatalf("recipient %q: factory must not be called", to)
		}
	}
}

func TestSendSmsRejectsInvalidPhone(t *testing.T) {
	for _, phone := range []string{"", "abc", "12345", "+86-138-0000-0000", "1234567890123456"} {
		f := &recordingFactory{}
		svc := NewService(f)
		err := svc.SendSms(context.Background(), phone, "123456")
		if err == nil {
			t.Fatalf("expected invalid phone %q to be rejected", phone)
		}
		// 空号码是缺字段（400）；非空但格式非法是语义校验（422，手册 4.14.1）。
		wantCode := apperrors.CodeUnprocessableEntity
		if phone == "" {
			wantCode = apperrors.CodeInvalidArgument
		}
		if apperrors.FromError(err).Code != wantCode {
			t.Fatalf("phone %q: code = %v, want %v", phone, apperrors.FromError(err).Code, wantCode)
		}
		if len(f.calls) != 0 {
			t.Fatalf("phone %q: factory must not be called", phone)
		}
	}
}

type fakeProviderLookup struct {
	provider *domainnotificationproviders.NotificationProvider
	err      error
	gotID    string
	gotOrg   string
}

func (f *fakeProviderLookup) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domainnotificationproviders.NotificationProvider, error) {
	f.gotID = id
	f.gotOrg = organizationID
	return f.provider, f.err
}

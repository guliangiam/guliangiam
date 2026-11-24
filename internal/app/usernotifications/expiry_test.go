package usernotifications

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeExpiryScanner struct {
	reminders []ExpiryReminder
	now       time.Time
	deadline  time.Time
	limit     int
	err       error
}

func (s *fakeExpiryScanner) ScanExpiring(_ context.Context, now, deadline time.Time, limit int) ([]ExpiryReminder, error) {
	s.now = now
	s.deadline = deadline
	s.limit = limit
	return s.reminders, s.err
}

type fakeReminderSink struct {
	admins    map[string][]string
	existing  map[string]bool
	published []PublishCommand
}

func (s *fakeReminderSink) Administrators(_ context.Context, organizationID string) ([]string, error) {
	return s.admins[organizationID], nil
}

func (s *fakeReminderSink) Exists(_ context.Context, organizationID, userID, sourceType, sourceID string) bool {
	return s.existing[organizationID+"|"+userID+"|"+sourceType+"|"+sourceID]
}

func (s *fakeReminderSink) Publish(_ context.Context, command PublishCommand) error {
	s.published = append(s.published, command)
	return nil
}

func TestExpiryReminderWorkerTickPublishesPerAdminAndSkipsExisting(t *testing.T) {
	now := time.Date(2026, 11, 23, 8, 0, 0, 0, time.UTC)
	expiresAt := now.Add(48 * time.Hour)
	scanner := &fakeExpiryScanner{reminders: []ExpiryReminder{
		{Kind: ExpiryKindInvitation, OrganizationID: "org-1", SourceID: "inv-1", EntityName: "INV-CODE", ExpiresAt: expiresAt},
		{Kind: ExpiryKindCredential, OrganizationID: "org-1", SourceID: "cert-1", EntityName: "signing-key", ExpiresAt: expiresAt},
		{Kind: ExpiryKindClientCertificate, OrganizationID: "org-2", SourceID: "cc-1", EntityName: "Portal", ExpiresAt: expiresAt},
	}}
	sink := &fakeReminderSink{
		admins: map[string][]string{"org-1": {"u1", "u2"}, "org-2": {"u3"}},
		existing: map[string]bool{
			// 邀请提醒已经给 u1 发过：只应跳过这一条。
			"org-1|u1|expiry_invitation|inv-1": true,
		},
	}
	worker := &ExpiryReminderWorker{
		Scanner: scanner,
		Sink:    sink,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:     func() time.Time { return now },
	}

	if err := worker.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if !scanner.now.Equal(now) {
		t.Fatalf("scan now = %v, want %v", scanner.now, now)
	}
	if !scanner.deadline.Equal(now.Add(DefaultExpiryReminderLeadTime)) {
		t.Fatalf("scan deadline = %v, want now+lead time", scanner.deadline)
	}
	if scanner.limit != expiryScanBatchSize {
		t.Fatalf("scan limit = %d, want %d", scanner.limit, expiryScanBatchSize)
	}

	// 邀请：u1 命中幂等跳过，u2 投递；凭据：u1/u2 各一条；客户端证书：u3 一条。
	if len(sink.published) != 4 {
		t.Fatalf("published = %d commands, want 4: %#v", len(sink.published), sink.published)
	}

	byKey := make(map[string]PublishCommand, len(sink.published))
	for _, command := range sink.published {
		byKey[command.UserID+"|"+command.EventType] = command
	}
	invitation := byKey["u2|invitation.expiring"]
	if invitation.Category != "business" || invitation.Title != "邀请码即将过期" ||
		invitation.Link != "/invitations" || invitation.SourceType != "expiry_invitation" {
		t.Fatalf("invitation command = %#v", invitation)
	}
	if !strings.Contains(invitation.Content, "INV-CODE") {
		t.Fatalf("invitation content = %q", invitation.Content)
	}
	credential := byKey["u1|credential.expiring"]
	if credential.Category != "security" || credential.Link != "/certs" ||
		credential.SourceType != "expiry_credential" {
		t.Fatalf("credential command = %#v", credential)
	}
	clientCertificate := byKey["u3|application.client_certificate.expiring"]
	if clientCertificate.Category != "business" || clientCertificate.Link != "/applications" ||
		clientCertificate.SourceType != "expiry_client_certificate" {
		t.Fatalf("client certificate command = %#v", clientCertificate)
	}

	for _, command := range sink.published {
		if len(command.NotificationID) != 36 {
			t.Fatalf("deterministic notification id = %q", command.NotificationID)
		}
		if command.OrganizationID == "" || command.SourceID == "" {
			t.Fatalf("missing tenant/source: %#v", command)
		}
	}
}

func TestExpiryReminderWorkerPublishesPartialScanResults(t *testing.T) {
	now := time.Date(2026, 11, 23, 8, 0, 0, 0, time.UTC)
	scanErr := errors.New("certs scan failed")
	scanner := &fakeExpiryScanner{
		err: scanErr,
		reminders: []ExpiryReminder{
			{Kind: ExpiryKindInvitation, OrganizationID: "org-1", SourceID: "inv-1", EntityName: "INV-CODE", ExpiresAt: now.Add(time.Hour)},
		},
	}
	sink := &fakeReminderSink{admins: map[string][]string{"org-1": {"u1"}}}
	worker := &ExpiryReminderWorker{
		Scanner: scanner,
		Sink:    sink,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:     func() time.Time { return now },
	}

	err := worker.Tick(context.Background())
	if !errors.Is(err, scanErr) {
		t.Fatalf("Tick() error = %v, want scan error", err)
	}
	if len(sink.published) != 1 {
		t.Fatalf("published = %d commands, want 1", len(sink.published))
	}
}

func TestRenderExpiryReminderMatchesRegistryTemplates(t *testing.T) {
	expiresAt := time.Date(2026, 12, 1, 9, 30, 0, 0, time.Local)

	title, content, link, sourceType, ok := renderExpiryReminder(ExpiryReminder{
		Kind: ExpiryKindInvitation, EntityName: "ABC123", ExpiresAt: expiresAt,
	})
	if !ok || title != "邀请码即将过期" || content != "邀请码 ABC123 将于 2026-12-01 09:30 过期。" ||
		link != "/invitations" || sourceType != expiryInvitationSourceType {
		t.Fatalf("invitation render = %q %q %q %q %v", title, content, link, sourceType, ok)
	}

	title, content, link, sourceType, ok = renderExpiryReminder(ExpiryReminder{
		Kind: ExpiryKindCredential, EntityName: "signing-key", ExpiresAt: expiresAt,
	})
	if !ok || title != "凭据即将过期" || content != "凭据 signing-key 将于 2026-12-01 09:30 过期，请及时轮换。" ||
		link != "/certs" || sourceType != expiryCredentialSourceType {
		t.Fatalf("credential render = %q %q %q %q %v", title, content, link, sourceType, ok)
	}

	title, content, link, sourceType, ok = renderExpiryReminder(ExpiryReminder{
		Kind: ExpiryKindClientCertificate, EntityName: "Portal", ExpiresAt: expiresAt,
	})
	if !ok || title != "应用证书即将过期" || content != "应用 Portal 的证书将于 2026-12-01 09:30 过期。" ||
		link != "/applications" || sourceType != expiryClientCertificateSourceType {
		t.Fatalf("client certificate render = %q %q %q %q %v", title, content, link, sourceType, ok)
	}

	if _, _, _, _, ok := renderExpiryReminder(ExpiryReminder{Kind: "unknown"}); ok {
		t.Fatal("unknown kind must not render")
	}
}

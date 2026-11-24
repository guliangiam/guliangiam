package auth

import (
	"context"
	"testing"
	"time"

	deviceauthcodesdomain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// These tests pin RFC 8628 §3.5 as implemented for 设计报告: the server
// enforces the polling interval instead of merely advertising it, terminates a
// client that keeps violating it, and reports `expired_token` for a code the
// client can no longer use rather than the private `invalid_device_code` that
// matched nothing in the RFC.

// testDeviceAuthCodeRepository is an in-memory device-code store whose counters
// behave like the SQL: the increment happens where the record is written.
type testDeviceAuthCodeRepository struct {
	entries map[string]*deviceauthcodesdomain.DeviceAuthCode
	deleted []string
}

func (r *testDeviceAuthCodeRepository) GetByDeviceCode(_ context.Context, deviceCode string) (*deviceauthcodesdomain.DeviceAuthCode, error) {
	entry, ok := r.entries[deviceCode]
	if !ok {
		return nil, apperrors.NewNotFound("device auth code not found")
	}
	return entry, nil
}

func (r *testDeviceAuthCodeRepository) GetByUserCode(_ context.Context, userCode string) (*deviceauthcodesdomain.DeviceAuthCode, error) {
	for _, entry := range r.entries {
		if entry.UserCode == userCode {
			return entry, nil
		}
	}
	return nil, apperrors.NewNotFound("device auth code not found")
}

func (r *testDeviceAuthCodeRepository) Create(_ context.Context, code *deviceauthcodesdomain.DeviceAuthCode) error {
	r.entries[code.DeviceCode] = code
	return nil
}

func (r *testDeviceAuthCodeRepository) UpdateVerification(_ context.Context, code *deviceauthcodesdomain.DeviceAuthCode) error {
	r.entries[code.DeviceCode] = code
	return nil
}

func (r *testDeviceAuthCodeRepository) RecordPoll(_ context.Context, deviceCode string, polledAt time.Time) (int, error) {
	entry, ok := r.entries[deviceCode]
	if !ok {
		return 0, apperrors.NewNotFound("device auth code not found")
	}
	entry.SlowDownCount++
	entry.LastPolledAt = &polledAt
	return entry.SlowDownCount, nil
}

func (r *testDeviceAuthCodeRepository) ResetSlowDown(_ context.Context, deviceCode string, polledAt time.Time) error {
	entry, ok := r.entries[deviceCode]
	if !ok {
		return apperrors.NewNotFound("device auth code not found")
	}
	entry.SlowDownCount = 0
	entry.LastPolledAt = &polledAt
	return nil
}

func (r *testDeviceAuthCodeRepository) DeleteByDeviceCode(_ context.Context, deviceCode string) error {
	delete(r.entries, deviceCode)
	r.deleted = append(r.deleted, deviceCode)
	return nil
}

func (r *testDeviceAuthCodeRepository) DeleteByUserCode(_ context.Context, userCode string) error {
	for key, entry := range r.entries {
		if entry.UserCode == userCode {
			delete(r.entries, key)
		}
	}
	return nil
}

func (r *testDeviceAuthCodeRepository) DeleteExpired(_ context.Context) (int64, error) {
	return 0, nil
}

// deviceTestNow matches fixedAuthClock so the fixtures below can be stated as
// offsets instead of absolute instants that silently fall on one side of "now".
var deviceTestNow = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

func deviceTestEntry(lastPolled *time.Time) *deviceauthcodesdomain.DeviceAuthCode {
	return &deviceauthcodesdomain.DeviceAuthCode{
		ID:              "device-1",
		DeviceCode:      "device-code-1",
		UserCode:        "ABCD-EFGH",
		ClientID:        "device-client",
		Scope:           "openid",
		ExpiresAt:       deviceTestNow.Add(30 * time.Minute),
		PollIntervalSec: 5,
		LastPolledAt:    lastPolled,
	}
}

func newDeviceTestService(t *testing.T, entry *deviceauthcodesdomain.DeviceAuthCode) (Service, *testDeviceAuthCodeRepository) {
	t.Helper()

	repo := &testDeviceAuthCodeRepository{
		entries: map[string]*deviceauthcodesdomain.DeviceAuthCode{entry.DeviceCode: entry},
	}
	// fixedAuthClock is 2026-09-16 12:00:00 UTC, which is inside the 5s interval
	// of a poll recorded at the same instant, and outside it for one an hour old.
	service := Service{
		Clock:         fixedAuthClock{},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "generated"},
		UOW: serviceTestUnitOfWork{
			loginTestUnitOfWork: loginTestUnitOfWork{appRepo: &loginTestAppRepository{}},
			deviceRepo:          repo,
		},
	}
	return service, repo
}

func devicePoll(service *Service) error {
	_, err := service.DeviceToken(context.Background(), DeviceTokenCommand{
		DeviceCode: "device-code-1",
		ClientID:   "device-client",
	})
	return err
}

func deviceProtocolCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	protocolErr, ok := err.(apperrors.ProtocolCodeError)
	if !ok {
		t.Fatalf("error %T does not carry a protocol code: %v", err, err)
	}
	return protocolErr.ProtocolCode()
}

func TestDeviceToken_SlowDownWhenPollingTooFast(t *testing.T) {
	justPolled := deviceTestNow
	service, _ := newDeviceTestService(t, deviceTestEntry(&justPolled))

	if got := deviceProtocolCode(t, devicePoll(&service)); got != deviceErrorSlowDown {
		t.Fatalf("protocol code = %q, want %q", got, deviceErrorSlowDown)
	}
}

func TestDeviceToken_TerminatesAfterRepeatedViolations(t *testing.T) {
	justPolled := deviceTestNow
	entry := deviceTestEntry(&justPolled)
	entry.SlowDownCount = maxDeviceSlowDownCount - 1
	service, repo := newDeviceTestService(t, entry)

	if got := deviceProtocolCode(t, devicePoll(&service)); got != deviceErrorExpiredToken {
		t.Fatalf("protocol code = %q, want %q", got, deviceErrorExpiredToken)
	}
	if len(repo.deleted) == 0 {
		t.Fatal("expected the device code to be deleted once the authorization is terminated")
	}
}

func TestDeviceToken_CompliantPollClearsTheCounter(t *testing.T) {
	longAgo := deviceTestNow.Add(-time.Hour)
	entry := deviceTestEntry(&longAgo)
	entry.SlowDownCount = 2
	service, repo := newDeviceTestService(t, entry)

	// The entry is unverified, so the call still fails — but with
	// authorization_pending, which proves the throttle admitted the poll.
	if got := deviceProtocolCode(t, devicePoll(&service)); got != deviceErrorAuthorizationPending {
		t.Fatalf("protocol code = %q, want %q", got, deviceErrorAuthorizationPending)
	}
	if got := repo.entries["device-code-1"].SlowDownCount; got != 0 {
		t.Fatalf("slow_down counter = %d, want 0 after a compliant poll", got)
	}
}

func TestDeviceToken_UnknownDeviceCodeIsExpiredToken(t *testing.T) {
	service, _ := newDeviceTestService(t, deviceTestEntry(nil))

	_, err := service.DeviceToken(context.Background(), DeviceTokenCommand{
		DeviceCode: "no-such-code",
		ClientID:   "device-client",
	})
	// RFC 8628 §3.5 has no `invalid_device_code`; a client dispatches on
	// expired_token for a code it can no longer use.
	if got := deviceProtocolCode(t, err); got != deviceErrorExpiredToken {
		t.Fatalf("protocol code = %q, want %q", got, deviceErrorExpiredToken)
	}
}

func TestDeviceToken_ExpiredCodeIsExpiredTokenAndIsRemoved(t *testing.T) {
	entry := deviceTestEntry(nil)
	entry.ExpiresAt = deviceTestNow.Add(-time.Minute)
	service, repo := newDeviceTestService(t, entry)

	if got := deviceProtocolCode(t, devicePoll(&service)); got != deviceErrorExpiredToken {
		t.Fatalf("protocol code = %q, want %q", got, deviceErrorExpiredToken)
	}
	if len(repo.deleted) == 0 {
		t.Fatal("expected an expired device code to be removed")
	}
}

func TestDeviceToken_RejectsWrongClient(t *testing.T) {
	service, _ := newDeviceTestService(t, deviceTestEntry(nil))

	_, err := service.DeviceToken(context.Background(), DeviceTokenCommand{
		DeviceCode: "device-code-1",
		ClientID:   "some-other-client",
	})
	if err == nil {
		t.Fatal("expected a client mismatch to be refused")
	}
}

func TestVerificationURI_IsAbsolute(t *testing.T) {
	cases := map[string]string{
		"https://iam.acme.example":  "https://iam.acme.example/device",
		"https://iam.acme.example/": "https://iam.acme.example/device",
		"":                          "",
	}
	for issuer, want := range cases {
		if got := verificationURI(issuer); got != want {
			t.Errorf("verificationURI(%q) = %q, want %q", issuer, got, want)
		}
	}
}

func TestVerificationURIComplete_CarriesUserCode(t *testing.T) {
	got := verificationURIComplete("https://iam.acme.example", "ABCD-EFGH")
	want := "https://iam.acme.example/device?user_code=ABCD-EFGH"
	if got != want {
		t.Fatalf("verificationURIComplete() = %q, want %q", got, want)
	}
	if verificationURIComplete("", "ABCD-EFGH") != "" {
		t.Fatal("expected an empty URI without an issuer")
	}
	if verificationURIComplete("https://iam.acme.example", "") != "" {
		t.Fatal("expected an empty URI without a user code")
	}
}

// TestDeviceErrorCodes_MatchRFC8628 guards the wire strings: a device client
// switches on these exact values, so a rename is a silent protocol break.
func TestDeviceErrorCodes_MatchRFC8628(t *testing.T) {
	want := map[string]string{
		deviceErrorAuthorizationPending: "authorization_pending",
		deviceErrorSlowDown:             "slow_down",
		deviceErrorExpiredToken:         "expired_token",
		deviceErrorAccessDenied:         "access_denied",
	}
	for got, expected := range want {
		if got != expected {
			t.Errorf("device error code = %q, want %q", got, expected)
		}
	}
}

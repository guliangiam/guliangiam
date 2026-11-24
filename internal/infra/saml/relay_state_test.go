package saml

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestRelayStateSigner(t *testing.T) *RelayStateSigner {
	t.Helper()

	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	signer, err := NewRelayStateSigner(key)
	if err != nil {
		t.Fatalf("NewRelayStateSigner: %v", err)
	}
	signer.now = func() time.Time {
		return time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC)
	}
	return signer
}

func TestRelayStateSigner_RoundTrip(t *testing.T) {
	signer := newTestRelayStateSigner(t)

	signed, err := signer.Sign("raw-state", "acme", "app")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !strings.HasPrefix(signed, "v1.") {
		t.Fatalf("expected v1 envelope, got %q", signed)
	}

	got, err := signer.Verify(signed, "acme", "app")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got != "raw-state" {
		t.Fatalf("expected raw-state, got %q", got)
	}
}

func TestRelayStateSigner_RejectsWrongOrganization(t *testing.T) {
	signer := newTestRelayStateSigner(t)
	signed, _ := signer.Sign("raw-state", "acme", "app")

	if _, err := signer.Verify(signed, "other-org", "app"); !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
}

func TestRelayStateSigner_RejectsWrongApplication(t *testing.T) {
	signer := newTestRelayStateSigner(t)
	signed, _ := signer.Sign("raw-state", "acme", "app")

	if _, err := signer.Verify(signed, "acme", "other-app"); !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
}

func TestRelayStateSigner_RejectsTamperedPayload(t *testing.T) {
	signer := newTestRelayStateSigner(t)
	signed, _ := signer.Sign("raw-state", "acme", "app")

	tampered := strings.Replace(signed, "raw-state", "evil-state", 1)
	if _, err := signer.Verify(tampered, "acme", "app"); !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
}

func TestRelayStateSigner_RejectsTamperedSignature(t *testing.T) {
	signer := newTestRelayStateSigner(t)
	signed, _ := signer.Sign("raw-state", "acme", "app")

	parts := strings.SplitN(signed, ".", 4)
	parts[2] = strings.Repeat("0", len(parts[2]))
	tampered := strings.Join(parts, ".")

	if _, err := signer.Verify(tampered, "acme", "app"); !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
}

func TestRelayStateSigner_RejectsExpired(t *testing.T) {
	signer := newTestRelayStateSigner(t)
	signed, _ := signer.Sign("raw-state", "acme", "app")

	signer.now = func() time.Time {
		return time.Date(2026, 10, 26, 8, 20, 0, 0, time.UTC)
	}
	if _, err := signer.Verify(signed, "acme", "app"); !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
}

func TestRelayStateSigner_RejectsFutureIssueTime(t *testing.T) {
	signer := newTestRelayStateSigner(t)
	signed, _ := signer.Sign("raw-state", "acme", "app")

	signer.now = func() time.Time {
		return time.Date(2026, 10, 26, 7, 50, 0, 0, time.UTC)
	}
	if _, err := signer.Verify(signed, "acme", "app"); !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
}

func TestRelayStateSigner_RejectsMalformedEnvelope(t *testing.T) {
	signer := newTestRelayStateSigner(t)

	cases := []string{
		"not-signed",
		"v1.only.two",
		"v2.1793000000.00.raw",
		"v1.not-a-number.00.raw",
		"v1.1793000000.zz.raw",
		"v1.1793000000...raw",
	}
	for _, input := range cases {
		if _, err := signer.Verify(input, "acme", "app"); !errors.Is(err, ErrInvalidRelayState) {
			t.Fatalf("input %q: expected ErrInvalidRelayState, got %v", input, err)
		}
	}
}

func TestRelayStateSigner_RejectsOversizedState(t *testing.T) {
	signer := newTestRelayStateSigner(t)

	if _, err := signer.Sign(strings.Repeat("a", relayStateMaxLength+1), "acme", "app"); !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
	if _, err := signer.Sign("", "acme", "app"); !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState for empty state, got %v", err)
	}
}

func TestNewRelayStateSigner_RejectsEmptyKey(t *testing.T) {
	if _, err := NewRelayStateSigner("   "); err == nil {
		t.Fatal("expected error for empty master key")
	}
}

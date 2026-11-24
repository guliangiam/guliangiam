package b2b

import (
	"strings"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ── Validate() ────────────────────────────────────────────────────────────

// 全角/非 ASCII 码点在旧实现里会被 byte(ch) 截断成 ASCII 合法字符（例如全角
// 'Ｅ' U+FF25 截断为 'E'），从而把非法域名误判为合法。校验必须按 rune 比较。
func TestIsValidDomainFormat_RejectsNonASCIILabels(t *testing.T) {
	for _, domain := range []string{"Ｅxample.com", "exampｌe.com", "例え.com"} {
		if isValidDomainFormat(domain) {
			t.Errorf("isValidDomainFormat(%q) = true, want false", domain)
		}
	}
	if !isValidDomainFormat("example.com") {
		t.Error("ASCII domain must stay valid")
	}
}

func TestOrganizationDomain_Validate_DefaultsStatusToPending(t *testing.T) {
	d := &OrganizationDomain{
		OrganizationID:    "org-1",
		Domain:            "corp.example.com",
		VerificationToken: "GuliangIAM-Verification=v1:abc123",
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != StatusPending {
		t.Errorf("expected default status %q, got %q", StatusPending, d.Status)
	}
}

func TestOrganizationDomain_Validate_RejectsIPAsDomain(t *testing.T) {
	d := &OrganizationDomain{
		OrganizationID:    "org-1",
		Domain:            "192.168.1.1",
		VerificationToken: "tok",
	}
	err := d.Validate()
	if err == nil {
		t.Fatal("expected error for IP address input")
	}
}

func TestOrganizationDomain_Validate_RejectsBareHostname(t *testing.T) {
	d := &OrganizationDomain{
		OrganizationID:    "org-1",
		Domain:            "localhost",
		VerificationToken: "tok",
	}
	err := d.Validate()
	if err == nil {
		t.Fatal("expected error for hostname without dot")
	}
}

func TestOrganizationDomain_Validate_AcceptsSubdomain(t *testing.T) {
	d := &OrganizationDomain{
		OrganizationID:    "org-1",
		Domain:            "engineering.corp.example.com",
		VerificationToken: "tok",
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrganizationDomain_Validate_RejectsEmptyDomain(t *testing.T) {
	d := &OrganizationDomain{
		OrganizationID:    "org-1",
		Domain:            "",
		VerificationToken: "tok",
	}
	err := d.Validate()
	if err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestOrganizationDomain_Validate_RejectsInvalidStatus(t *testing.T) {
	d := &OrganizationDomain{
		OrganizationID:    "org-1",
		Domain:            "corp.example.com",
		VerificationToken: "tok",
		Status:            "revoked",
	}
	err := d.Validate()
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
}

// ── 状态机 ────────────────────────────────────────────────────────────────

func TestMarkVerified_PendingToVerified(t *testing.T) {
	d := &OrganizationDomain{
		Status: StatusPending,
	}
	now := time.Now()
	if err := d.MarkVerified(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != StatusVerified {
		t.Errorf("expected status %q, got %q", StatusVerified, d.Status)
	}
	if !d.VerifiedAt.Equal(now) {
		t.Error("VerifiedAt not set correctly")
	}
}

func TestMarkVerified_RejectsAlreadyVerified(t *testing.T) {
	d := &OrganizationDomain{Status: StatusVerified}
	err := d.MarkVerified(time.Now())
	if err == nil {
		t.Fatal("expected error: verified is terminal")
	}
}

func TestMarkVerified_RejectsFailed(t *testing.T) {
	d := &OrganizationDomain{Status: StatusFailed}
	err := d.MarkVerified(time.Now())
	if err == nil {
		t.Fatal("expected error: failed is terminal")
	}
}

func TestMarkFailed_PendingToFailed(t *testing.T) {
	d := &OrganizationDomain{Status: StatusPending}
	if err := d.MarkFailed(time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, d.Status)
	}
}

func TestMarkFailed_RejectsVerified(t *testing.T) {
	d := &OrganizationDomain{Status: StatusVerified}
	err := d.MarkFailed(time.Now())
	if err == nil {
		t.Fatal("expected error: verified is terminal")
	}
}

// ── 续验预算（RecordVerificationFailure / Success / IsDue）──────────────────

func TestRecordVerificationFailure_SchedulesRetryWithinBudget(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	d := &OrganizationDomain{Status: StatusPending}
	err := d.RecordVerificationFailure("txt_missing: no TXT record found", now, 5, func(attempts int) time.Duration {
		return time.Duration(attempts) * time.Minute
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != StatusPending {
		t.Errorf("status = %q, want pending", d.Status)
	}
	if d.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", d.Attempts)
	}
	if d.LastError != "txt_missing: no TXT record found" {
		t.Errorf("last_error = %q", d.LastError)
	}
	if !d.LastCheckedAt.Equal(now) {
		t.Error("last_checked_at not set")
	}
	if got, want := d.NextRetryAt, now.Add(time.Minute); !got.Equal(want) {
		t.Errorf("next_retry_at = %v, want %v", got, want)
	}
	if !d.UpdatedAt.Equal(now) {
		t.Error("updated_at not set")
	}
}

func TestRecordVerificationFailure_ExhaustsBudgetAndFails(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	d := &OrganizationDomain{Status: StatusPending, Attempts: 4}
	err := d.RecordVerificationFailure("dns_error: TXT lookup failed", now, 5, func(int) time.Duration {
		return time.Minute
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != StatusFailed {
		t.Errorf("status = %q, want failed", d.Status)
	}
	if d.Attempts != 5 {
		t.Errorf("attempts = %d, want 5", d.Attempts)
	}
	if !d.LastCheckedAt.Equal(now) {
		t.Error("last_checked_at not set")
	}
}

func TestRecordVerificationFailure_BackoffMonotonic(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	backoff := func(attempts int) time.Duration {
		// 测试下限：1,2,4,8 分钟（小于 maxAttempts，避免转终态）
		return time.Duration(1<<uint(attempts-1)) * time.Minute
	}
	d := &OrganizationDomain{Status: StatusPending}
	var prev time.Duration
	for i := 1; i <= 4; i++ {
		if err := d.RecordVerificationFailure("txt_missing", now, 5, backoff); err != nil {
			t.Fatalf("attempt %d: unexpected error: %v", i, err)
		}
		delay := d.NextRetryAt.Sub(now)
		if delay < prev {
			t.Fatalf("attempt %d: backoff %v decreased from %v", i, delay, prev)
		}
		prev = delay
	}
	if d.Attempts != 4 || d.Status != StatusPending {
		t.Fatalf("attempts/status = %d/%s, want 4/pending", d.Attempts, d.Status)
	}
}

func TestRecordVerificationFailure_RejectsNonPending(t *testing.T) {
	for _, status := range []string{StatusVerified, StatusFailed} {
		d := &OrganizationDomain{Status: status}
		err := d.RecordVerificationFailure("txt_missing", time.Now(), 5, nil)
		if err == nil {
			t.Errorf("status %q: expected error", status)
		}
		if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
			t.Errorf("status %q: code = %q, want 422", status, code)
		}
	}
}

func TestRecordVerificationFailure_TruncatesLastErrorByRune(t *testing.T) {
	long := strings.Repeat("验", MaxLastErrorRunes+10)
	d := &OrganizationDomain{Status: StatusPending}
	if err := d.RecordVerificationFailure(long, time.Now(), 5, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len([]rune(d.LastError)); got != MaxLastErrorRunes {
		t.Errorf("last_error rune length = %d, want %d", got, MaxLastErrorRunes)
	}
}

func TestRecordVerificationSuccess_ClearsLastError(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	d := &OrganizationDomain{
		Status: StatusPending, Attempts: 2, LastError: "txt_missing",
	}
	if err := d.RecordVerificationSuccess(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != StatusVerified {
		t.Errorf("status = %q, want verified", d.Status)
	}
	if !d.VerifiedAt.Equal(now) {
		t.Error("verified_at not set")
	}
	if d.LastError != "" {
		t.Errorf("last_error = %q, want empty", d.LastError)
	}
	if !d.LastCheckedAt.Equal(now) {
		t.Error("last_checked_at not set")
	}
}

func TestRecordVerificationSuccess_RejectsNonPending(t *testing.T) {
	d := &OrganizationDomain{Status: StatusFailed}
	if err := d.RecordVerificationSuccess(time.Now()); err == nil {
		t.Fatal("expected error: failed is terminal")
	}
}

func TestIsDue(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		d    OrganizationDomain
		want bool
	}{
		{"pending with zero retry time", OrganizationDomain{Status: StatusPending}, true},
		{"pending before next retry", OrganizationDomain{Status: StatusPending, NextRetryAt: now.Add(time.Minute)}, false},
		{"pending at next retry", OrganizationDomain{Status: StatusPending, NextRetryAt: now}, true},
		{"pending after next retry", OrganizationDomain{Status: StatusPending, NextRetryAt: now.Add(-time.Minute)}, true},
		{"verified", OrganizationDomain{Status: StatusVerified}, false},
		{"failed", OrganizationDomain{Status: StatusFailed}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.d.IsDue(now); got != tc.want {
				t.Errorf("IsDue = %v, want %v", got, tc.want)
			}
		})
	}
}

// ── IsTerminal ────────────────────────────────────────────────────────────

func TestIsTerminal(t *testing.T) {
	if IsTerminal(StatusPending) {
		t.Error("pending should not be terminal")
	}
	if !IsTerminal(StatusVerified) {
		t.Error("verified should be terminal")
	}
	if !IsTerminal(StatusFailed) {
		t.Error("failed should be terminal")
	}
}

func TestOrganizationDomain_SemanticRulesAnswer422(t *testing.T) {
	ip := &OrganizationDomain{Domain: "1.2.3.4"}
	if code := apperrors.FromError(ip.Validate()).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("ip domain: code = %q, want 422", code)
	}

	verified := &OrganizationDomain{Status: StatusVerified}
	if code := apperrors.FromError(verified.MarkVerified(time.Now())).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("verify non-pending: code = %q, want 422", code)
	}
}

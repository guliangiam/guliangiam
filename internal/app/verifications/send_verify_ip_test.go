package verifications

import (
	"context"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestConsumeVerificationCodeWithClientIP_RejectsExhaustedIP(t *testing.T) {
	rateLimiter := NewRateLimiter()
	t.Cleanup(rateLimiter.Stop)
	ip := "203.0.113.9"
	for i := 0; i < MaxIPVerifyAttempts; i++ {
		rateLimiter.RecordIPAttempt(ip)
	}

	svc := Service{RateLimiter: rateLimiter}
	err := svc.ConsumeVerificationCodeWithClientIP(
		context.Background(), "", VerificationTypeEmail, "someone@example.com", "000000", ip)
	typed, ok := err.(*apperrors.Error)
	if !ok || typed.Code != apperrors.CodeTooManyRequests {
		t.Fatalf("ConsumeVerificationCodeWithClientIP() error = %v, want code %q", err, apperrors.CodeTooManyRequests)
	}
}

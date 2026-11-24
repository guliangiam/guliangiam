package auth

import (
	"context"
	"strings"

	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/phonefmt"
)

const (
	verificationTypeEmail = "email"
	verificationTypePhone = "phone"

	// Login verification requests use "sms" while the verification table uses
	// "phone"; the request-level channel never leaks into storage.
	loginVerificationChannelEmail = "email"
	loginVerificationChannelSMS   = "sms"
)

func isLoginVerificationType(value string) bool {
	switch strings.TrimSpace(value) {
	case loginVerificationChannelEmail, loginVerificationChannelSMS:
		return true
	default:
		return false
	}
}

func loginVerificationTableType(channel string) (string, bool) {
	switch strings.TrimSpace(channel) {
	case loginVerificationChannelEmail:
		return verificationTypeEmail, true
	case loginVerificationChannelSMS:
		return verificationTypePhone, true
	default:
		return "", false
	}
}

// consumeLoginVerificationCode validates and consumes a login one-time code.
// It is a function variable so tests can replace it; bootstrap wires it to the
// verification service, which enforces expiry, one-time consumption, recipient
// and per-IP rate limits. organizationID binds the code to the tenant resolved
// from the login request.
var consumeLoginVerificationCode = func(ctx context.Context, organizationID, verificationType, recipient, code, clientIP string) error {
	_ = ctx
	_ = organizationID
	_ = verificationType
	_ = recipient
	_ = code
	_ = clientIP
	return apperrors.NewUnimplemented("login verification code check is not yet wired")
}

// WireLoginVerificationConsumer connects the login code consumer to the
// verification service. Call this once during bootstrap.
func WireLoginVerificationConsumer(consume func(ctx context.Context, organizationID, verificationType, recipient, code, clientIP string) error) {
	consumeLoginVerificationCode = consume
}

// verifyEmailCode checks if the given email verification code is valid and consumes it.
// This is a function variable that can be replaced in tests.
var verifyEmailCode = func(ctx context.Context, dest, passcode string) error {
	// When not wired, returns unimplemented. The actual implementation is
	// called via (Service).verifyEmailCodeWithRepo when ctx and repo are available.
	_ = ctx
	_ = dest
	_ = passcode
	return apperrors.NewUnimplemented("email verification code check is not yet wired")
}

// verifySmsCode checks if the given SMS verification code is valid and consumes it.
var verifySmsCode = func(ctx context.Context, phone, countryCode, passcode string) error {
	_ = ctx
	_ = phone
	_ = countryCode
	_ = passcode
	return apperrors.NewUnimplemented("sms verification code check is not yet wired")
}

// WireVerificationAdapters connects the verification code function variables
// to the repository-based implementations. Call this once during bootstrap.
func WireVerificationAdapters(getRepo func() verificationsdomain.Repository) {
	verifyEmailCode = func(ctx context.Context, dest, passcode string) error {
		return verifyEmailCodeWithRepo(ctx, getRepo(), dest, passcode)
	}
	verifySmsCode = func(ctx context.Context, phone, countryCode, passcode string) error {
		return verifySmsCodeWithRepo(ctx, getRepo(), phone, countryCode, passcode)
	}
}

// matchVerificationCode keeps the legacy plaintext lookup first (so existing
// rows and repository expectations stay intact), then falls back to the digest
// path for rows written after verification-code hashing.
//
// organizationID binds the code to the tenant resolved by the calling flow: a
// code issued for another organization is rejected indistinguishably from an
// invalid code (BE-A-20，与 verifications.consumeVerificationCode 同口径)。
// Callers without tenant context pass "".
//
// Failed matches count against the recipient budget and an exhausted budget
// refuses matching before touching the repository（「失败 3 次即失效该码」，
// functional §1.2）；成功匹配即清零。
func matchVerificationCode(ctx context.Context, repo verificationsdomain.Repository,
	organizationID, recipient, verificationType, code string) (*verificationsdomain.Verification, error) {
	if verificationAttemptLimiter != nil && !verificationAttemptLimiter.CheckAttempt(verificationType, recipient) {
		return nil, apperrors.NewTooManyRequests("too many failed attempts, please request a new code")
	}

	verification, err := repo.GetActiveByRecipientAndCode(ctx, recipient, code, verificationType)
	if err != nil {
		verification, err = appverifications.MatchActiveVerificationCode(ctx, repo, recipient, verificationType, code)
		if err != nil {
			recordVerificationFailure(verificationType, recipient)
			return nil, err
		}
	}

	if organizationID != "" && !strings.EqualFold(verification.OrganizationID, organizationID) {
		recordVerificationFailure(verificationType, recipient)
		return nil, apperrors.NewNotFound("verification not found or expired")
	}

	if verificationAttemptLimiter != nil {
		verificationAttemptLimiter.ResetAttempts(verificationType, recipient)
	}
	return verification, nil
}

// VerificationAttemptLimiter mirrors the recipient-level failure budget owned
// by the verifications service. *verifications.RateLimiter satisfies it;
// bootstrap wires the same instance the service-side consume path uses so both
// sides share one budget.
type VerificationAttemptLimiter interface {
	CheckAttempt(verificationType, recipient string) bool
	RecordFailedAttempt(verificationType, recipient string)
	ResetAttempts(verificationType, recipient string)
}

var verificationAttemptLimiter VerificationAttemptLimiter

// WireVerificationAttemptLimiter connects the code failure budget to the
// verification service's rate limiter. Call this once during bootstrap.
func WireVerificationAttemptLimiter(limiter VerificationAttemptLimiter) {
	verificationAttemptLimiter = limiter
}

func recordVerificationFailure(verificationType, recipient string) {
	if verificationAttemptLimiter != nil {
		verificationAttemptLimiter.RecordFailedAttempt(verificationType, recipient)
	}
}

// verificationMatchError keeps an exhausted failure budget as 429 so the
// client learns to request a fresh code; every other match failure stays the
// generic invalid-code answer（不泄露失败原因差异）。
func verificationMatchError(err error, message string) error {
	if apperrors.FromError(err).Code == apperrors.CodeTooManyRequests {
		return err
	}
	return apperrors.NewInvalidArgument(message)
}

// verifyEmailCodeWithRepo performs the actual email verification using the repository.
func verifyEmailCodeWithRepo(ctx context.Context, repo verificationsdomain.Repository, dest, passcode string) error {
	verification, err := matchVerificationCode(ctx, repo, "", strings.TrimSpace(dest), verificationTypeEmail, strings.TrimSpace(passcode))
	if err != nil {
		return verificationMatchError(err, "email verification code is invalid or expired")
	}
	return repo.UpdateStatus(ctx, verification.ID, "consumed")
}

// verifySmsCodeWithRepo performs the actual SMS verification using the repository.
// Recipients are matched as E.164. `countryCode` is only a legacy dial-prefix
// hint; an ISO 3166-1 alpha-2 value is rejected rather than concatenated into
// the recipient (the old `CN138...` bug).
func verifySmsCodeWithRepo(ctx context.Context, repo verificationsdomain.Repository, phone, countryCode, passcode string) error {
	recipient, err := phonefmt.ToE164(phone, countryCode)
	if err != nil {
		return err
	}
	verification, err := matchVerificationCode(ctx, repo, "", recipient, verificationTypePhone, strings.TrimSpace(passcode))
	if err != nil {
		return verificationMatchError(err, "sms verification code is invalid or expired")
	}
	return repo.UpdateStatus(ctx, verification.ID, "consumed")
}

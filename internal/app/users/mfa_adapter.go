package users

import (
	"context"
	"strings"

	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/phonefmt"
)

var (
	smsSetupVerify = func(secret, countryCode, passcode string) error {
		_ = secret
		_ = countryCode
		_ = passcode
		return apperrors.NewUnimplemented("sms mfa setup verify is not yet implemented: verification system migration pending")
	}
	emailSetupVerify = func(secret, passcode string) error {
		_ = secret
		_ = passcode
		return apperrors.NewUnimplemented("email mfa setup verify is not yet implemented: verification system migration pending")
	}
)

// WireMfaVerification connects the MFA setup verification function variables
// to the repository-based implementations. Call this once during bootstrap.
func WireMfaVerification(getRepo func() verificationsdomain.Repository) {
	smsSetupVerify = func(secret, countryCode, passcode string) error {
		ctx := context.Background()
		recipient, err := phonefmt.ToE164(secret, countryCode)
		if err != nil {
			return err
		}
		repo := getRepo()
		verification, err := appverifications.MatchActiveVerificationCode(ctx, repo, recipient, "phone", strings.TrimSpace(passcode))
		if err != nil {
			return apperrors.NewInvalidArgument("sms verification code is invalid or expired")
		}
		return repo.UpdateStatus(ctx, verification.ID, "consumed")
	}
	emailSetupVerify = func(secret, passcode string) error {
		ctx := context.Background()
		repo := getRepo()
		verification, err := appverifications.MatchActiveVerificationCode(ctx, repo, strings.TrimSpace(secret), mfaMethodEmail, strings.TrimSpace(passcode))
		if err != nil {
			return apperrors.NewInvalidArgument("email verification code is invalid or expired")
		}
		return repo.UpdateStatus(ctx, verification.ID, "consumed")
	}
}

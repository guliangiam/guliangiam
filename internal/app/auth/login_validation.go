package auth

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func validateLoginCommand(command LoginCommand) error {
	challengeID := strings.TrimSpace(command.ChallengeID)
	if challengeID == "" && strings.TrimSpace(command.OrganizationSlug) == "" {
		return apperrors.NewInvalidArgument("organizationSlug is required")
	}
	if challengeID == "" && strings.TrimSpace(command.Username) == "" {
		return apperrors.NewInvalidArgument("username is required")
	}
	verificationCode := strings.TrimSpace(command.VerificationCode)
	verificationType := strings.TrimSpace(command.VerificationType)
	if challengeID == "" {
		if verificationCode != "" {
			// One-time-code login is an alternative first factor: a request may
			// carry either password or a verification code, never both.
			if strings.TrimSpace(command.Password) != "" {
				return apperrors.NewInvalidArgument("password and verificationCode cannot be provided together")
			}
			if !isLoginVerificationType(verificationType) {
				return apperrors.NewInvalidArgument("verificationType must be 'email' or 'sms'")
			}
		} else if strings.TrimSpace(command.Password) == "" {
			return apperrors.NewInvalidArgument("password is required")
		}
	} else if verificationCode != "" {
		return apperrors.NewInvalidArgument("challengeId and verificationCode cannot be provided together")
	}
	if challengeID == "" && strings.TrimSpace(command.ApplicationID) == "" && strings.TrimSpace(command.ResponseType) != "" {
		return apperrors.NewInvalidArgument("applicationId is required when responseType is set")
	}
	if strings.TrimSpace(command.Passcode) != "" && strings.TrimSpace(command.RecoveryCode) != "" {
		return apperrors.NewInvalidArgument("passcode and recoveryCode cannot be provided together")
	}
	isPushAttempt := strings.TrimSpace(command.MfaType) == contactPush
	// A remember-me token is a single-use substitute for the MFA passcode, so it
	// satisfies the same "second factor present" rules.
	hasRememberMeToken := strings.TrimSpace(command.RememberMeToken) != ""
	if challengeID != "" && strings.TrimSpace(command.Passcode) == "" && strings.TrimSpace(command.RecoveryCode) == "" && !isPushAttempt && !hasRememberMeToken {
		return apperrors.NewInvalidArgument("challengeId requires passcode or recoveryCode")
	}
	if strings.TrimSpace(command.MfaType) != "" && strings.TrimSpace(command.Passcode) == "" && !isPushAttempt && !hasRememberMeToken {
		return apperrors.NewInvalidArgument("mfaType requires passcode")
	}
	if command.EnableMfaRemember && strings.TrimSpace(command.Passcode) == "" && strings.TrimSpace(command.RecoveryCode) == "" && !isPushAttempt && !hasRememberMeToken {
		return apperrors.NewInvalidArgument("enableMfaRemember requires passcode or recoveryCode")
	}
	return nil
}

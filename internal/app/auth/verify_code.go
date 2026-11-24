package auth

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// VerifyCodeCommand is the input for standalone verification code checking.
type VerifyCodeCommand struct {
	Dest        string `json:"dest"`
	Code        string `json:"code"`
	Type        string `json:"type"` // "email" or "sms"
	Phone       string `json:"phone,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
}

// VerifyCode checks a verification code independently (not tied to a login flow).
func (s *Service) VerifyCode(ctx context.Context, cmd VerifyCodeCommand) error {
	switch cmd.Type {
	case contactEmail:
		if err := verifyEmailCode(ctx, cmd.Dest, cmd.Code); err != nil {
			return err
		}
	case contactSMS:
		if err := verifySmsCode(ctx, cmd.Phone, cmd.CountryCode, cmd.Code); err != nil {
			return err
		}
	default:
		return apperrors.NewInvalidArgument("verification type must be \"email\" or \"sms\"")
	}
	return nil
}

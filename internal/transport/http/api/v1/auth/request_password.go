package auth

import (
	"unicode/utf8"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ResetPasswordRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
	VerificationCode string `json:"verificationCode"`
	NewPassword      string `json:"newPassword"`
	CaptchaToken     string `json:"captchaToken,omitempty"`
	CaptchaID        string `json:"captchaId,omitempty"`
}

type CheckPasswordRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
	Password         string `json:"password"`
}

type SignupRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Email            string `json:"email,omitempty"`
	Phone            string `json:"phone,omitempty"`
	SignupAppID      string `json:"signupAppId,omitempty"`
	VerificationCode string `json:"verificationCode,omitempty"`
	InvitationCode   string `json:"invitationCode,omitempty"`
	CaptchaToken     string `json:"captchaToken,omitempty"`
	CaptchaID        string `json:"captchaId,omitempty"`
}

func (r ResetPasswordRequest) Validate() error {
	if r.NewPassword != "" && utf8.RuneCountInString(r.NewPassword) > maxPasswordLen {
		return apperrors.NewInvalidArgument("newPassword exceeds maximum length")
	}
	if err := validateOptionalString("organizationSlug", r.OrganizationSlug); err != nil {
		return err
	}
	if err := validateOptionalString("username", r.Username); err != nil {
		return err
	}
	if err := validateOptionalString("verificationCode", r.VerificationCode); err != nil {
		return err
	}
	if err := validateOptionalString("captchaToken", r.CaptchaToken); err != nil {
		return err
	}
	return validateOptionalString("captchaId", r.CaptchaID)
}

func (r ResetPasswordRequest) toCommand() appauth.ResetPasswordCommand {
	return appauth.ResetPasswordCommand{
		OrganizationSlug: r.OrganizationSlug,
		Username:         r.Username,
		VerificationCode: r.VerificationCode,
		NewPassword:      r.NewPassword,
		CaptchaToken:     r.CaptchaToken,
		CaptchaID:        r.CaptchaID,
	}
}

func (r CheckPasswordRequest) Validate() error {
	if r.Password != "" && utf8.RuneCountInString(r.Password) > maxPasswordLen {
		return apperrors.NewInvalidArgument("password exceeds maximum length")
	}
	if err := validateOptionalString("organizationSlug", r.OrganizationSlug); err != nil {
		return err
	}
	if err := validateOptionalString("username", r.Username); err != nil {
		return err
	}
	return nil
}

func (r CheckPasswordRequest) toCommand() appauth.CheckPasswordCommand {
	return appauth.CheckPasswordCommand{
		OrganizationSlug: r.OrganizationSlug,
		Username:         r.Username,
		Password:         r.Password,
	}
}

type ChangePasswordRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
	OldPassword      string `json:"oldPassword"`
	NewPassword      string `json:"newPassword"`
}

func (r ChangePasswordRequest) Validate() error {
	// organizationSlug/username are session-derived: the handler overwrites
	// both from SessionContext before calling the app layer, and a browser
	// caller has no trustworthy values to send. Requiring them here rejected
	// every self-service change request, which is fatal once the password gate
	// forces the user onto /account (the change call is their only way out).
	if r.OldPassword == "" {
		return apperrors.NewInvalidArgument("oldPassword is required")
	}
	if r.NewPassword == "" {
		return apperrors.NewInvalidArgument("newPassword is required")
	}
	if len(r.NewPassword) > 512 {
		return apperrors.NewInvalidArgument("new password exceeds maximum length")
	}
	return nil
}

func (r ChangePasswordRequest) toCommand() appauth.ChangePasswordCommand {
	return appauth.ChangePasswordCommand{
		OrganizationSlug: r.OrganizationSlug,
		Username:         r.Username,
		OldPassword:      r.OldPassword,
		NewPassword:      r.NewPassword,
	}
}

func (r SignupRequest) Validate() error {
	if r.Password != "" && utf8.RuneCountInString(r.Password) > maxPasswordLen {
		return apperrors.NewInvalidArgument("password exceeds maximum length")
	}
	if err := validateOptionalString("organizationSlug", r.OrganizationSlug); err != nil {
		return err
	}
	if err := validateOptionalString("username", r.Username); err != nil {
		return err
	}
	if err := validateOptionalString("email", r.Email); err != nil {
		return err
	}
	if err := validateOptionalString("phone", r.Phone); err != nil {
		return err
	}
	if err := validateOptionalString("signupAppId", r.SignupAppID); err != nil {
		return err
	}
	if err := validateOptionalString("verificationCode", r.VerificationCode); err != nil {
		return err
	}
	if err := validateOptionalString("captchaToken", r.CaptchaToken); err != nil {
		return err
	}
	return validateOptionalString("captchaId", r.CaptchaID)
}

func (r SignupRequest) toCommand() appauth.SignupCommand {
	return appauth.SignupCommand{
		OrganizationSlug: r.OrganizationSlug,
		Username:         r.Username,
		Password:         r.Password,
		Email:            r.Email,
		Phone:            r.Phone,
		SignupAppID:      r.SignupAppID,
		VerificationCode: r.VerificationCode,
		InvitationCode:   r.InvitationCode,
		CaptchaToken:     r.CaptchaToken,
		CaptchaID:        r.CaptchaID,
	}
}

type VerifyCodeRequest struct {
	Dest        string `json:"dest"`
	Code        string `json:"code"`
	Type        string `json:"type"`
	Phone       string `json:"phone,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
}

func (r VerifyCodeRequest) Validate() error {
	if r.Type != "email" && r.Type != "sms" {
		return apperrors.NewInvalidArgument("type must be \"email\" or \"sms\"")
	}
	if r.Code == "" {
		return apperrors.NewInvalidArgument("code is required")
	}
	if r.Type == "email" && r.Dest == "" {
		return apperrors.NewInvalidArgument("dest is required for email verification")
	}
	if r.Type == "sms" && r.Phone == "" {
		return apperrors.NewInvalidArgument("phone is required for sms verification")
	}
	return nil
}

func (r VerifyCodeRequest) toCommand() appauth.VerifyCodeCommand {
	return appauth.VerifyCodeCommand{
		Dest:        r.Dest,
		Code:        r.Code,
		Type:        r.Type,
		Phone:       r.Phone,
		CountryCode: r.CountryCode,
	}
}

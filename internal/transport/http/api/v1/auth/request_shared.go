package auth

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

type GetAccountRequest struct {
	SessionID        string
	OrganizationSlug string
	Username         string
	AccessToken      string
	AuthTime         time.Time
	Sid              string
}

type LoginRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	ApplicationID    string `json:"applicationId"`
	ChallengeID      string `json:"challengeId,omitempty"`
	MfaType          string `json:"mfaType,omitempty"`
	Passcode         string `json:"passcode,omitempty"`
	RecoveryCode     string `json:"recoveryCode,omitempty"`
	VerificationCode string `json:"verificationCode,omitempty"`
	VerificationType string `json:"verificationType,omitempty"`
	// CaptchaToken/CaptchaID carry the human-check response when the
	// organization enables a captcha provider (server-side enforced in
	// app/auth EnforceCaptcha, same contract as signup).
	CaptchaToken string `json:"captchaToken,omitempty"`
	CaptchaID    string `json:"captchaId,omitempty"`
	// RememberMeToken is optional: a valid token substitutes the MFA passcode
	// and is rotated server-side.
	RememberMeToken   string   `json:"rememberMeToken,omitempty"`
	EnableMfaRemember bool     `json:"enableMfaRemember,omitempty"`
	ResponseType      string   `json:"responseType,omitempty"`
	RedirectURI       string   `json:"redirectUri,omitempty"`
	State             string   `json:"state,omitempty"`
	Nonce             string   `json:"nonce,omitempty"`
	Scope             []string `json:"scope,omitempty"`
	CodeChallenge     string   `json:"codeChallenge,omitempty"`
	ChallengeMethod   string   `json:"codeChallengeMethod,omitempty"`
	Resource          string   `json:"resource,omitempty"`
	SAMLRequest       string   `json:"samlRequest,omitempty"`
	RelayState        string   `json:"relayState,omitempty"`
	CASService        string   `json:"casService,omitempty"`
}

type NativeSsoCompleteRequest struct {
	AccessToken   string `json:"accessToken"`
	ApplicationID string `json:"applicationId"`
}

func (r GetAccountRequest) toQuery() appauth.AccountQuery {
	return appauth.AccountQuery{
		OrganizationSlug: r.OrganizationSlug,
		Username:         r.Username,
		AccessToken:      r.AccessToken,
	}
}

type UpdateAccountRequest struct {
	DisplayName      *string `json:"displayName,omitempty"`
	AvatarURL        *string `json:"avatarUrl,omitempty"`
	Email            *string `json:"email,omitempty"`
	Phone            *string `json:"phone,omitempty"`
	VerificationCode string  `json:"verificationCode,omitempty"`
}

func (r UpdateAccountRequest) toCommand(session GetAccountRequest) appauth.UpdateAccountCommand {
	return appauth.UpdateAccountCommand{
		OrganizationSlug: session.OrganizationSlug,
		Username:         session.Username,
		AccessToken:      session.AccessToken,
		DisplayName:      r.DisplayName,
		AvatarURL:        r.AvatarURL,
		Email:            r.Email,
		Phone:            r.Phone,
		VerificationCode: r.VerificationCode,
	}
}

func requestFromHTTPRequest(r *http.Request) GetAccountRequest {
	// Fail-close：身份**只能**从 SessionContextMiddleware 注入的 session context 取。
	// 历史上曾有 fallback 读 X-Username / X-Organization-Slug Header，
	// 攻击者可自行伪造这些 Header 冒充任意用户（PEP fail-close 审计 2026-09-24）。
	session, ok := SessionContextFromContext(r.Context())
	if !ok {
		// No session context → 返回空结构体，handler 层应据此返回 401。
		return GetAccountRequest{}
	}
	return GetAccountRequest{
		SessionID:        session.ID,
		OrganizationSlug: session.OrganizationSlug,
		Username:         session.Username,
		AccessToken:      session.AccessToken,
		AuthTime:         session.AuthTime,
		Sid:              session.Sid,
	}
}

func (r LoginRequest) Validate() error {
	// Required-field checks are handled by validateLoginCommand in the application layer.
	// Transport-layer validation focuses on length bounds and format constraints only.
	if r.Password != "" && utf8.RuneCountInString(r.Password) > maxPasswordLen {
		return apperrors.NewInvalidArgument("password exceeds maximum length")
	}
	if err := validateOptionalString("organizationSlug", r.OrganizationSlug); err != nil {
		return err
	}
	if err := validateOptionalString("username", r.Username); err != nil {
		return err
	}
	if err := validateOptionalString("applicationId", r.ApplicationID); err != nil {
		return err
	}
	if err := validateOptionalString("challengeId", r.ChallengeID); err != nil {
		return err
	}
	if err := validateOptionalString("mfaType", r.MfaType); err != nil {
		return err
	}
	if r.Passcode != "" && utf8.RuneCountInString(r.Passcode) > maxStringLength {
		return apperrors.NewInvalidArgument("passcode exceeds maximum length")
	}
	if r.RecoveryCode != "" && utf8.RuneCountInString(r.RecoveryCode) > maxStringLength {
		return apperrors.NewInvalidArgument("recoveryCode exceeds maximum length")
	}
	if r.VerificationCode != "" && utf8.RuneCountInString(r.VerificationCode) > maxStringLength {
		return apperrors.NewInvalidArgument("verificationCode exceeds maximum length")
	}
	if err := validateOptionalString("rememberMeToken", r.RememberMeToken); err != nil {
		return err
	}
	if err := validateOptionalString("verificationType", r.VerificationType); err != nil {
		return err
	}
	if err := validateOptionalString("captchaToken", r.CaptchaToken); err != nil {
		return err
	}
	if err := validateOptionalString("captchaId", r.CaptchaID); err != nil {
		return err
	}
	if err := validateOptionalString("responseType", r.ResponseType); err != nil {
		return err
	}
	if err := validateOptionalString("redirectUri", r.RedirectURI); err != nil {
		return err
	}
	if err := validateOptionalString("samlRequest", r.SAMLRequest); err != nil {
		return err
	}
	if err := validateOptionalString("casService", r.CASService); err != nil {
		return err
	}
	return validateStringSlice("scope", r.Scope)
}

func (r LoginRequest) toCommand(httpRequest *http.Request) appauth.LoginCommand {
	return appauth.LoginCommand{
		OrganizationSlug:          r.OrganizationSlug,
		Username:                  r.Username,
		Password:                  r.Password,
		ApplicationID:             r.ApplicationID,
		ChallengeID:               r.ChallengeID,
		MfaType:                   r.MfaType,
		Passcode:                  r.Passcode,
		RecoveryCode:              r.RecoveryCode,
		VerificationCode:          r.VerificationCode,
		VerificationType:          r.VerificationType,
		CaptchaToken:              strings.TrimSpace(r.CaptchaToken),
		CaptchaID:                 strings.TrimSpace(r.CaptchaID),
		TrustedDeviceCredentialID: TrustedDeviceCredentialFromRequest(httpRequest),
		RememberMeToken:           strings.TrimSpace(r.RememberMeToken),
		EnableMfaRemember:         r.EnableMfaRemember,
		IPAddress:                 clientIPFromRequest(httpRequest),
		UserAgent:                 strings.TrimSpace(httpRequest.UserAgent()),
		ResponseType:              r.ResponseType,
		RedirectURI:               r.RedirectURI,
		State:                     r.State,
		Nonce:                     r.Nonce,
		Scope:                     append([]string(nil), r.Scope...),
		CodeChallenge:             r.CodeChallenge,
		ChallengeMethod:           r.ChallengeMethod,
		Resource:                  r.Resource,
		SAMLRequest:               r.SAMLRequest,
		RelayState:                r.RelayState,
		CASService:                r.CASService,
		PreviousSessionID:         ExtractPreviousSessionIDFromRequest(httpRequest),
	}
}

func clientIPFromRequest(r *http.Request) string {
	return util.ClientIP(r)
}

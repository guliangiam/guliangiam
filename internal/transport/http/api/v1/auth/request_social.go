package auth

import (
	"net/http"
	"strings"
	"unicode/utf8"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type SocialLoginRequest struct {
	ProviderID       string   `json:"providerId"`
	Code             string   `json:"code"`
	State            string   `json:"state,omitempty"`
	CodeVerifier     string   `json:"codeVerifier,omitempty"`
	OrganizationSlug string   `json:"organizationSlug,omitempty"`
	ApplicationID    string   `json:"applicationId"`
	ResponseType     string   `json:"responseType,omitempty"`
	RedirectURI      string   `json:"redirectUri,omitempty"`
	Nonce            string   `json:"nonce,omitempty"`
	Scope            []string `json:"scope,omitempty"`
}

func (r SocialLoginRequest) Validate() error {
	if utf8.RuneCountInString(r.ProviderID) == 0 {
		return apperrors.NewInvalidArgument("providerId is required")
	}
	if utf8.RuneCountInString(r.Code) == 0 {
		return apperrors.NewInvalidArgument("code is required")
	}
	if err := validateOptionalString("providerId", r.ProviderID); err != nil {
		return err
	}
	if err := validateOptionalString("code", r.Code); err != nil {
		return err
	}
	if len(r.CodeVerifier) > 128 {
		return apperrors.NewInvalidArgument("codeVerifier exceeds maximum length (128 chars)")
	}
	if len(r.CodeVerifier) > 0 && len(r.CodeVerifier) < 43 {
		return apperrors.NewInvalidArgument("codeVerifier too short (min 43 chars for PKCE)")
	}
	if err := validateOptionalString("codeVerifier", r.CodeVerifier); err != nil {
		return err
	}
	if err := validateOptionalString("organizationSlug", r.OrganizationSlug); err != nil {
		return err
	}
	if err := validateOptionalString("applicationId", r.ApplicationID); err != nil {
		return err
	}
	return validateStringSlice("scope", r.Scope)
}

func (r SocialLoginRequest) toCommand(httpRequest *http.Request) appauth.SocialLoginCommand {
	return appauth.SocialLoginCommand{
		ProviderID:        r.ProviderID,
		Code:              r.Code,
		State:             r.State,
		CodeVerifier:      r.CodeVerifier,
		OrganizationSlug:  r.OrganizationSlug,
		ApplicationID:     r.ApplicationID,
		IPAddress:         clientIPFromRequest(httpRequest),
		UserAgent:         strings.TrimSpace(httpRequest.UserAgent()),
		ResponseType:      r.ResponseType,
		RedirectURI:       r.RedirectURI,
		Nonce:             r.Nonce,
		Scope:             append([]string(nil), r.Scope...),
		PreviousSessionID: ExtractPreviousSessionIDFromRequest(httpRequest),
	}
}

type SocialAuthorizeRequest struct {
	ProviderID       string   `json:"providerId"`
	OrganizationSlug string   `json:"organizationSlug,omitempty"`
	ApplicationID    string   `json:"applicationId,omitempty"`
	ResponseType     string   `json:"responseType,omitempty"`
	RedirectURI      string   `json:"redirectUri,omitempty"`
	State            string   `json:"state,omitempty"`
	Nonce            string   `json:"nonce,omitempty"`
	Scope            []string `json:"scope,omitempty"`
}

func (r SocialAuthorizeRequest) toCommand() appauth.SocialAuthorizeCommand {
	return appauth.SocialAuthorizeCommand{
		ProviderID:       r.ProviderID,
		OrganizationSlug: r.OrganizationSlug,
		ApplicationID:    r.ApplicationID,
		ResponseType:     r.ResponseType,
		RedirectURI:      r.RedirectURI,
		State:            r.State,
		Nonce:            r.Nonce,
		Scope:            append([]string(nil), r.Scope...),
	}
}

type LdapLoginRequest struct {
	OrganizationSlug string   `json:"organizationSlug"`
	Username         string   `json:"username"`
	Password         string   `json:"password"`
	LdapID           string   `json:"ldapId,omitempty"`
	ApplicationID    string   `json:"applicationId"`
	CaptchaToken     string   `json:"captchaToken,omitempty"`
	CaptchaID        string   `json:"captchaId,omitempty"`
	ResponseType     string   `json:"responseType,omitempty"`
	RedirectURI      string   `json:"redirectUri,omitempty"`
	State            string   `json:"state,omitempty"`
	Nonce            string   `json:"nonce,omitempty"`
	Scope            []string `json:"scope,omitempty"`
}

func (r LdapLoginRequest) Validate() error {
	if utf8.RuneCountInString(r.OrganizationSlug) == 0 {
		return apperrors.NewInvalidArgument("organizationSlug is required")
	}
	if utf8.RuneCountInString(r.Username) == 0 {
		return apperrors.NewInvalidArgument("username is required")
	}
	if r.Password != "" && utf8.RuneCountInString(r.Password) > maxPasswordLen {
		return apperrors.NewInvalidArgument("password exceeds maximum length")
	}
	if err := validateOptionalString("ldapId", r.LdapID); err != nil {
		return err
	}
	if err := validateOptionalString("applicationId", r.ApplicationID); err != nil {
		return err
	}
	if err := validateOptionalString("captchaToken", r.CaptchaToken); err != nil {
		return err
	}
	if err := validateOptionalString("captchaId", r.CaptchaID); err != nil {
		return err
	}
	return validateStringSlice("scope", r.Scope)
}

func (r LdapLoginRequest) toCommand(httpRequest *http.Request) appauth.LdapLoginCommand {
	return appauth.LdapLoginCommand{
		OrganizationSlug:  r.OrganizationSlug,
		Username:          r.Username,
		Password:          r.Password,
		LdapID:            r.LdapID,
		ApplicationID:     r.ApplicationID,
		CaptchaToken:      strings.TrimSpace(r.CaptchaToken),
		CaptchaID:         strings.TrimSpace(r.CaptchaID),
		IPAddress:         clientIPFromRequest(httpRequest),
		UserAgent:         strings.TrimSpace(httpRequest.UserAgent()),
		ResponseType:      r.ResponseType,
		RedirectURI:       r.RedirectURI,
		State:             r.State,
		Nonce:             r.Nonce,
		Scope:             append([]string(nil), r.Scope...),
		PreviousSessionID: ExtractPreviousSessionIDFromRequest(httpRequest),
	}
}

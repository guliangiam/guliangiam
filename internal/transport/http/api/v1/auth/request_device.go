package auth

import (
	"net/http"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
)

type DeviceAuthorizeRequest struct {
	ClientID string `json:"clientId"`
	Scope    string `json:"scope,omitempty"`
}

type DeviceTokenRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret,omitempty"`
	DeviceCode   string `json:"deviceCode"`
}

type DeviceVerifyRequest struct {
	UserCode string `json:"userCode"`
}

type DeviceCancelRequest struct {
	DeviceCode string `json:"deviceCode"`
	ClientID   string `json:"clientId"`
}

func (r DeviceAuthorizeRequest) toCommand(httpRequest *http.Request) appauth.DeviceAuthorizeCommand {
	return appauth.DeviceAuthorizeCommand{
		ClientID:  r.ClientID,
		Scope:     r.Scope,
		IssuerURL: platformconfig.IssuerURL(httpRequest),
	}
}

func (r DeviceTokenRequest) toCommand() appauth.DeviceTokenCommand {
	return appauth.DeviceTokenCommand{
		ClientID:     r.ClientID,
		ClientSecret: r.ClientSecret,
		DeviceCode:   r.DeviceCode,
	}
}

func (r DeviceVerifyRequest) toCommand(httpRequest *http.Request) appauth.DeviceVerifyCommand {
	actor := requestFromHTTPRequest(httpRequest)
	return appauth.DeviceVerifyCommand{
		UserCode:       r.UserCode,
		OrganizationID: "",
		UserID:         "",
		Username:       actor.Username,
	}
}

func (r DeviceCancelRequest) toCommand() appauth.DeviceCancelCommand {
	return appauth.DeviceCancelCommand{
		DeviceCode: r.DeviceCode,
		ClientID:   r.ClientID,
	}
}

type ClientCredentialsRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Scope        string `json:"scope,omitempty"`
}

func (r ClientCredentialsRequest) Validate() error {
	if err := validateOptionalString("clientId", r.ClientID); err != nil {
		return err
	}
	if err := validateOptionalString("clientSecret", r.ClientSecret); err != nil {
		return err
	}
	return validateOptionalString("scope", r.Scope)
}

func (r ClientCredentialsRequest) toCommand(httpRequest *http.Request) appauth.ExchangeClientCredentialsCommand {
	return appauth.ExchangeClientCredentialsCommand{
		ClientID:         r.ClientID,
		ClientSecret:     r.ClientSecret,
		Scope:            r.Scope,
		PeerCertificates: platformmtls.PeerCertificatesFromRequest(httpRequest),
	}
}

package auth

import (
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

type CreateQRCodeRequest struct {
	OrganizationID string `json:"organizationId"`
	ApplicationID  string `json:"applicationId"`
}

func (r CreateQRCodeRequest) toCommand() appauth.CreateQRCodeCommand {
	return appauth.CreateQRCodeCommand{
		OrganizationID: r.OrganizationID,
		ApplicationID:  r.ApplicationID,
	}
}

type ScanQRCodeRequest struct {
	Code        string `json:"code"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	IPAddress   string `json:"ipAddress,omitempty"`
	UserAgent   string `json:"userAgent,omitempty"`
}

func (r ScanQRCodeRequest) toCommand(sessionUsername string, sessionOrganizationID string) appauth.ScanQRCodeCommand {
	return appauth.ScanQRCodeCommand{
		Code:                  r.Code,
		Username:              r.Username,
		DisplayName:           r.DisplayName,
		ScannerUsername:       sessionUsername,
		ScannerOrganizationID: sessionOrganizationID,
		IPAddress:             r.IPAddress,
		UserAgent:             r.UserAgent,
	}
}

type ConfirmQRCodeRequest struct {
	Code     string `json:"code"`
	Approved bool   `json:"approved"`
}

func (r ConfirmQRCodeRequest) toCommand(sessionUsername string, sessionOrganizationID string) appauth.ConfirmQRCodeCommand {
	return appauth.ConfirmQRCodeCommand{
		Code:                  r.Code,
		ScannerUsername:       sessionUsername,
		ScannerOrganizationID: sessionOrganizationID,
		Approved:              r.Approved,
	}
}

type ExchangeQRCodeRequest struct {
	CodeID       string   `json:"codeId"`
	ResponseType string   `json:"responseType,omitempty"`
	RedirectURI  string   `json:"redirectUri,omitempty"`
	State        string   `json:"state,omitempty"`
	Nonce        string   `json:"nonce,omitempty"`
	Scope        []string `json:"scope,omitempty"`
}

func (r ExchangeQRCodeRequest) toCommand() appauth.ExchangeQRCodeCommand {
	return appauth.ExchangeQRCodeCommand{
		CodeID:       r.CodeID,
		ResponseType: r.ResponseType,
		RedirectURI:  r.RedirectURI,
		State:        r.State,
		Nonce:        r.Nonce,
		Scope:        r.Scope,
	}
}

package auth

import (
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

type InitiateFaceIDRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
	ApplicationID    string `json:"applicationId"`
}

func (r InitiateFaceIDRequest) toCommand() appauth.InitiateFaceIDCommand {
	return appauth.InitiateFaceIDCommand{
		OrganizationSlug: r.OrganizationSlug,
		Username:         r.Username,
		ApplicationID:    r.ApplicationID,
	}
}

type VerifyFaceIDRequest struct {
	SessionID  string `json:"sessionId"`
	ImageData  string `json:"imageData"`
	DeviceInfo string `json:"deviceInfo,omitempty"`
}

func (r VerifyFaceIDRequest) toCommand() appauth.VerifyFaceIDCommand {
	return appauth.VerifyFaceIDCommand{
		SessionID:  r.SessionID,
		ImageData:  r.ImageData,
		DeviceInfo: r.DeviceInfo,
	}
}

type CompleteFaceIDLoginRequest struct {
	SessionID string `json:"sessionId"`
	// ApplicationID 是协议上下文的目标应用（授权页登录时由控制台给出）；
	// 带 responseType 却不带它会被服务端拦成 422。
	ApplicationID string   `json:"applicationId,omitempty"`
	ResponseType  string   `json:"responseType,omitempty"`
	RedirectURI   string   `json:"redirectUri,omitempty"`
	State         string   `json:"state,omitempty"`
	Nonce         string   `json:"nonce,omitempty"`
	Scope         []string `json:"scope,omitempty"`
}

func (r CompleteFaceIDLoginRequest) toCommand() appauth.CompleteFaceIDLoginCommand {
	return appauth.CompleteFaceIDLoginCommand{
		SessionID:     r.SessionID,
		ApplicationID: r.ApplicationID,
		ResponseType:  r.ResponseType,
		RedirectURI:   r.RedirectURI,
		State:         r.State,
		Nonce:         r.Nonce,
		Scope:         r.Scope,
	}
}

package auth

import (
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

type InitiatePushMfaRequest struct {
	OrganizationID string `json:"organizationId"`
	UserID         string `json:"userId"`
	ApplicationID  string `json:"applicationId"`
}

func (r InitiatePushMfaRequest) toCommand() appauth.InitiatePushMfaCommand {
	return appauth.InitiatePushMfaCommand{
		OrganizationID: r.OrganizationID,
		UserID:         r.UserID,
		ApplicationID:  r.ApplicationID,
	}
}

type VerifyPushMfaRequest struct {
	ChallengeID string `json:"challengeId"`
}

func (r VerifyPushMfaRequest) toCommand() appauth.VerifyPushMfaCommand {
	return appauth.VerifyPushMfaCommand{
		ChallengeID: r.ChallengeID,
	}
}

type EnablePushMfaRequest struct {
	UserID          string `json:"userId"`
	MfaPushReceiver string `json:"mfaPushReceiver"`
	MfaPushProvider string `json:"mfaPushProvider"`
}

func (r EnablePushMfaRequest) toCommand() appauth.EnablePushMfaCommand {
	return appauth.EnablePushMfaCommand{
		UserID:          r.UserID,
		MfaPushReceiver: r.MfaPushReceiver,
		MfaPushProvider: r.MfaPushProvider,
	}
}

type DisablePushMfaRequest struct {
	UserID string `json:"userId"`
}

func (r DisablePushMfaRequest) toCommand() appauth.DisablePushMfaCommand {
	return appauth.DisablePushMfaCommand{
		UserID: r.UserID,
	}
}

type WebAuthnRegistrationBeginRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
}

func (r WebAuthnRegistrationBeginRequest) toCommand() appauth.WebAuthnRegistrationBeginCommand {
	return appauth.WebAuthnRegistrationBeginCommand{
		OrganizationSlug: r.OrganizationSlug,
		Username:         r.Username,
	}
}

type WebAuthnRegistrationFinishRequest struct {
	OrganizationSlug string   `json:"organizationSlug"`
	Username         string   `json:"username"`
	Challenge        string   `json:"challenge"`
	CredentialID     string   `json:"credentialId"`
	PublicKey        string   `json:"publicKey"`
	AttestationType  string   `json:"attestationType"`
	Transports       []string `json:"transports,omitempty"`
}

func (r WebAuthnRegistrationFinishRequest) toCommand() appauth.WebAuthnRegistrationFinishCommand {
	return appauth.WebAuthnRegistrationFinishCommand{
		OrganizationSlug: r.OrganizationSlug,
		Username:         r.Username,
		Challenge:        r.Challenge,
		CredentialID:     r.CredentialID,
		PublicKey:        r.PublicKey,
		AttestationType:  r.AttestationType,
		Transports:       r.Transports,
	}
}

type WebAuthnAuthenticationBeginRequest struct {
	OrganizationSlug string `json:"organizationSlug"`
	Username         string `json:"username"`
}

func (r WebAuthnAuthenticationBeginRequest) toCommand() appauth.WebAuthnAuthenticationBeginCommand {
	return appauth.WebAuthnAuthenticationBeginCommand{
		OrganizationSlug: r.OrganizationSlug,
		Username:         r.Username,
	}
}

type WebAuthnAuthenticationFinishRequest struct {
	OrganizationSlug  string `json:"organizationSlug"`
	Username          string `json:"username"`
	Challenge         string `json:"challenge"`
	CredentialID      string `json:"credentialId"`
	Signature         string `json:"signature"`
	AuthenticatorData string `json:"authenticatorData"`
	ClientDataJSON    string `json:"clientDataJSON"`
}

func (r WebAuthnAuthenticationFinishRequest) toCommand() appauth.WebAuthnAuthenticationFinishCommand {
	return appauth.WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  r.OrganizationSlug,
		Username:          r.Username,
		Challenge:         r.Challenge,
		CredentialID:      r.CredentialID,
		Signature:         r.Signature,
		AuthenticatorData: r.AuthenticatorData,
		ClientDataJSON:    r.ClientDataJSON,
	}
}

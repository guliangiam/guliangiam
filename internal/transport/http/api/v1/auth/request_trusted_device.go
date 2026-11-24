package auth

import (
	"strings"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

type TrustedDeviceRegisterRequest struct {
	ChallengeID        string `json:"challengeId"`
	Nonce              string `json:"nonce"`
	DeviceCredentialID string `json:"deviceCredentialId"`
	DevicePublicKey    string `json:"devicePublicKey"`
	Signature          string `json:"signature"`
}

func (r TrustedDeviceRegisterRequest) toCommand(session SessionContext, ipAddress, userAgent string) appauth.TrustedDeviceRegisterCommand {
	return appauth.TrustedDeviceRegisterCommand{
		OrganizationID:     session.OrganizationID,
		UserID:             session.UserID,
		ChallengeID:        strings.TrimSpace(r.ChallengeID),
		Nonce:              strings.TrimSpace(r.Nonce),
		DeviceCredentialID: strings.TrimSpace(r.DeviceCredentialID),
		DevicePublicKey:    strings.TrimSpace(r.DevicePublicKey),
		Signature:          strings.TrimSpace(r.Signature),
		IPAddress:          ipAddress,
		UserAgent:          userAgent,
	}
}

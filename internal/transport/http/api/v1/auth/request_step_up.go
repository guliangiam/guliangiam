package auth

import (
	"strings"
	"time"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

type StepUpBeginRequest struct {
	OperationScope string `json:"operationScope"`
}

func (r StepUpBeginRequest) toCommand(session SessionContext) appauth.StepUpBeginCommand {
	return appauth.StepUpBeginCommand{
		OrganizationID: session.OrganizationID,
		UserID:         session.UserID,
		SessionID:      session.ID,
		OperationScope: strings.TrimSpace(r.OperationScope),
	}
}

type StepUpFinishRequest struct {
	ChallengeID    string `json:"challengeId"`
	OperationScope string `json:"operationScope"`
	// TOTP path.
	Passcode string `json:"passcode"`
	// Passkey path: the assertion is verified against the challenge issued by
	// /api/v1/auth/webauthn/authentication/begin. Mutually exclusive with passcode.
	WebAuthnChallengeID string `json:"webauthnChallengeId"`
	Challenge           string `json:"challenge"`
	CredentialID        string `json:"credentialId"`
	Signature           string `json:"signature"`
	AuthenticatorData   string `json:"authenticatorData"`
	ClientDataJSON      string `json:"clientDataJSON"`
}

func (r StepUpFinishRequest) toCommand(session SessionContext) appauth.StepUpFinishCommand {
	return appauth.StepUpFinishCommand{
		OrganizationID:      session.OrganizationID,
		UserID:              session.UserID,
		SessionID:           session.ID,
		ChallengeID:         strings.TrimSpace(r.ChallengeID),
		OperationScope:      strings.TrimSpace(r.OperationScope),
		Passcode:            strings.TrimSpace(r.Passcode),
		WebAuthnChallengeID: strings.TrimSpace(r.WebAuthnChallengeID),
		WebAuthnChallenge:   strings.TrimSpace(r.Challenge),
		CredentialID:        strings.TrimSpace(r.CredentialID),
		Signature:           strings.TrimSpace(r.Signature),
		AuthenticatorData:   strings.TrimSpace(r.AuthenticatorData),
		ClientDataJSON:      strings.TrimSpace(r.ClientDataJSON),
	}
}

type StepUpBeginResponse struct {
	ChallengeID    string    `json:"challengeId"`
	OperationScope string    `json:"operationScope"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

type StepUpFinishResponse struct {
	AssuranceLevel string    `json:"assuranceLevel"`
	SessionID      string    `json:"sessionId"`
	OperationScope string    `json:"operationScope"`
	ElevatedAt     time.Time `json:"elevatedAt"`
}

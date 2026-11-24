package webauthn

import (
	"time"
)

// CredentialStatus is the lifecycle state of a WebAuthn credential.
type CredentialStatus string

const (
	CredentialStatusActive      CredentialStatus = "active"
	CredentialStatusRevoked     CredentialStatus = "revoked"
	CredentialStatusDeactivated CredentialStatus = "deactivated"
)

// Credential represents a stored WebAuthn credential (passkey).
type Credential struct {
	ID              string
	UserID          string
	CredentialID    []byte
	PublicKey       []byte
	AttestationType string
	Transport       []string
	SignCount       int64
	Name            string
	AAGUID          string
	Status          CredentialStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ActiveCredentialIDs returns the raw credential IDs of active credentials,
// suitable for excludeCredentials in registration begin responses.
func ActiveCredentialIDs(credentials []*Credential) [][]byte {
	ids := make([][]byte, 0, len(credentials))
	for _, credential := range credentials {
		if credential == nil || credential.Status != CredentialStatusActive {
			continue
		}
		ids = append(ids, credential.CredentialID)
	}
	return ids
}

package webauthncredentials

import (
	"database/sql"
	"time"
)

// credentialRow 是 webauthn_credentials 表的行模型，
// JSON 列用 []byte 承接，mapper 层再反序列化。
type credentialRow struct {
	ID              string
	UserID          string
	CredentialID    []byte
	PublicKey       []byte
	AttestationType string
	Transport       []byte // JSON []string
	SignCount       int64
	Name            string
	AAGUID          string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// challengeRow 是 webauthn_challenges 表的行模型：organization_id 与 consumed_at
// 可空，用 sql.Null* 承接。
type challengeRow struct {
	ID             string
	ChallengeHash  string
	CeremonyType   string
	UserID         string
	OrganizationID sql.NullString
	RelyingPartyID string
	ExpiresAt      time.Time
	ConsumedAt     sql.NullTime
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

package authorizationcodes

import "time"

// authorizationCodeRow mirrors the SELECT in sql.go. It deliberately has no
// Code field: the table stores only code_hash, so a row read back can never
// yield the plaintext credential.
type authorizationCodeRow struct {
	ID               string
	UserID           string
	Username         string
	OrganizationID   string
	OrganizationSlug string
	ApplicationID    string
	ApplicationSlug  string
	RedirectURI      string
	Scopes           []string
	State            string
	Nonce            string
	CodeChallenge    string
	ChallengeMethod  string
	Resource         string
	Sid              string
	AuthTime         time.Time
	ExpiresAt        time.Time
	UsedAt           *time.Time
	CreatedAt        time.Time
}

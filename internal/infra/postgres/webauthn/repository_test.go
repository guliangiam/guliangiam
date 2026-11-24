package webauthn

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stretchr/testify/assert"
)

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	assert.NotNil(t, repo)
	assert.Nil(t, repo.db)
}

func TestRowToDomain_MapsStatus(t *testing.T) {
	cred := rowToDomain(credentialRow{
		ID:              "cred-1",
		UserID:          "user-1",
		CredentialID:    []byte{1, 2, 3},
		PublicKey:       []byte{4, 5, 6},
		AttestationType: "none",
		Status:          "revoked",
	}, pgtype.UUID{})
	assert.Equal(t, domain.CredentialStatusRevoked, cred.Status)
}

package webauthncredentials

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// rowToDomain 把表行模型转成 domain.Credential。
// Transport 是 JSON 列，需要 UnmarshalJSON。
func rowToDomain(row credentialRow) (*domain.Credential, error) {
	c := &domain.Credential{
		ID:              row.ID,
		UserID:          row.UserID,
		CredentialID:    row.CredentialID,
		PublicKey:       row.PublicKey,
		AttestationType: row.AttestationType,
		SignCount:       row.SignCount,
		Name:            row.Name,
		AAGUID:          row.AAGUID,
		Status:          domain.CredentialStatus(row.Status),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	if err := mysqlshared.UnmarshalJSON(row.Transport, &c.Transport); err != nil {
		return nil, err
	}
	if c.Transport == nil {
		c.Transport = []string{}
	}
	return c, nil
}

// challengeRowToDomain 把表行模型转成 domain.Challenge。
func challengeRowToDomain(row challengeRow) *domain.Challenge {
	c := &domain.Challenge{
		ID:             row.ID,
		ChallengeHash:  row.ChallengeHash,
		CeremonyType:   row.CeremonyType,
		UserID:         row.UserID,
		OrganizationID: row.OrganizationID.String,
		RelyingPartyID: row.RelyingPartyID,
		ExpiresAt:      row.ExpiresAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.ConsumedAt.Valid {
		consumedAt := row.ConsumedAt.Time
		c.ConsumedAt = &consumedAt
	}
	return c
}

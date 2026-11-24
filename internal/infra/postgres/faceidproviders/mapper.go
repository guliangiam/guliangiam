package faceidproviders

import (
	"errors"
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// faceIdProviderEncryptedMapper guards access_key_secret (A-6.1). The column
// keeps its historical VARCHAR(255) definition and now stores a v2 envelope
// bound to (table, column, row id) instead of the plaintext cloud secret.
var faceIdProviderEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	faceIdProviderTable                    = "guliangiam.face_id_providers"
	faceIdProviderColumnAccessKeySecret    = "access_key_secret"
	faceIdProviderAccessKeySecretMaxLength = 255
)

func faceIdProviderAAD(providerID string) crypto.AADContext {
	return crypto.AADContext{Table: faceIdProviderTable, Column: faceIdProviderColumnAccessKeySecret, PKValue: providerID}
}

func toFaceIdProvider(row faceIdProviderRow) (*domain.FaceIdProvider, error) {
	provider := &domain.FaceIdProvider{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		Name:            row.Name,
		ProviderType:    row.ProviderType,
		AccessKeyID:     row.AccessKeyID,
		AccessKeySecret: row.AccessKeySecret,
		Endpoint:        row.Endpoint,
		IsEnabled:       row.IsEnabled,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Revision:        row.Revision,
	}
	if err := shared.UnmarshalJSON(row.Config, &provider.Config); err != nil {
		return nil, err
	}
	if provider.Config == nil {
		provider.Config = map[string]any{}
	}

	secret, needsRotation, err := openFaceIdAccessKeySecret(row)
	if err != nil {
		return nil, err
	}
	provider.AccessKeySecret = secret
	provider.SecretNeedsRotation = needsRotation
	return provider, nil
}

// openFaceIdAccessKeySecret decrypts a sealed value. A stored value that is
// not an envelope is a pre-A-6.1 plaintext row: it is returned for compatibility
// but the caller must surface SecretNeedsRotation and an operator WARN. The
// WARN carries only row identifiers, never the secret or the ciphertext.
func openFaceIdAccessKeySecret(row faceIdProviderRow) (string, bool, error) {
	if row.AccessKeySecret == "" {
		return "", false, nil
	}

	plaintext, err := faceIdProviderEncryptedMapper.DecryptEnvelopeColumn(faceIdProviderAAD(row.ID), row.AccessKeySecret)
	if err == nil {
		return plaintext, false, nil
	}
	if !errors.Is(err, crypto.ErrNotEnvelope) {
		return "", false, apperrors.WrapInternalf(err, "decrypt face id provider access_key_secret")
	}

	// A v0 direct-KEK token is ciphertext, not plaintext. Open it so the row
	// keeps working, but still mark it for rotation. Plaintext fails this
	// attempt (it is not a decryptable legacy token) and takes the fallback.
	if legacy, legacyErr := faceIdProviderEncryptedMapper.DecryptColumn(faceIdProviderAAD(row.ID), row.AccessKeySecret); legacyErr == nil {
		slog.Warn("face id provider access_key_secret is a legacy v0 envelope; run `migrate encrypt-faceid-secrets`",
			"provider_id", row.ID, "organization_id", row.OrganizationID)
		return legacy, true, nil
	}

	slog.Warn("face id provider access_key_secret is stored in plaintext; run `migrate encrypt-faceid-secrets`",
		"provider_id", row.ID, "organization_id", row.OrganizationID)
	return row.AccessKeySecret, true, nil
}

// faceIdAccessKeySecretForWrite returns the value to persist without mutating
// the domain object. An empty secret stays empty ("not configured").
func faceIdAccessKeySecretForWrite(provider *domain.FaceIdProvider) (string, error) {
	if provider.AccessKeySecret == "" {
		return "", nil
	}
	sealed, err := faceIdProviderEncryptedMapper.EncryptColumn(faceIdProviderAAD(provider.ID), provider.AccessKeySecret)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt face id provider access_key_secret")
	}
	// The column is still VARCHAR(255). Refuse an oversized envelope instead of
	// letting the database silently fail the write; no value is logged.
	if len(sealed) > faceIdProviderAccessKeySecretMaxLength {
		return "", apperrors.NewInternalf(
			"encrypted face id provider access_key_secret is %d characters, exceeding the %d-character column",
			len(sealed), faceIdProviderAccessKeySecretMaxLength)
	}
	return sealed, nil
}

package faceidproviders

import (
	"errors"
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// faceIdProviderEncryptedMapper 负责 access_key_secret 列的信封加解密（A-6.1）。
// 列定义保持 VARCHAR(255)，库内改为绑定 (table, column, 行 id) 的 v2 信封。
var faceIdProviderEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	faceIdProviderTable                    = "guliangiam.face_id_providers"
	faceIdProviderColumnAccessKeySecret    = "access_key_secret"
	faceIdProviderAccessKeySecretMaxLength = 255
)

func faceIdProviderAAD(providerID string) crypto.AADContext {
	return crypto.AADContext{Table: faceIdProviderTable, Column: faceIdProviderColumnAccessKeySecret, PKValue: providerID}
}

// rowToDomain 把表行模型转成 domain.FaceIdProvider；access_key_secret 在这里解密。
func rowToDomain(row faceIdProviderRow) (*domain.FaceIdProvider, error) {
	p := &domain.FaceIdProvider{
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
	if err := mysqlshared.UnmarshalJSON(row.Config, &p.Config); err != nil {
		return nil, err
	}
	if p.Config == nil {
		p.Config = map[string]any{}
	}

	secret, needsRotation, err := openFaceIdAccessKeySecret(row)
	if err != nil {
		return nil, err
	}
	p.AccessKeySecret = secret
	p.SecretNeedsRotation = needsRotation
	return p, nil
}

// openFaceIdAccessKeySecret 解密信封。非信封值按 A-6.1 之前的存量明文兼容读取，
// 但必须标记轮换并写结构化 WARN；WARN 只包含行标识，不含明文或密文。
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

	// v0 是密文而非明文：先尝试 legacy 解密，成功后仍标记轮换。
	if legacy, legacyErr := faceIdProviderEncryptedMapper.DecryptColumn(faceIdProviderAAD(row.ID), row.AccessKeySecret); legacyErr == nil {
		slog.Warn("face id provider access_key_secret is a legacy v0 envelope; run `migrate encrypt-faceid-secrets`",
			"provider_id", row.ID, "organization_id", row.OrganizationID)
		return legacy, true, nil
	}

	slog.Warn("face id provider access_key_secret is stored in plaintext; run `migrate encrypt-faceid-secrets`",
		"provider_id", row.ID, "organization_id", row.OrganizationID)
	return row.AccessKeySecret, true, nil
}

// sealAccessKeySecret 返回持久化值，不改 domain 对象；空值保持空值（未配置）。
func sealAccessKeySecret(provider *domain.FaceIdProvider) (string, error) {
	if provider.AccessKeySecret == "" {
		return "", nil
	}
	sealed, err := faceIdProviderEncryptedMapper.EncryptColumn(faceIdProviderAAD(provider.ID), provider.AccessKeySecret)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt face id provider access_key_secret")
	}
	if len(sealed) > faceIdProviderAccessKeySecretMaxLength {
		return "", apperrors.NewInternalf(
			"encrypted face id provider access_key_secret is %d characters, exceeding the %d-character column",
			len(sealed), faceIdProviderAccessKeySecretMaxLength)
	}
	return sealed, nil
}

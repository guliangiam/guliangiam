package certs

import domain "github.com/guliangiam/guliangiam/internal/domain/certs"

func toListItem(cert *domain.Cert) CertListItem {
	return CertListItem{
		ID:              cert.ID,
		OrganizationID:  cert.OrganizationID,
		Name:            cert.Name,
		DisplayName:     cert.DisplayName,
		Scope:           cert.Scope,
		Type:            cert.Type,
		CryptoAlgorithm: cert.CryptoAlgorithm,
		BitSize:         cert.BitSize,
		ExpireInYears:   cert.ExpireInYears,
		ExpireTime:      cert.ExpireTime,
		Provider:        cert.Provider,
		CreatedAt:       cert.CreatedAt,
		UpdatedAt:       cert.UpdatedAt,
	}
}

func toDetail(cert *domain.Cert) CertDetail {
	return CertDetail{
		CertListItem:           toListItem(cert),
		Account:                cert.Account,
		AccessKey:              cert.AccessKey,
		AccessSecretSet:        cert.AccessSecret != "",
		AccessSecretMasked:     maskSecret(cert.AccessSecret),
		AuthorityPublicKey:     cert.AuthorityPublicKey,
		AuthorityRootPublicKey: cert.AuthorityRootPublicKey,
		AcmeIssuer:             cert.AcmeIssuer,
		AcmeEndpoint:           cert.AcmeEndpoint,
		Domain:                 append([]string(nil), cert.Domain...),
		DomainExpireTime:       cert.DomainExpireTime,
		Certificate:            cert.Certificate,
		PrivateKeySet:          cert.PrivateKey != "",
		PrivateKeyMasked:       maskSecret(cert.PrivateKey),
		Revision:               cert.Revision,
	}
}

// maskSecret returns first 2 chars + *** + last 2 chars, or empty string.
// Short secrets (< 6 chars) return fully masked "******".
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) < 6 {
		return "******"
	}
	return s[:2] + "***" + s[len(s)-2:]
}

package certs

import "time"

type CertListItem struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organizationId"`
	Name            string     `json:"name"`
	DisplayName     string     `json:"displayName"`
	Scope           string     `json:"scope"`
	Type            string     `json:"type"`
	CryptoAlgorithm string     `json:"cryptoAlgorithm"`
	BitSize         int        `json:"bitSize"`
	ExpireInYears   int        `json:"expireInYears"`
	ExpireTime      *time.Time `json:"expireTime,omitempty"`
	Provider        string     `json:"provider,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type CertDetail struct {
	CertListItem
	Account                string     `json:"account,omitempty"`
	AccessKey              string     `json:"accessKey,omitempty"`
	AccessSecretSet        bool       `json:"accessSecretSet"`
	AccessSecretMasked     string     `json:"accessSecretMasked,omitempty"`
	AuthorityPublicKey     string     `json:"authorityPublicKey,omitempty"`
	AuthorityRootPublicKey string     `json:"authorityRootPublicKey,omitempty"`
	AcmeIssuer             string     `json:"acmeIssuer,omitempty"`
	AcmeEndpoint           string     `json:"acmeEndpoint,omitempty"`
	Domain                 []string   `json:"domain,omitempty"`
	DomainExpireTime       *time.Time `json:"domainExpireTime,omitempty"`
	Certificate            string     `json:"certificate,omitempty"`
	PrivateKeySet          bool       `json:"privateKeySet"`
	PrivateKeyMasked       string     `json:"privateKeyMasked,omitempty"`
	Revision               int64      `json:"revision"`
}

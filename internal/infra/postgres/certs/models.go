package certs

import "time"

type certRow struct {
	ID                     string
	OrganizationID         string
	Name                   string
	DisplayName            string
	Scope                  string
	Type                   string
	CryptoAlgorithm        string
	BitSize                int
	ExpireInYears          int
	ExpireTime             *time.Time
	Certificate            string
	PrivateKey             string
	AuthorityPublicKey     string
	AuthorityRootPublicKey string
	AcmeIssuer             string
	AcmeEndpoint           string
	Provider               string
	Account                string
	AccessKey              string
	AccessSecret           string
	Domain                 []byte
	DomainExpireTime       *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Revision               int64
}

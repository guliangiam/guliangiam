package certs

import (
	"context"
	"time"
)

type Cert struct {
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
	Domain                 []string
	DomainExpireTime       *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Revision               int64
}

type CertificateRenewer interface {
	ObtainCertificate(ctx context.Context, domain string) (certPEM []byte, keyPEM []byte, err error)
	RenewCertificate(ctx context.Context, domain string, certPEM []byte, keyPEM []byte) (newCertPEM []byte, newKeyPEM []byte, err error)
}

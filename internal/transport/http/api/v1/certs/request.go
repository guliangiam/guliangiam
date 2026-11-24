package certs

import appcerts "github.com/guliangiam/guliangiam/internal/app/certs"

type ListCertsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Scope          string
	Type           string
}

type GetCertRequest struct {
	ID string
}

type CreateCertRequest struct {
	OrganizationID         string   `json:"organizationId"`
	Name                   string   `json:"name"`
	DisplayName            string   `json:"displayName"`
	Scope                  string   `json:"scope,omitempty"`
	Type                   string   `json:"type,omitempty"`
	CryptoAlgorithm        string   `json:"cryptoAlgorithm,omitempty"`
	BitSize                int      `json:"bitSize,omitempty"`
	ExpireInYears          int      `json:"expireInYears,omitempty"`
	Certificate            string   `json:"certificate,omitempty"`
	PrivateKey             string   `json:"privateKey,omitempty"`
	AuthorityPublicKey     string   `json:"authorityPublicKey,omitempty"`
	AuthorityRootPublicKey string   `json:"authorityRootPublicKey,omitempty"`
	AcmeIssuer             string   `json:"acmeIssuer,omitempty"`
	AcmeEndpoint           string   `json:"acmeEndpoint,omitempty"`
	Provider               string   `json:"provider,omitempty"`
	Account                string   `json:"account,omitempty"`
	AccessKey              string   `json:"accessKey,omitempty"`
	AccessSecret           string   `json:"accessSecret,omitempty"`
	Domain                 []string `json:"domain,omitempty"`
}

type PatchCertRequest struct {
	DisplayName            *string  `json:"displayName,omitempty"`
	Scope                  *string  `json:"scope,omitempty"`
	Type                   *string  `json:"type,omitempty"`
	CryptoAlgorithm        *string  `json:"cryptoAlgorithm,omitempty"`
	BitSize                *int     `json:"bitSize,omitempty"`
	ExpireInYears          *int     `json:"expireInYears,omitempty"`
	Certificate            *string  `json:"certificate,omitempty"`
	PrivateKey             *string  `json:"privateKey,omitempty"`
	AuthorityPublicKey     *string  `json:"authorityPublicKey,omitempty"`
	AuthorityRootPublicKey *string  `json:"authorityRootPublicKey,omitempty"`
	AcmeIssuer             *string  `json:"acmeIssuer,omitempty"`
	AcmeEndpoint           *string  `json:"acmeEndpoint,omitempty"`
	Provider               *string  `json:"provider,omitempty"`
	Account                *string  `json:"account,omitempty"`
	AccessKey              *string  `json:"accessKey,omitempty"`
	AccessSecret           *string  `json:"accessSecret,omitempty"`
	Domain                 []string `json:"domain,omitempty"`
	Revision               int64    `json:"revision,omitempty"`
}

func (r CreateCertRequest) toCommand() appcerts.CreateCertCommand {
	return appcerts.CreateCertCommand{
		OrganizationID:         r.OrganizationID,
		Name:                   r.Name,
		DisplayName:            r.DisplayName,
		Scope:                  r.Scope,
		Type:                   r.Type,
		CryptoAlgorithm:        r.CryptoAlgorithm,
		BitSize:                r.BitSize,
		ExpireInYears:          r.ExpireInYears,
		Certificate:            r.Certificate,
		PrivateKey:             r.PrivateKey,
		AuthorityPublicKey:     r.AuthorityPublicKey,
		AuthorityRootPublicKey: r.AuthorityRootPublicKey,
		AcmeIssuer:             r.AcmeIssuer,
		AcmeEndpoint:           r.AcmeEndpoint,
		Provider:               r.Provider,
		Account:                r.Account,
		AccessKey:              r.AccessKey,
		AccessSecret:           r.AccessSecret,
		Domain:                 r.Domain,
	}
}

func (r PatchCertRequest) toCommand(id string) appcerts.PatchCertCommand {
	command := appcerts.PatchCertCommand{
		ID:                     id,
		DisplayName:            r.DisplayName,
		Scope:                  r.Scope,
		Type:                   r.Type,
		CryptoAlgorithm:        r.CryptoAlgorithm,
		BitSize:                r.BitSize,
		ExpireInYears:          r.ExpireInYears,
		Certificate:            r.Certificate,
		PrivateKey:             r.PrivateKey,
		AuthorityPublicKey:     r.AuthorityPublicKey,
		AuthorityRootPublicKey: r.AuthorityRootPublicKey,
		AcmeIssuer:             r.AcmeIssuer,
		AcmeEndpoint:           r.AcmeEndpoint,
		Provider:               r.Provider,
		Account:                r.Account,
		AccessKey:              r.AccessKey,
		AccessSecret:           r.AccessSecret,
		Revision:               r.Revision,
	}
	if r.Domain != nil {
		command.Domain = r.Domain
	}
	return command
}

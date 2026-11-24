package certs

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
)

type CreateCertCommand struct {
	OrganizationID         string
	Name                   string
	DisplayName            string
	Scope                  string
	Type                   string
	CryptoAlgorithm        string
	BitSize                int
	ExpireInYears          int
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
}

func (s Service) NewCert(command CreateCertCommand) (*domain.Cert, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()

	return &domain.Cert{
		ID:                     id,
		OrganizationID:         command.OrganizationID,
		Name:                   command.Name,
		DisplayName:            command.DisplayName,
		Scope:                  command.Scope,
		Type:                   command.Type,
		CryptoAlgorithm:        command.CryptoAlgorithm,
		BitSize:                command.BitSize,
		ExpireInYears:          command.ExpireInYears,
		Certificate:            command.Certificate,
		PrivateKey:             command.PrivateKey,
		AuthorityPublicKey:     command.AuthorityPublicKey,
		AuthorityRootPublicKey: command.AuthorityRootPublicKey,
		AcmeIssuer:             command.AcmeIssuer,
		AcmeEndpoint:           command.AcmeEndpoint,
		Provider:               command.Provider,
		Account:                command.Account,
		AccessKey:              command.AccessKey,
		AccessSecret:           command.AccessSecret,
		Domain:                 append([]string(nil), command.Domain...),
		CreatedAt:              now,
		UpdatedAt:              now,
		Revision:               1,
	}, nil
}

func (s Service) CreateCert(ctx context.Context, command CreateCertCommand) error {
	if err := validateCreateCertCommand(command); err != nil {
		return err
	}

	cert, err := s.NewCert(command)
	if err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Certs().Create(ctx, cert); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			cert.OrganizationID,
			"certs.created",
			"certs",
			cert.ID,
			"certs",
			"certs.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

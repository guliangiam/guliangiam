package certs

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchCertCommand struct {
	ID                     string
	OrganizationID         string
	DisplayName            *string
	Scope                  *string
	Type                   *string
	CryptoAlgorithm        *string
	BitSize                *int
	ExpireInYears          *int
	Certificate            *string
	PrivateKey             *string
	AuthorityPublicKey     *string
	AuthorityRootPublicKey *string
	AcmeIssuer             *string
	AcmeEndpoint           *string
	Provider               *string
	Account                *string
	AccessKey              *string
	AccessSecret           *string
	Domain                 []string
	Revision               int64
}

func (s Service) PatchCert(ctx context.Context, command PatchCertCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if err := validatePatchCertCommand(command); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		cert, err := tx.Certs().GetByID(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && cert.Revision != command.Revision {
			return apperrors.NewConflict("cert revision mismatch")
		}

		if command.DisplayName != nil {
			cert.DisplayName = *command.DisplayName
		}
		if command.Scope != nil {
			cert.Scope = *command.Scope
		}
		if command.Type != nil {
			cert.Type = *command.Type
		}
		if command.CryptoAlgorithm != nil {
			cert.CryptoAlgorithm = *command.CryptoAlgorithm
		}
		if command.BitSize != nil {
			cert.BitSize = *command.BitSize
		}
		if command.ExpireInYears != nil {
			cert.ExpireInYears = *command.ExpireInYears
		}
		if command.Certificate != nil {
			cert.Certificate = *command.Certificate
		}
		if command.PrivateKey != nil {
			cert.PrivateKey = *command.PrivateKey
		}
		if command.Provider != nil {
			cert.Provider = *command.Provider
		}
		if command.Account != nil {
			cert.Account = *command.Account
		}
		if command.AccessKey != nil {
			cert.AccessKey = *command.AccessKey
		}
		if command.AccessSecret != nil {
			cert.AccessSecret = *command.AccessSecret
		}
		if command.AuthorityPublicKey != nil {
			cert.AuthorityPublicKey = *command.AuthorityPublicKey
		}
		if command.AuthorityRootPublicKey != nil {
			cert.AuthorityRootPublicKey = *command.AuthorityRootPublicKey
		}
		if command.AcmeIssuer != nil {
			cert.AcmeIssuer = *command.AcmeIssuer
		}
		if command.AcmeEndpoint != nil {
			cert.AcmeEndpoint = *command.AcmeEndpoint
		}
		if command.Domain != nil {
			cert.Domain = append([]string(nil), command.Domain...)
		}

		cert.UpdatedAt = s.Clock.Now()
		cert.Revision++

		if err := tx.Certs().Update(ctx, cert); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			cert.OrganizationID,
			"certs.updated",
			"certs",
			cert.ID,
			"certs",
			"certs.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

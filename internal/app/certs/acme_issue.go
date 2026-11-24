package certs

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// IssueCertWithACME issues a new SSL certificate using ACME (Let's Encrypt).
func (s Service) IssueCertWithACME(ctx context.Context, command CreateCertCommand) error {
	if s.ACMERenewer == nil {
		return apperrors.NewUnavailable("ACME certificate issuance is not configured")
	}

	if len(command.Domain) == 0 {
		return apperrors.NewInvalidArgument("at least one domain is required for ACME issuance")
	}

	// Obtain certificate via ACME for the first domain
	domain := command.Domain[0]
	certPEM, keyPEM, err := s.ACMERenewer.ObtainCertificate(ctx, domain)
	if err != nil {
		return apperrors.NewInternalf("ACME certificate issuance failed: %v", err)
	}

	// Set the certificate and key from ACME response
	command.Certificate = string(certPEM)
	command.PrivateKey = string(keyPEM)
	command.Type = certTypeSSL
	command.CryptoAlgorithm = "ES256" // ECC P-256
	command.Scope = certTypeSSL
	command.Provider = "letsencrypt"

	// Create the cert record
	return s.CreateCert(ctx, command)
}

// RenewCertWithACME renews an existing SSL certificate using ACME.
// Callers must resolve the tenant first: RenewCertWithACME never performs an
// unbound by-ID lookup, so a background/admin caller has to pass the
// organization that owns the certificate.
func (s Service) RenewCertWithACME(ctx context.Context, organizationID, certID string) error {
	// 平台管理员可续期任意 org 的证书 → 由调用方解析目标 org；本方法按 org 读取。
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if s.ACMERenewer == nil {
		return apperrors.NewUnavailable("ACME certificate renewal is not configured")
	}

	cert, err := s.UOW.Certs().GetByID(ctx, certID, organizationID)
	if err != nil {
		return err
	}

	if cert.Scope != certTypeSSL || !strings.HasPrefix(strings.ToLower(cert.Provider), "letsencrypt") {
		return apperrors.NewInvalidArgument("certificate is not an ACME-issued SSL certificate")
	}

	if len(cert.Domain) == 0 {
		return apperrors.NewInvalidArgument("certificate has no domain configured")
	}

	domain := cert.Domain[0]
	certPEM, keyPEM, err := s.ACMERenewer.RenewCertificate(ctx, domain, []byte(cert.Certificate), []byte(cert.PrivateKey))
	if err != nil {
		return apperrors.NewInternalf("ACME certificate renewal failed: %v", err)
	}

	// Update the certificate
	cert.Certificate = string(certPEM)
	cert.PrivateKey = string(keyPEM)
	cert.Revision++

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
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

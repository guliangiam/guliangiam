package certs

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type CheckDomainExpirationResult struct {
	IsExpired     bool       `json:"isExpired"`
	NotAfter      *time.Time `json:"notAfter,omitempty"`
	DaysRemaining int        `json:"daysRemaining"`
}

func (s Service) CheckDomainExpiration(ctx context.Context, organizationID, id string) (*CheckDomainExpirationResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	cert, err := s.UOW.Certs().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, apperrors.NewNotFound("certificate not found")
	}

	if cert.Certificate == "" {
		return nil, apperrors.NewInvalidArgument("certificate has no PEM data to check")
	}

	block, _ := pem.Decode([]byte(cert.Certificate))
	if block == nil {
		return nil, apperrors.NewInvalidArgument("failed to decode PEM block")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "parse x509 certificate")
	}

	notAfter := x509Cert.NotAfter
	now := s.Clock.Now()
	daysRemaining := int(notAfter.Sub(now).Hours() / 24)
	isExpired := now.After(notAfter)

	return &CheckDomainExpirationResult{
		IsExpired:     isExpired,
		NotAfter:      &notAfter,
		DaysRemaining: daysRemaining,
	}, nil
}

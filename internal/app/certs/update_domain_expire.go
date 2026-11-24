package certs

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// UpdateDomainExpire updates the domain expiration time for a certificate.
func (s Service) UpdateDomainExpire(ctx context.Context, organizationID, id, domainExpireTime string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		cert, err := tx.Certs().GetByID(ctx, id, organizationID)
		if err != nil {
			return apperrors.NewNotFound("certificate not found")
		}

		if domainExpireTime != "" {
			parsed, parseErr := time.Parse(time.RFC3339, domainExpireTime)
			if parseErr != nil {
				return apperrors.NewInvalidArgument("invalid domainExpireTime format, expected RFC3339")
			}
			cert.DomainExpireTime = &parsed
		} else {
			cert.DomainExpireTime = nil
		}

		cert.UpdatedAt = s.Clock.Now()
		cert.Revision++

		if err := tx.Certs().Update(ctx, cert); err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			cert.OrganizationID,
			"cert.domain_expire_updated",
			"certificate",
			id,
			"cert",
			"cert.domain_expire_updated",
			map[string]any{"certId": id, "orgId": cert.OrganizationID, "newExpireTime": domainExpireTime},
		)
	})
}

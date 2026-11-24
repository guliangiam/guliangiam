package certs

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) DeleteCert(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		cert, err := tx.Certs().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.Certs().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			cert.OrganizationID,
			"certs.deleted",
			"certs",
			id,
			"certs",
			"certs.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

package gateway

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s *Service) DeleteSite(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first inside the transaction so a foreign id is rejected before
		// the delete and the audit row carries the entity's real organization.
		site, err := tx.Gateway().GetSiteByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Gateway().DeleteSite(ctx, id); err != nil {
			return err
		}
		return s.embedAuditEvent(
			tx,
			ctx,
			site.OrganizationID,
			"gateway.site.deleted",
			"site",
			id,
			"site",
			"gateway.site.deleted",
			map[string]any{keySiteID: id, keyOrgID: site.OrganizationID},
		)
	})
}

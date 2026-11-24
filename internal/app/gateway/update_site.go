package gateway

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s *Service) UpdateSite(ctx context.Context, organizationID string, command UpdateSiteCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validateUpdateSiteCommand(command); err != nil {
		return err
	}

	// command.ID comes from the URL path. Ownership is decided inside the
	// transaction before any mutation: a foreign or missing id returns NotFound
	// and leaves no write and no audit record behind.
	now := s.Clock.Now()
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		site, err := tx.Gateway().GetSiteByIDForOrganization(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}
		site.Name = command.Name
		site.Host = command.Host
		site.Status = command.Status
		site.SslMode = command.SslMode
		site.CertID = command.CertID
		site.ForceHttps = command.ForceHttps
		site.EnableHealthCheck = command.EnableHealthCheck
		site.HealthCheckInterval = command.HealthCheckInterval
		site.EnableAlert = command.EnableAlert
		site.AlertInterval = command.AlertInterval
		site.AlertTryTimes = command.AlertTryTimes
		site.AlertProviderIDs = command.AlertProviderIDs
		site.Metadata = command.Metadata
		site.UpdatedAt = now
		site.Revision = command.Revision + 1

		if err := tx.Gateway().UpdateSite(ctx, site); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			site.OrganizationID,
			"gateway.site.updated",
			"site",
			site.ID,
			"site",
			"gateway.site.updated",
			map[string]any{

				keySiteID: site.ID,

				keyName: site.Name,

				keyOrgID: site.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}

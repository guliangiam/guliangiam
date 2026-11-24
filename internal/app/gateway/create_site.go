package gateway

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
)

func (s *Service) CreateSite(ctx context.Context, command CreateSiteCommand) (*SiteDTO, error) {
	if err := validateCreateSiteCommand(command); err != nil {
		return nil, err
	}

	// Transport converges command.OrganizationID through TrustedOrganizationID
	// before this use case runs, so the insert can only target the tenant the
	// caller's scope authorizes.
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	site := &domain.Site{
		ID:                  id,
		OrganizationID:      command.OrganizationID,
		Name:                command.Name,
		Host:                command.Host,
		Status:              command.Status,
		SslMode:             command.SslMode,
		CertID:              command.CertID,
		ForceHttps:          command.ForceHttps,
		EnableHealthCheck:   command.EnableHealthCheck,
		HealthCheckInterval: command.HealthCheckInterval,
		LastHealthStatus:    healthStatusUnknown,
		EnableAlert:         command.EnableAlert,
		AlertInterval:       command.AlertInterval,
		AlertTryTimes:       command.AlertTryTimes,
		AlertProviderIDs:    command.AlertProviderIDs,
		Metadata:            command.Metadata,
		CreatedAt:           now,
		UpdatedAt:           now,
		Revision:            1,
	}

	return toSiteDTO(site), s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

		if err := tx.Gateway().CreateSite(ctx, site); err != nil {
			return err

		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			site.OrganizationID,
			"gateway.site.created",
			"site",
			site.ID,
			"site",
			"gateway.site.created",
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

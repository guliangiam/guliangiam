package syncers

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) ListSyncers(ctx context.Context, q ListSyncersQuery) ([]SyncerListItem, int64, error) {
	items, total, err := s.UOW.Syncers().List(ctx, domain.ListQuery(q))
	if err != nil {
		return nil, 0, err
	}
	r := make([]SyncerListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s *Service) GetSyncer(ctx context.Context, organizationID, id string) (*SyncerDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	syncer, err := s.UOW.Syncers().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if err := ensureSyncerTenantScope(syncer, organizationID); err != nil {
		return nil, err
	}
	d := toDetail(syncer)
	return &d, nil
}
func (s *Service) CreateSyncer(ctx context.Context, c CreateSyncerCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	isEnabled := true
	if c.IsEnabled != nil {
		isEnabled = *c.IsEnabled
	}
	syncer := &domain.Syncer{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Type: domain.SyncerType(c.Type), DatabaseType: c.DatabaseType, Host: c.Host, Port: c.Port, Database: c.Database, TableName: c.TableName,
		Username: c.Username, Password: c.Password, SslMode: c.SslMode,
		SshType: c.SshType, SshHost: c.SshHost, SshPort: c.SshPort, SshUsername: c.SshUsername,
		SshPassword: c.SshPassword, SshCertID: c.SshCertID,
		IsEnabled: isEnabled, SyncInterval: c.SyncInterval,
		FieldMapping: c.FieldMapping, ColumnMapping: c.ColumnMapping,
		IsReadOnly: c.IsReadOnly, Affinity: c.Affinity, AffiliationTable: c.AffiliationTable,
		AvatarBaseUrl: c.AvatarBaseUrl, ErrorText: c.ErrorText,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

		if err := tx.Syncers().Create(ctx, syncer); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			syncer.OrganizationID,
			"",
			"syncer.created",
			"syncer",
			syncer.ID,
			"syncer",
			"syncer.created",
			map[string]any{

				keySyncerID: syncer.ID,

				keyName: syncer.Name,

				keyOrgID: syncer.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}
func (s *Service) PatchSyncer(ctx context.Context, organizationID string, c PatchSyncerCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		syncer, err := tx.Syncers().GetByIDForOrganization(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if err := ensureSyncerTenantScope(syncer, organizationID); err != nil {
			return err
		}
		if c.Revision > 0 && syncer.Revision != c.Revision {
			return apperrors.NewConflict("syncer revision mismatch")
		}
		if c.DisplayName != nil {
			syncer.DisplayName = *c.DisplayName
		}
		if c.Host != nil {
			syncer.Host = *c.Host
		}
		if c.Port != nil {
			syncer.Port = *c.Port
		}
		if c.Database != nil {
			syncer.Database = *c.Database
		}
		if c.TableName != nil {
			syncer.TableName = *c.TableName
		}
		if c.Username != nil {
			syncer.Username = *c.Username
		}
		if c.Password != nil {
			syncer.Password = *c.Password
		}
		if c.IsEnabled != nil {
			syncer.IsEnabled = *c.IsEnabled
		}
		if c.SyncInterval != nil {
			syncer.SyncInterval = *c.SyncInterval
		}
		if c.FieldMapping != nil {
			syncer.FieldMapping = *c.FieldMapping
		}
		if c.DatabaseType != nil {
			syncer.DatabaseType = *c.DatabaseType
		}
		if c.SslMode != nil {
			syncer.SslMode = *c.SslMode
		}
		if c.SshType != nil {
			syncer.SshType = *c.SshType
		}
		if c.SshHost != nil {
			syncer.SshHost = *c.SshHost
		}
		if c.SshPort != nil {
			syncer.SshPort = *c.SshPort
		}
		if c.SshUsername != nil {
			syncer.SshUsername = *c.SshUsername
		}
		if c.SshPassword != nil {
			syncer.SshPassword = *c.SshPassword
		}
		if c.SshCertID != nil {
			syncer.SshCertID = *c.SshCertID
		}
		if c.ColumnMapping != nil {
			syncer.ColumnMapping = *c.ColumnMapping
		}
		if c.IsReadOnly != nil {
			syncer.IsReadOnly = *c.IsReadOnly
		}
		if c.Affinity != nil {
			syncer.Affinity = *c.Affinity
		}
		if c.AffiliationTable != nil {
			syncer.AffiliationTable = *c.AffiliationTable
		}
		if c.AvatarBaseUrl != nil {
			syncer.AvatarBaseUrl = *c.AvatarBaseUrl
		}
		if c.ErrorText != nil {
			syncer.ErrorText = *c.ErrorText
		}
		syncer.UpdatedAt = s.Clock.Now()
		syncer.Revision++

		if err := tx.Syncers().Update(ctx, syncer); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			syncer.OrganizationID,
			"",
			"syncer.updated",
			"syncer",
			syncer.ID,
			"syncer",
			"syncer.updated",
			map[string]any{

				keySyncerID: syncer.ID,

				keyName: syncer.Name,

				keyOrgID: syncer.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}
func (s *Service) DeleteSyncer(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		syncer, err := tx.Syncers().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := ensureSyncerTenantScope(syncer, organizationID); err != nil {
			return err
		}

		if err := tx.Syncers().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			syncer.OrganizationID,
			"",
			"syncer.deleted",
			"syncer",
			id,
			"syncer",
			"syncer.deleted",
			map[string]any{

				keySyncerID: id,

				keyName: syncer.Name,

				keyOrgID: syncer.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}

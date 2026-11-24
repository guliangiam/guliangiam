package syncers

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
)

func toListItem(s *domain.Syncer) SyncerListItem {
	return SyncerListItem{ID: s.ID, OrganizationID: s.OrganizationID, Name: s.Name, DisplayName: s.DisplayName,
		Type: string(s.Type), Host: s.Host, Port: s.Port, IsEnabled: s.IsEnabled, SyncInterval: s.SyncInterval,
		LastSyncAt: s.LastSyncAt, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
}
func toDetail(s *domain.Syncer) SyncerDetail {
	return SyncerDetail{SyncerListItem: toListItem(s), DatabaseType: s.DatabaseType, Database: s.Database,
		TableName: s.TableName, SslMode: s.SslMode, SshType: s.SshType, SshHost: s.SshHost,
		SshPort: s.SshPort, SshUsername: s.SshUsername, SshCertID: s.SshCertID,
		FieldMapping: s.FieldMapping, ColumnMapping: s.ColumnMapping,
		IsReadOnly: s.IsReadOnly, Affinity: s.Affinity, AffiliationTable: s.AffiliationTable,
		AvatarBaseUrl: s.AvatarBaseUrl, ErrorText: s.ErrorText, Revision: s.Revision}
}

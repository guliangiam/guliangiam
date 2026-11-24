package syncers

import (
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
)

// syncerEncryptedMapper 负责 password 和 ssh_password 两列的 AES-256-GCM 加解密（DB-058）。
var syncerEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	syncerTable             = "guliangiam.syncers"
	syncerColumnPassword    = "password"
	syncerColumnSSHPassword = "ssh_password"
)

func syncerAAD(syncerID, column string) crypto.AADContext {
	return crypto.AADContext{Table: syncerTable, Column: column, PKValue: syncerID}
}

// toSyncer 把 MySQL 表扫描结果转成 domain.Syncer。列映射 + JSON 反序列化 + 密码解密。
func toSyncer(row syncerRow) (*domain.Syncer, error) {
	columnMapping := ""
	if row.ColumnMapping != nil {
		columnMapping = string(row.ColumnMapping)
	}
	syncer := &domain.Syncer{
		ID:               row.ID,
		OrganizationID:   row.OrganizationID,
		Name:             row.Name,
		DisplayName:      row.DisplayName,
		Type:             domain.SyncerType(row.Type),
		DatabaseType:     row.DatabaseType,
		Host:             row.Host,
		Port:             row.Port,
		Database:         row.Database,
		TableName:        row.TableName,
		Username:         row.Username,
		Password:         row.Password,
		SslMode:          row.SslMode,
		SshType:          row.SshType,
		SshHost:          row.SshHost,
		SshPort:          row.SshPort,
		SshUsername:      row.SshUsername,
		SshPassword:      row.SshPassword,
		SshCertID:        row.SshCertID,
		IsEnabled:        row.IsEnabled,
		SyncInterval:     row.SyncInterval,
		FieldMapping:     row.FieldMapping,
		ColumnMapping:    columnMapping,
		IsReadOnly:       row.IsReadOnly,
		Affinity:         row.Affinity,
		AffiliationTable: row.AffiliationTable,
		AvatarBaseUrl:    row.AvatarBaseUrl,
		ErrorText:        row.ErrorText,
		LastSyncAt:       row.LastSyncAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
	}
	if err := decryptSyncerSecrets(syncer); err != nil {
		return nil, err
	}
	return syncer, nil
}

// syncerSecretsForWrite 返回持久化机密列所需的值，不改 domain 对象。
func syncerSecretsForWrite(s *domain.Syncer) (password, sshPassword string, err error) {
	password, sshPassword = s.Password, s.SshPassword
	if !syncerEncryptedMapper.IsEnabled() {
		return password, sshPassword, nil
	}
	password, err = syncerEncryptedMapper.EncryptColumn(syncerAAD(s.ID, syncerColumnPassword), s.Password)
	if err != nil {
		return "", "", apperrors.WrapInternalf(err, "encrypt syncer password")
	}
	sshPassword, err = syncerEncryptedMapper.EncryptColumn(syncerAAD(s.ID, syncerColumnSSHPassword), s.SshPassword)
	if err != nil {
		return "", "", apperrors.WrapInternalf(err, "encrypt syncer ssh_password")
	}
	return password, sshPassword, nil
}

// decryptSyncerSecrets 解密 password 和 ssh_password（读路径）。
func decryptSyncerSecrets(s *domain.Syncer) error {
	if !syncerEncryptedMapper.IsEnabled() {
		return nil
	}
	password, err := syncerEncryptedMapper.DecryptColumn(syncerAAD(s.ID, syncerColumnPassword), s.Password)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt syncer password")
	}
	sshPassword, err := syncerEncryptedMapper.DecryptColumn(syncerAAD(s.ID, syncerColumnSSHPassword), s.SshPassword)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt syncer ssh_password")
	}
	s.Password = password
	s.SshPassword = sshPassword
	return nil
}

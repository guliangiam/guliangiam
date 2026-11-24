package syncers

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type syncerRow struct {
	ID               string
	OrganizationID   string
	Name             string
	DisplayName      string
	Type             string
	DatabaseType     string
	Host             string
	Port             int
	Database         string
	TableName        string
	Username         string
	Password         string
	SslMode          string
	SshType          string
	SshHost          string
	SshPort          int
	SshUsername      string
	SshPassword      string
	SshCertID        string
	IsEnabled        bool
	SyncInterval     int
	FieldMapping     string
	ColumnMapping    []byte
	IsReadOnly       bool
	Affinity         string
	AffiliationTable string
	AvatarBaseUrl    string
	ErrorText        string
	LastSyncAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Revision         int64
}

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

// syncerEncryptedMapper guards the DB-058 confidential columns: password and
// ssh_password are stored as AES-256-GCM ciphertext.
var syncerEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	syncerTable             = "guliangiam.syncers"
	syncerColumnPassword    = "password"
	syncerColumnSSHPassword = "ssh_password"
)

func syncerAAD(syncerID, column string) crypto.AADContext {
	return crypto.AADContext{Table: syncerTable, Column: column, PKValue: syncerID}
}

// syncerSecretsForWrite returns the values to persist for the confidential
// columns. It never mutates the domain object so callers that already hold the
// plaintext (for a response payload) are not silently corrupted.
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

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

// GetByIDForOrganization 只在 syncer 属于该组织时返回；跨租户与不存在统一 NotFound。
// 管理面与运维（run/test-connection）都走这一条查询，不再保留系统作用域读取。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Syncer, error) {
	return r.getByIDQuery(ctx, queryGetSyncerByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Syncer, error) {
	var row syncerRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Type,
		&row.DatabaseType,
		&row.Host, &row.Port, &row.Database, &row.TableName, &row.Username,
		&row.Password,
		&row.SslMode,
		&row.SshType, &row.SshHost, &row.SshPort, &row.SshUsername, &row.SshPassword, &row.SshCertID,
		&row.IsEnabled, &row.SyncInterval, &row.FieldMapping,
		&row.ColumnMapping,
		&row.IsReadOnly, &row.Affinity, &row.AffiliationTable, &row.AvatarBaseUrl, &row.ErrorText,
		&row.LastSyncAt, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("syncer not found")
		}
		return nil, err
	}

	return toSyncer(row)
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Syncer, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListSyncers, query.OrganizationID, query.Keyword, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Syncer, 0)
	for rows.Next() {
		var row syncerRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Type,
			&row.DatabaseType,
			&row.Host, &row.Port, &row.Database, &row.TableName, &row.Username,
			&row.Password,
			&row.SslMode,
			&row.SshType, &row.SshHost, &row.SshPort, &row.SshUsername, &row.SshPassword, &row.SshCertID,
			&row.IsEnabled, &row.SyncInterval, &row.FieldMapping,
			&row.ColumnMapping,
			&row.IsReadOnly, &row.Affinity, &row.AffiliationTable, &row.AvatarBaseUrl, &row.ErrorText,
			&row.LastSyncAt, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		syncer, err := toSyncer(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, syncer)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountSyncers, query.OrganizationID, query.Keyword, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, syncer *domain.Syncer) error {
	columnMapping := []byte(syncer.ColumnMapping)
	if len(columnMapping) == 0 {
		columnMapping = []byte("[]")
	}
	password, sshPassword, err := syncerSecretsForWrite(syncer)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsertSyncer,
		syncer.ID, syncer.OrganizationID, syncer.Name, syncer.DisplayName, syncer.Type,
		syncer.DatabaseType,
		syncer.Host, syncer.Port, syncer.Database, syncer.TableName, syncer.Username,
		password, syncer.SslMode,
		syncer.SshType, syncer.SshHost, syncer.SshPort, syncer.SshUsername, sshPassword, syncer.SshCertID,
		syncer.IsEnabled, syncer.SyncInterval, syncer.FieldMapping,
		columnMapping,
		syncer.IsReadOnly, syncer.Affinity, syncer.AffiliationTable, syncer.AvatarBaseUrl, syncer.ErrorText,
		syncer.LastSyncAt, syncer.CreatedAt, syncer.UpdatedAt, syncer.Revision,
	)
	return err
}

func (r Repository) Update(ctx context.Context, syncer *domain.Syncer) error {
	columnMapping := []byte(syncer.ColumnMapping)
	if len(columnMapping) == 0 {
		columnMapping = []byte("[]")
	}
	password, sshPassword, err := syncerSecretsForWrite(syncer)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateSyncer,
		syncer.DisplayName, syncer.Host, syncer.Port, syncer.Database, syncer.TableName,
		syncer.Username, password, syncer.IsEnabled, syncer.SyncInterval, syncer.FieldMapping,
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		syncer.LastSyncAt, syncer.UpdatedAt, syncer.Revision-1,
		syncer.DatabaseType, syncer.SslMode,
		syncer.SshType, syncer.SshHost, syncer.SshPort, syncer.SshUsername, sshPassword, syncer.SshCertID,
		columnMapping,
		syncer.IsReadOnly, syncer.Affinity, syncer.AffiliationTable, syncer.AvatarBaseUrl, syncer.ErrorText,
		syncer.ID, syncer.OrganizationID,
	).Scan(&newRevision)
	if err != nil {
		// 实体已在 app 层按租户读取：ErrNoRows 只代表 revision 冲突。
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("syncer revision conflict")
		}
		return err
	}
	syncer.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteSyncer, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("syncer not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

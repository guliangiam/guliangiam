package syncers

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 syncers 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByIDForOrganization 只在 syncer 属于该组织时返回；跨租户与不存在统一 NotFound。
// 管理面与运维（run/test-connection）都走这一条查询；MySQL 无 RLS，organization_id
// 谓词是唯一的租户边界。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Syncer, error) {
	return r.getByIDQuery(ctx, queryGetSyncerByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Syncer, error) {
	var row syncerRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
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
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("syncer not found")
		}
		return nil, err
	}
	return toSyncer(row)
}

// List 分页列出 syncers。
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Syncer, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	var isEnabled any
	if query.IsEnabled != nil {
		isEnabled = *query.IsEnabled
	}

	rows, err := r.db.QueryContext(ctx, queryListSyncers,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
		limit, offset,
	)
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
	if err := r.db.QueryRowContext(ctx, queryCountSyncers,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create 插入 syncer。ColumnMapping 空时存 "[]"。
func (r Repository) Create(ctx context.Context, syncer *domain.Syncer) error {
	columnMapping := []byte(syncer.ColumnMapping)
	if len(columnMapping) == 0 {
		columnMapping = []byte("[]")
	}
	password, sshPassword, err := syncerSecretsForWrite(syncer)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertSyncer,
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

// Update 乐观锁更新。UPDATE + SELECT revision。
func (r Repository) Update(ctx context.Context, syncer *domain.Syncer) error {
	columnMapping := []byte(syncer.ColumnMapping)
	if len(columnMapping) == 0 {
		columnMapping = []byte("[]")
	}
	password, sshPassword, err := syncerSecretsForWrite(syncer)
	if err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, queryUpdateSyncer,
		syncer.DisplayName, syncer.Host, syncer.Port, syncer.Database, syncer.TableName,
		syncer.Username, password, syncer.IsEnabled, syncer.SyncInterval, syncer.FieldMapping,
		syncer.LastSyncAt, syncer.UpdatedAt,
		syncer.DatabaseType, syncer.SslMode,
		syncer.SshType, syncer.SshHost, syncer.SshPort, syncer.SshUsername, sshPassword, syncer.SshCertID,
		columnMapping,
		syncer.IsReadOnly, syncer.Affinity, syncer.AffiliationTable, syncer.AvatarBaseUrl, syncer.ErrorText,
		syncer.ID, syncer.Revision, syncer.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// 实体已在 app 层按租户读取：0 affected 只代表 revision 冲突。
		return apperrors.NewConflict("syncer revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, syncer.ID, syncer.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("syncer not found")
		}
		return err
	}
	syncer.Revision = newRevision
	return nil
}

// Delete 删除 syncer（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteSyncer, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("syncer not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

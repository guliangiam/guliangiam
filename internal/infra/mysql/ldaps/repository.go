package ldaps

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 ldaps 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按主键取 ldap（系统/后台任务路径，不带请求租户）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Ldap, error) {
	return r.getByIDQuery(ctx, queryGetLdapByID, id)
}

// GetByIDForOrganization 只在 ldap 属于该组织时返回；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Ldap, error) {
	return r.getByIDQuery(ctx, queryGetLdapByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Ldap, error) {
	var row ldapRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
		&row.ServerName, &row.Host, &row.Port, &row.Admin,
		&row.BindDN, &row.BindPassword, &row.BaseDN,
		&row.Type, &row.AttributeName, &row.PasswordType,
		&row.Filter, &row.FilterFields,
		&row.EnableSsl, &row.AllowSelfSignedCert,
		&row.EnableGroups, &row.DefaultGroups,
		&row.CustomAttributes,
		&row.IsEnabled, &row.SyncInterval, &row.LastSyncAt,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("ldap not found")
		}
		return nil, err
	}
	return toLdap(row)
}

// List 分页列出 ldaps。
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Ldap, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	var isEnabled any
	if query.IsEnabled != nil {
		isEnabled = *query.IsEnabled
	}

	rows, err := r.db.QueryContext(ctx, queryListLdaps,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Ldap, 0)
	for rows.Next() {
		var row ldapRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.ServerName, &row.Host, &row.Port, &row.Admin,
			&row.BindDN, &row.BindPassword, &row.BaseDN,
			&row.Type, &row.AttributeName, &row.PasswordType,
			&row.Filter, &row.FilterFields,
			&row.EnableSsl, &row.AllowSelfSignedCert,
			&row.EnableGroups, &row.DefaultGroups,
			&row.CustomAttributes,
			&row.IsEnabled, &row.SyncInterval, &row.LastSyncAt,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		l, err := toLdap(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, l)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountLdaps,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create 插入 ldap。JSON 列先序列化成 []byte。
func (r Repository) Create(ctx context.Context, ldap *domain.Ldap) error {
	customAttrs, err := mysqlshared.MarshalJSON(ldap.CustomAttributes)
	if err != nil {
		return err
	}
	filterFields, err := mysqlshared.MarshalJSON(ldap.FilterFields)
	if err != nil {
		return err
	}
	defaultGroups, err := mysqlshared.MarshalJSON(ldap.DefaultGroups)
	if err != nil {
		return err
	}

	if err := encryptLdapSecrets(ldap); err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryInsertLdap,
		ldap.ID, ldap.OrganizationID, ldap.Name, ldap.DisplayName,
		ldap.ServerName, ldap.Host, ldap.Port, ldap.Admin,
		ldap.BindDN, ldap.BindPassword, ldap.BaseDN,
		ldap.Type, ldap.AttributeName, ldap.PasswordType, ldap.Filter,
		filterFields, ldap.EnableSsl, ldap.AllowSelfSignedCert,
		ldap.EnableGroups, defaultGroups, customAttrs,
		ldap.IsEnabled, ldap.SyncInterval, ldap.LastSyncAt,
		ldap.CreatedAt, ldap.UpdatedAt, ldap.Revision,
	)
	return err
}

// Update 乐观锁更新。UPDATE + SELECT revision。
func (r Repository) Update(ctx context.Context, ldap *domain.Ldap) error {
	customAttrs, err := mysqlshared.MarshalJSON(ldap.CustomAttributes)
	if err != nil {
		return err
	}
	filterFields, err := mysqlshared.MarshalJSON(ldap.FilterFields)
	if err != nil {
		return err
	}
	defaultGroups, err := mysqlshared.MarshalJSON(ldap.DefaultGroups)
	if err != nil {
		return err
	}

	if err := encryptLdapSecrets(ldap); err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, queryUpdateLdap,
		ldap.DisplayName, ldap.ServerName, ldap.Host, ldap.Port,
		ldap.Admin, ldap.BindDN, ldap.BindPassword, ldap.BaseDN,
		ldap.Type, ldap.AttributeName, ldap.PasswordType, ldap.Filter,
		filterFields, ldap.EnableSsl, ldap.AllowSelfSignedCert,
		ldap.EnableGroups, defaultGroups, customAttrs,
		ldap.IsEnabled, ldap.SyncInterval, ldap.LastSyncAt,
		ldap.UpdatedAt, ldap.ID, ldap.Revision, ldap.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("ldap revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, ldap.ID, ldap.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("ldap not found")
		}
		return err
	}
	ldap.Revision = newRevision
	return nil
}

// Delete 只在 ldap 属于该组织时删除；0 行受影响统一 NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteLdap, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("ldap not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

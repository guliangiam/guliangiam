package providers

import (
	"context"
	"database/sql"
	"time"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 providers 存储层。
// 与 internal/infra/postgres/providers.Repository 实现同域接口。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// rowsAffected 从 sql.Result 安全提取影响行数。
func rowsAffected(r sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Provider, error) {
	return r.getByID(ctx, queryGetProviderByID, id, providerTenantID(ctx), providerTenantID(ctx))
}

// GetByIDForOrganization is the management by-ID read: MySQL has no RLS, so the
// explicit organization_id predicate is the only tenant boundary. A foreign or
// missing id is NotFound.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Provider, error) {
	return r.getByID(ctx, queryGetProviderByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.Provider, error) {
	var row providerRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Category, &row.ProviderType, &row.SubType, &row.Method, &row.ClientID, &row.ClientSecret, &row.ClientID2, &row.ClientSecret2, &row.ProviderURL, &row.AuthUrl, &row.TokenUrl, &row.UserInfoUrl, &row.CallbackUrl, &row.SigningAlg, &row.Scopes, &row.IdPEntityId, &row.SsoUrl, &row.SloUrl, &row.Metadata, &row.IssuerUrl, &row.Region, &row.Host, &row.Port, &row.DatabaseType, &row.UserMapping, &row.Template, &row.CustomApi, &row.CustomHeader, &row.CustomBody, &row.Admin, &row.Password, &row.AppId, &row.AppSecret, &row.AgentID, &row.ContentType, &row.Sender, &row.Receiver, &row.Enabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("provider not found")
		}
		return nil, err
	}

	provider, err := toProvider(row)
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

// List 参数：orgChk, org, kwChk, kw1, kw2, catChk, cat, ptChk, pt, limit, offset = 11
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Provider, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	rows, err := r.db.QueryContext(ctx, queryListProviders,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		query.Category, query.Category,
		query.ProviderType, query.ProviderType,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Provider, 0)
	for rows.Next() {
		var row providerRow
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Category, &row.ProviderType, &row.SubType, &row.Method, &row.ClientID, &row.ClientSecret, &row.ClientID2, &row.ClientSecret2, &row.ProviderURL, &row.AuthUrl, &row.TokenUrl, &row.UserInfoUrl, &row.CallbackUrl, &row.SigningAlg, &row.Scopes, &row.IdPEntityId, &row.SsoUrl, &row.SloUrl, &row.Metadata, &row.IssuerUrl, &row.Region, &row.Host, &row.Port, &row.DatabaseType, &row.UserMapping, &row.Template, &row.CustomApi, &row.CustomHeader, &row.CustomBody, &row.Admin, &row.Password, &row.AppId, &row.AppSecret, &row.AgentID, &row.ContentType, &row.Sender, &row.Receiver, &row.Enabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision); err != nil {
			return nil, 0, err
		}
		provider, err := toProvider(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, &provider)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count 参数同 List 去掉 limit/offset = 9
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountProviders,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		query.Category, query.Category,
		query.ProviderType, query.ProviderType,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, provider *domain.Provider) error {
	scopes, err := mysqlshared.MarshalJSON(provider.Scopes)
	if err != nil {
		return err
	}
	userMapping, err := mysqlshared.MarshalJSON(provider.UserMapping)
	if err != nil {
		return err
	}
	secrets, err := sealProviderSecrets(provider)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertProvider,
		provider.ID, provider.OrganizationID, provider.Name, provider.DisplayName, provider.Category, provider.ProviderType, provider.SubType, provider.Method, provider.ClientID, secrets.clientSecret, provider.ClientID2, secrets.clientSecret2, provider.ProviderURL, provider.AuthUrl, provider.TokenUrl, provider.UserInfoUrl, provider.CallbackUrl, provider.SigningAlg, scopes, provider.IdPEntityId, provider.SsoUrl, provider.SloUrl, provider.Metadata, provider.IssuerUrl, provider.Region, provider.Host, provider.Port, provider.DatabaseType, userMapping, provider.Template, provider.CustomApi, provider.CustomHeader, provider.CustomBody, provider.Admin, secrets.password, provider.AppId, secrets.appSecret, provider.AgentID, provider.ContentType, provider.Sender, provider.Receiver, provider.Enabled,
		provider.CreatedAt, provider.UpdatedAt, provider.Revision, provider.Family, provider.Kind, provider.Variant)
	return err
}

// Update 乐观锁更新（MySQL 无 RETURNING，拆成两步）。
func (r Repository) Update(ctx context.Context, provider *domain.Provider) error {
	scopes, err := mysqlshared.MarshalJSON(provider.Scopes)
	if err != nil {
		return err
	}
	userMapping, err := mysqlshared.MarshalJSON(provider.UserMapping)
	if err != nil {
		return err
	}
	secrets, err := sealProviderSecrets(provider)
	if err != nil {
		return err
	}

	// UPDATE ... WHERE id = ? AND revision = ?
	res, err := r.db.ExecContext(ctx, queryUpdateProvider,
		provider.DisplayName, provider.Category, provider.ProviderType, provider.SubType, provider.Method, provider.ClientID, secrets.clientSecret, provider.ClientID2, secrets.clientSecret2, provider.ProviderURL, provider.AuthUrl, provider.TokenUrl, provider.UserInfoUrl, provider.CallbackUrl, provider.SigningAlg, scopes, provider.IdPEntityId, provider.SsoUrl, provider.SloUrl, provider.Metadata, provider.IssuerUrl, provider.Region, provider.Host, provider.Port, provider.DatabaseType, userMapping, provider.Template, provider.CustomApi, provider.CustomHeader, provider.CustomBody, provider.Admin, secrets.password, provider.AppId, secrets.appSecret, provider.AgentID, provider.ContentType, provider.Sender, provider.Receiver, provider.Enabled,
		provider.UpdatedAt,
		provider.Family, provider.Kind, provider.Variant, provider.Name,
		provider.ID, provider.Revision-1,
		providerTenantID(ctx), providerTenantID(ctx))
	n, err := rowsAffected(res, err)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("provider revision conflict")
	}

	// 回读新 revision
	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, provider.ID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("provider not found")
		}
		return err
	}
	provider.Revision = newRevision
	return nil
}

func (r Repository) GetConfig(ctx context.Context, providerID string) (*domain.ProviderConfig, error) {
	var row providerConfigRow
	err := r.db.QueryRowContext(ctx, queryGetProviderConfigByProviderID, providerID, providerTenantID(ctx), providerTenantID(ctx)).Scan(&row.ProviderID, &row.ConfigType, &row.Config, &row.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return &domain.ProviderConfig{
				ProviderID: providerID,
				Config:     map[string]any{},
			}, nil
		}
		return nil, apperrors.NewInternal("get provider config")
	}

	config, err := toProviderConfig(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode provider config")
	}
	return config, nil
}

func (r Repository) GetConfigByType(ctx context.Context, providerID, configType string) (*domain.ProviderConfig, error) {
	var row providerConfigRow
	err := r.db.QueryRowContext(ctx, queryGetProviderConfigByType, providerID, configType).Scan(&row.ProviderID, &row.ConfigType, &row.Config, &row.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("provider config not found")
		}
		return nil, apperrors.NewInternal("get provider config by type")
	}
	config, err := toProviderConfig(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode provider config")
	}
	return config, nil
}

func (r Repository) ListConfigsByType(ctx context.Context, providerIDs []string, configType string) ([]domain.ProviderConfig, error) {
	items := make([]domain.ProviderConfig, 0, len(providerIDs))
	if len(providerIDs) == 0 {
		return items, nil
	}
	args := make([]any, 0, len(providerIDs)+1)
	args = append(args, configType)
	for _, providerID := range providerIDs {
		args = append(args, providerID)
	}
	rows, err := r.db.QueryContext(ctx, buildListProviderConfigsByTypeQuery(len(providerIDs)), args...)
	if err != nil {
		return nil, apperrors.NewInternal("list provider configs by type")
	}
	defer rows.Close()
	for rows.Next() {
		var row providerConfigRow
		if err := rows.Scan(&row.ProviderID, &row.ConfigType, &row.Config, &row.UpdatedAt); err != nil {
			return nil, apperrors.NewInternal("scan provider config")
		}
		config, err := toProviderConfig(row)
		if err != nil {
			return nil, apperrors.NewInternal("decode provider config")
		}
		items = append(items, *config)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r Repository) UpdateConfig(ctx context.Context, config *domain.ProviderConfig) error {
	payload, err := mysqlshared.MarshalJSON(config.Config)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryUpsertProviderConfig, config.ProviderID, config.ConfigType, payload, config.UpdatedAt)
	return err
}

func (r Repository) ListSecretStatuses(ctx context.Context, providerID string) ([]domain.ProviderSecretStatus, error) {
	rows, err := r.db.QueryContext(ctx, queryListProviderSecretStatuses, providerID, providerTenantID(ctx), providerTenantID(ctx))
	if err != nil {
		return nil, apperrors.NewInternal("list provider secret statuses")
	}
	defer rows.Close()

	items := make([]domain.ProviderSecretStatus, 0)
	for rows.Next() {
		var row providerSecretStatusRow
		if err := rows.Scan(&row.ID, &row.ProviderID, &row.SecretType, &row.Status, &row.LastRotatedAt, &row.ExpiresAt, &row.UpdatedAt, &row.Version); err != nil {
			return nil, apperrors.NewInternal("scan provider secret status")
		}
		items = append(items, toProviderSecretStatus(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return items, nil
}

func (r Repository) RotateSecretStatus(ctx context.Context, current domain.ProviderSecretStatus, previousStatus string, previousExpiresAt time.Time) error {
	if _, err := r.db.ExecContext(ctx, queryRetirePreviousProviderSecrets,
		previousStatus, previousExpiresAt, current.UpdatedAt,
		current.ProviderID, current.SecretType, current.Version,
	); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, queryInsertProviderSecretStatus,
		current.ID, current.ProviderID, current.SecretType, current.Status,
		current.LastRotatedAt, current.UpdatedAt, current.Version,
	)
	return err
}

func (r Repository) Delete(ctx context.Context, id string) error {
	n, err := rowsAffected(r.db.ExecContext(ctx, queryDeleteProvider, id, providerTenantID(ctx), providerTenantID(ctx)))
	if err != nil {
		return err
	}
	// A row outside the tenant scope must look missing, not silently succeed.
	if n == 0 {
		return apperrors.NewNotFound("provider not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

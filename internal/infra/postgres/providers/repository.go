package providers

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Provider, error) {
	return r.getByID(ctx, queryGetProviderByID, id, providerTenantID(ctx))
}

// GetByIDForOrganization is the management by-ID read: the trusted tenant is an
// explicit predicate rather than an ambient context value, so a foreign or
// missing id is NotFound on every dialect.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Provider, error) {
	return r.getByID(ctx, queryGetProviderByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.Provider, error) {
	var row providerRow
	err := r.db.QueryRow(ctx, query, args...).Scan(&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Category, &row.ProviderType, &row.SubType, &row.Method, &row.ClientID, &row.ClientSecret, &row.ClientID2, &row.ClientSecret2, &row.ProviderURL, &row.AuthUrl, &row.TokenUrl, &row.UserInfoUrl, &row.CallbackUrl, &row.SigningAlg, &row.Scopes, &row.IdPEntityId, &row.SsoUrl, &row.SloUrl, &row.Metadata, &row.IssuerUrl, &row.Region, &row.Host, &row.Port, &row.DatabaseType, &row.UserMapping, &row.Template, &row.CustomApi, &row.CustomHeader, &row.CustomBody, &row.Admin, &row.Password, &row.AppId, &row.AppSecret, &row.AgentID, &row.ContentType, &row.Sender, &row.Receiver, &row.Enabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision, &row.Family, &row.Kind, &row.Variant)
	if err != nil {
		if err == pgx.ErrNoRows {
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

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Provider, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListProviders, query.OrganizationID, query.Keyword, query.Category, query.ProviderType, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Provider, 0)
	for rows.Next() {
		var row providerRow
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Category, &row.ProviderType, &row.SubType, &row.Method, &row.ClientID, &row.ClientSecret, &row.ClientID2, &row.ClientSecret2, &row.ProviderURL, &row.AuthUrl, &row.TokenUrl, &row.UserInfoUrl, &row.CallbackUrl, &row.SigningAlg, &row.Scopes, &row.IdPEntityId, &row.SsoUrl, &row.SloUrl, &row.Metadata, &row.IssuerUrl, &row.Region, &row.Host, &row.Port, &row.DatabaseType, &row.UserMapping, &row.Template, &row.CustomApi, &row.CustomHeader, &row.CustomBody, &row.Admin, &row.Password, &row.AppId, &row.AppSecret, &row.AgentID, &row.ContentType, &row.Sender, &row.Receiver, &row.Enabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision, &row.Family, &row.Kind, &row.Variant); err != nil {
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

	var total int64
	if err := r.db.QueryRow(ctx, queryCountProviders, query.OrganizationID, query.Keyword, query.Category, query.ProviderType).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, provider *domain.Provider) error {
	userMapping, err := shared.MarshalJSON(provider.UserMapping)
	if err != nil {
		return err
	}
	secrets, err := sealProviderSecrets(provider)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsertProvider,
		provider.ID, provider.OrganizationID, provider.Name, provider.DisplayName, provider.Category, provider.ProviderType, provider.SubType, provider.Method, provider.ClientID, secrets.clientSecret, provider.ClientID2, secrets.clientSecret2, provider.ProviderURL, provider.AuthUrl, provider.TokenUrl, provider.UserInfoUrl, provider.CallbackUrl, provider.SigningAlg, provider.Scopes, provider.IdPEntityId, provider.SsoUrl, provider.SloUrl, provider.Metadata, provider.IssuerUrl, provider.Region, provider.Host, provider.Port, provider.DatabaseType, userMapping, provider.Template, provider.CustomApi, provider.CustomHeader, provider.CustomBody, provider.Admin, secrets.password, provider.AppId, secrets.appSecret, provider.AgentID, provider.ContentType, provider.Sender, provider.Receiver, provider.Enabled,
		provider.CreatedAt, provider.UpdatedAt, provider.Revision, provider.Family, provider.Kind, provider.Variant)
	return err
}

func (r Repository) Update(ctx context.Context, provider *domain.Provider) error {
	userMapping, err := shared.MarshalJSON(provider.UserMapping)
	if err != nil {
		return err
	}
	secrets, err := sealProviderSecrets(provider)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateProvider,
		provider.DisplayName, provider.Category, provider.ProviderType, provider.SubType, provider.Method, provider.ClientID, secrets.clientSecret, provider.ClientID2, secrets.clientSecret2, provider.ProviderURL, provider.AuthUrl, provider.TokenUrl, provider.UserInfoUrl, provider.CallbackUrl, provider.SigningAlg, provider.Scopes, provider.IdPEntityId, provider.SsoUrl, provider.SloUrl, provider.Metadata, provider.IssuerUrl, provider.Region, provider.Host, provider.Port, provider.DatabaseType, userMapping, provider.Template, provider.CustomApi, provider.CustomHeader, provider.CustomBody, provider.Admin, secrets.password, provider.AppId, secrets.appSecret, provider.AgentID, provider.ContentType, provider.Sender, provider.Receiver, provider.Enabled,
		provider.UpdatedAt, provider.Revision-1, provider.ID, provider.Family, provider.Kind, provider.Variant, provider.Name, providerTenantID(ctx)).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("provider revision conflict")
		}
		return err
	}
	provider.Revision = newRevision
	return nil
}

func (r Repository) GetConfig(ctx context.Context, providerID string) (*domain.ProviderConfig, error) {
	var row providerConfigRow
	err := r.db.QueryRow(ctx, queryGetProviderConfigByProviderID, providerID, providerTenantID(ctx)).Scan(&row.ProviderID, &row.ConfigType, &row.Config, &row.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
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
	err := r.db.QueryRow(ctx, queryGetProviderConfigByType, providerID, configType).Scan(&row.ProviderID, &row.ConfigType, &row.Config, &row.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
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
	rows, err := r.db.Query(ctx, buildListProviderConfigsByTypeQuery(len(providerIDs)), args...)
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
	payload, err := shared.MarshalJSON(config.Config)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryUpsertProviderConfig, config.ProviderID, config.ConfigType, payload, config.UpdatedAt)
	return err
}

func (r Repository) ListSecretStatuses(ctx context.Context, providerID string) ([]domain.ProviderSecretStatus, error) {
	rows, err := r.db.Query(ctx, queryListProviderSecretStatuses, providerID, providerTenantID(ctx))
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
	if _, err := r.db.Exec(ctx, queryRetirePreviousProviderSecrets,
		current.ProviderID, current.SecretType, current.Version,
		previousStatus, previousExpiresAt, current.UpdatedAt,
	); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, queryInsertProviderSecretStatus,
		current.ID, current.ProviderID, current.SecretType, current.Status,
		current.LastRotatedAt, current.UpdatedAt, current.Version,
	)
	return err
}

func (r Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, queryDeleteProvider, id, providerTenantID(ctx))
	if err != nil {
		return err
	}
	// A row outside the tenant scope must look missing, not silently succeed.
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("provider not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

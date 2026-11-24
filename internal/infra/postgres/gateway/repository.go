package gateway

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
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

func (r Repository) GetServerByID(ctx context.Context, id string) (*domain.Server, error) {
	return r.getServerByID(ctx, queryGetServerByID, id)
}

func (r Repository) GetServerByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Server, error) {
	return r.getServerByID(ctx, queryGetServerByIDForOrganization, id, organizationID)
}

func (r Repository) getServerByID(ctx context.Context, query string, args ...any) (*domain.Server, error) {
	var row serverRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.Endpoint, &row.Protocol, &row.Status,
		&row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("server not found")
		}
		return nil, err
	}

	server := toServer(row)
	return &server, nil
}

func (r Repository) ListServers(ctx context.Context, query domain.ListServersQuery) ([]*domain.Server, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListServers, query.OrganizationID, query.Keyword, query.Status, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Server, 0)
	for rows.Next() {
		var row serverRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.Endpoint, &row.Protocol, &row.Status,
			&row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		server := toServer(row)
		items = append(items, &server)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountServers, query.OrganizationID, query.Keyword, query.Status).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) CreateServer(ctx context.Context, server *domain.Server) error {
	_, err := r.db.Exec(ctx, queryInsertServer,
		server.ID, server.OrganizationID, server.Name, server.Endpoint, server.Protocol,
		server.Status, metadataOrEmpty(server.Metadata), server.CreatedAt, server.UpdatedAt, server.Revision,
	)
	return err
}

func (r Repository) UpdateServer(ctx context.Context, server *domain.Server) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryUpdateServer,
		server.Name, server.Endpoint, server.Protocol, server.Status,
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		metadataOrEmpty(server.Metadata), server.UpdatedAt, server.Revision-1, server.ID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("server revision conflict")
		}
		return err
	}
	server.Revision = newRevision
	return nil
}

func (r Repository) DeleteServer(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx, queryDeleteServer, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("server not found")
	}
	return nil
}

func (r Repository) GetSiteByID(ctx context.Context, id string) (*domain.Site, error) {
	return r.getSiteByID(ctx, queryGetSiteByID, id)
}

func (r Repository) GetSiteByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Site, error) {
	return r.getSiteByID(ctx, queryGetSiteByIDForOrganization, id, organizationID)
}

func (r Repository) getSiteByID(ctx context.Context, query string, args ...any) (*domain.Site, error) {
	var row siteRow
	var certID *string
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.Host, &row.Status,
		&row.SslMode, &certID, &row.ForceHttps,
		&row.EnableHealthCheck, &row.HealthCheckInterval, &row.LastHealthStatus, &row.LastHealthAt,
		&row.EnableAlert, &row.AlertInterval, &row.AlertTryTimes, &row.AlertProviderIDs,
		&row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision,
	)
	row.CertID = stringOrEmpty(certID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("site not found")
		}
		return nil, err
	}

	site := toSite(row)
	return &site, nil
}

func (r Repository) ListSites(ctx context.Context, query domain.ListSitesQuery) ([]*domain.Site, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListSites, query.OrganizationID, query.Keyword, query.Status, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Site, 0)
	for rows.Next() {
		var row siteRow
		var certID *string
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.Host, &row.Status,
			&row.SslMode, &certID, &row.ForceHttps,
			&row.EnableHealthCheck, &row.HealthCheckInterval, &row.LastHealthStatus, &row.LastHealthAt,
			&row.EnableAlert, &row.AlertInterval, &row.AlertTryTimes, &row.AlertProviderIDs,
			&row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		row.CertID = stringOrEmpty(certID)
		site := toSite(row)
		items = append(items, &site)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountSites, query.OrganizationID, query.Keyword, query.Status).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) CreateSite(ctx context.Context, site *domain.Site) error {
	_, err := r.db.Exec(ctx, queryInsertSite,
		site.ID, site.OrganizationID, site.Name, site.Host,
		site.Status, site.SslMode, nullableUUID(site.CertID), site.ForceHttps,
		site.EnableHealthCheck, site.HealthCheckInterval, site.LastHealthStatus, site.LastHealthAt,
		site.EnableAlert, site.AlertInterval, site.AlertTryTimes, stringSlice(site.AlertProviderIDs),
		metadataOrEmpty(site.Metadata), site.CreatedAt, site.UpdatedAt, site.Revision,
	)
	return err
}

func (r Repository) UpdateSite(ctx context.Context, site *domain.Site) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryUpdateSite,
		site.Name, site.Host, site.Status,
		site.SslMode, nullableUUID(site.CertID), site.ForceHttps,
		site.EnableHealthCheck, site.HealthCheckInterval, site.LastHealthStatus, site.LastHealthAt,
		site.EnableAlert, site.AlertInterval, site.AlertTryTimes, stringSlice(site.AlertProviderIDs),
		metadataOrEmpty(site.Metadata),
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		site.UpdatedAt, site.Revision-1, site.ID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("site revision conflict")
		}
		return err
	}
	site.Revision = newRevision
	return nil
}

func (r Repository) DeleteSite(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx, queryDeleteSite, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("site not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// nullableUUID 把空字符串转成 NULL：cert_id 是 uuid 列，直接写 "" 会触发
// SQLSTATE 22P02（invalid input syntax for type uuid）。
func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

// metadataOrEmpty 保证 NOT NULL 的 JSONB 列收到 {} 而不是 NULL。
func metadataOrEmpty(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

// stringSlice 保证 NOT NULL 的 TEXT[] 列收到空数组而不是 NULL。
func stringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}

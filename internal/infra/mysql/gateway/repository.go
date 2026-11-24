package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 gateway (servers + sites) 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// serverRow 是 servers 表扫描目标。
// Metadata PG 是 jsonb → MySQL TEXT (JSON 字符串)。
type serverRow struct {
	ID             string
	OrganizationID string
	Name           string
	Endpoint       string
	Protocol       string
	Status         string
	Metadata       string // JSON → map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
	Revision       int64
}

// siteRow 是 sites 表扫描目标。
// Metadata PG jsonb → MySQL TEXT；AlertProviderIDs PG text[] → MySQL TEXT (JSON 数组)。
type siteRow struct {
	ID                  string
	OrganizationID      string
	Name                string
	Host                string
	Status              string
	SslMode             string
	CertID              string
	ForceHttps          bool
	EnableHealthCheck   bool
	HealthCheckInterval int
	LastHealthStatus    string
	LastHealthAt        *time.Time
	EnableAlert         bool
	AlertInterval       int
	AlertTryTimes       int
	AlertProviderIDs    string // JSON 数组 → []string
	Metadata            string // JSON → map[string]any
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
	Revision            int64
}

// ── Servers ──────────────────────────────────────────────────────

// GetServerByID 按 id 查 Server（系统级；请求路径必须使用 scoped 版本）。
func (r Repository) GetServerByID(ctx context.Context, id string) (*domain.Server, error) {
	return r.getServerByID(ctx, queryGetServerByID, id)
}

// GetServerByIDForOrganization 按 id + 组织查 Server；跨租户与不存在统一 NotFound。
func (r Repository) GetServerByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Server, error) {
	return r.getServerByID(ctx, queryGetServerByIDForOrganization, id, organizationID)
}

func (r Repository) getServerByID(ctx context.Context, query string, args ...any) (*domain.Server, error) {
	var row serverRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.Endpoint, &row.Protocol, &row.Status,
		&row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("server not found")
		}
		return nil, err
	}
	s := toServer(row)
	return &s, nil
}

// ListServers 分页列出 Server。MySQL 版需要手动处理参数重复占位符。
func (r Repository) ListServers(ctx context.Context, q domain.ListServersQuery) ([]*domain.Server, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	// 占位符参数：org_id×2 + keyword×3 + status×2 + limit + offset = 9
	args := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.Status, q.Status,
		limit, offset,
	}
	rows, err := r.db.QueryContext(ctx, queryListServers, args...)
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
		s := toServer(row)
		items = append(items, &s)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count 参数：org_id×2 + keyword×3 + status×2 = 7
	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.Status, q.Status,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountServers, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// CreateServer 插入 Server，metadata 用 JSON 字符串。
func (r Repository) CreateServer(ctx context.Context, s *domain.Server) error {
	metadataJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertServer,
		s.ID, s.OrganizationID, s.Name, s.Endpoint, s.Protocol,
		s.Status, string(metadataJSON), s.CreatedAt, s.UpdatedAt, s.Revision,
	)
	return err
}

// UpdateServer 更新 Server。MySQL 没有 RETURNING，拆成 UPDATE + SELECT revision。
// affected rows == 0 说明 revision 冲突或行已删除。
func (r Repository) UpdateServer(ctx context.Context, s *domain.Server) error {
	metadataJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, queryUpdateServer,
		s.Name, s.Endpoint, s.Protocol, s.Status,
		string(metadataJSON), s.UpdatedAt, s.ID, s.Revision,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("server revision conflict")
	}

	// 取新 revision
	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectServerRevision, s.ID).Scan(&newRevision); err != nil {
		return err
	}
	s.Revision = newRevision
	return nil
}

// DeleteServer 软删除 Server。
func (r Repository) DeleteServer(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, queryDeleteServer, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("server not found")
	}
	return nil
}

// ── Sites ────────────────────────────────────────────────────────

// GetSiteByID 按 id 查 Site（系统级；请求路径必须使用 scoped 版本）。
func (r Repository) GetSiteByID(ctx context.Context, id string) (*domain.Site, error) {
	return r.getSiteByID(ctx, queryGetSiteByID, id)
}

// GetSiteByIDForOrganization 按 id + 组织查 Site；跨租户与不存在统一 NotFound。
func (r Repository) GetSiteByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Site, error) {
	return r.getSiteByID(ctx, queryGetSiteByIDForOrganization, id, organizationID)
}

func (r Repository) getSiteByID(ctx context.Context, query string, args ...any) (*domain.Site, error) {
	var row siteRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.Host, &row.Status,
		&row.SslMode, &row.CertID, &row.ForceHttps,
		&row.EnableHealthCheck, &row.HealthCheckInterval, &row.LastHealthStatus, &row.LastHealthAt,
		&row.EnableAlert, &row.AlertInterval, &row.AlertTryTimes, &row.AlertProviderIDs,
		&row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("site not found")
		}
		return nil, err
	}
	s := toSite(row)
	return &s, nil
}

// ListSites 分页列出 Site，参数占位符同 ListServers（org_id×2+keyword×3+status×2+limit+offset=9）。
func (r Repository) ListSites(ctx context.Context, q domain.ListSitesQuery) ([]*domain.Site, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	args := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.Status, q.Status,
		limit, offset,
	}
	rows, err := r.db.QueryContext(ctx, queryListSites, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Site, 0)
	for rows.Next() {
		var row siteRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.Host, &row.Status,
			&row.SslMode, &row.CertID, &row.ForceHttps,
			&row.EnableHealthCheck, &row.HealthCheckInterval, &row.LastHealthStatus, &row.LastHealthAt,
			&row.EnableAlert, &row.AlertInterval, &row.AlertTryTimes, &row.AlertProviderIDs,
			&row.Metadata, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		s := toSite(row)
		items = append(items, &s)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.Status, q.Status,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountSites, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// CreateSite 插入 Site，metadata + alert_provider_ids 用 JSON 字符串。
func (r Repository) CreateSite(ctx context.Context, s *domain.Site) error {
	metadataJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return err
	}
	alertIDsJSON, err := json.Marshal(s.AlertProviderIDs)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertSite,
		s.ID, s.OrganizationID, s.Name, s.Host,
		s.Status, s.SslMode, s.CertID, s.ForceHttps,
		s.EnableHealthCheck, s.HealthCheckInterval, s.LastHealthStatus, s.LastHealthAt,
		s.EnableAlert, s.AlertInterval, s.AlertTryTimes, string(alertIDsJSON),
		string(metadataJSON), s.CreatedAt, s.UpdatedAt, s.Revision,
	)
	return err
}

// UpdateSite 更新 Site。MySQL 没有 RETURNING，拆成 UPDATE + SELECT revision。
func (r Repository) UpdateSite(ctx context.Context, s *domain.Site) error {
	metadataJSON, err := json.Marshal(s.Metadata)
	if err != nil {
		return err
	}
	alertIDsJSON, err := json.Marshal(s.AlertProviderIDs)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, queryUpdateSite,
		s.Name, s.Host, s.Status,
		s.SslMode, s.CertID, s.ForceHttps,
		s.EnableHealthCheck, s.HealthCheckInterval, s.LastHealthStatus, s.LastHealthAt,
		s.EnableAlert, s.AlertInterval, s.AlertTryTimes, string(alertIDsJSON),
		string(metadataJSON),
		s.UpdatedAt, s.ID, s.Revision,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("site revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectSiteRevision, s.ID).Scan(&newRevision); err != nil {
		return err
	}
	s.Revision = newRevision
	return nil
}

// DeleteSite 软删除 Site。
func (r Repository) DeleteSite(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, queryDeleteSite, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("site not found")
	}
	return nil
}

// ── mappers ──────────────────────────────────────────────────────

// toServer 把 MySQL serverRow 转成 domain.Server，处理 metadata JSON 反序列化。
func toServer(row serverRow) domain.Server {
	s := domain.Server{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		Endpoint:       row.Endpoint,
		Protocol:       row.Protocol,
		Status:         row.Status,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		DeletedAt:      row.DeletedAt,
		Revision:       row.Revision,
	}
	if row.Metadata != "" {
		_ = json.Unmarshal([]byte(row.Metadata), &s.Metadata)
	}
	return s
}

// toSite 把 MySQL siteRow 转成 domain.Site，处理 metadata + alert_provider_ids JSON。
func toSite(row siteRow) domain.Site {
	s := domain.Site{
		ID:                  row.ID,
		OrganizationID:      row.OrganizationID,
		Name:                row.Name,
		Host:                row.Host,
		Status:              row.Status,
		SslMode:             row.SslMode,
		CertID:              row.CertID,
		ForceHttps:          row.ForceHttps,
		EnableHealthCheck:   row.EnableHealthCheck,
		HealthCheckInterval: row.HealthCheckInterval,
		LastHealthStatus:    row.LastHealthStatus,
		LastHealthAt:        row.LastHealthAt,
		EnableAlert:         row.EnableAlert,
		AlertInterval:       row.AlertInterval,
		AlertTryTimes:       row.AlertTryTimes,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		DeletedAt:           row.DeletedAt,
		Revision:            row.Revision,
	}
	if row.Metadata != "" {
		_ = json.Unmarshal([]byte(row.Metadata), &s.Metadata)
	}
	if row.AlertProviderIDs != "" {
		_ = json.Unmarshal([]byte(row.AlertProviderIDs), &s.AlertProviderIDs)
	}
	return s
}

// 编译期接口断言
var _ domain.Repository = Repository{}

package connectors

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	domainconnectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	tableConnectors           = "guliangiam.connectors"
	columnBearerTokenSealed   = "bearer_token_sealed"
	connectorRevisionConflict = "connector revision conflict"
)

// Repository 是 guliangiam.connectors 的 PostgreSQL 适配器。
//
// db 可以是请求/UoW 事务（shared.DBTX），此时 FORCE RLS 的租户上下文由外层
// 事务的 set_tenant_context 提供；组织谓词仍在每条 SQL 中显式存在。
// encryptor 由装配层注入，Bearer 明文只在内存中停留。
type Repository struct {
	db        shared.DBTX
	encryptor *crypto.FieldEncryptor
}

// NewRepository 构造适配器。encryptor 允许为 nil，但任何非空 Bearer 的
// 封/解都会 fail-closed（返回 Internal），不会退化为明文存储。
func NewRepository(db shared.DBTX, encryptor *crypto.FieldEncryptor) Repository {
	return Repository{db: db, encryptor: encryptor}
}

var _ domainconnectors.ConnectorRepository = Repository{}

// Get 返回指定租户下的 connector；跨租户与不存在同为 NotFound。
func (r Repository) Get(ctx context.Context, connectorID, organizationID string) (*domainconnectors.Connector, error) {
	if connectorID == "" || organizationID == "" {
		return nil, apperrors.NewInvalidArgument("connectorID and organizationID are required")
	}
	row := r.db.QueryRow(ctx, queryConnectorGet, connectorID, organizationID)
	connector, err := r.scanConnector(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("connector not found")
		}
		return nil, err
	}
	return connector, nil
}

// List 返回指定租户的全部 connector，按名称排序。
func (r Repository) List(ctx context.Context, organizationID string) ([]*domainconnectors.Connector, error) {
	if organizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationID is required")
	}
	rows, err := r.db.Query(ctx, queryConnectorList, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domainconnectors.Connector, 0, 32)
	for rows.Next() {
		var (
			connector domainconnectors.Connector
			sealed    string
		)
		if err := rows.Scan(
			&connector.ID, &connector.OrganizationID, &connector.Name,
			&connector.SCIMBaseURL, &sealed, &connector.IsEnabled,
			&connector.TimeoutMs, &connector.MaxRatePerMin,
			&connector.Revision, &connector.CreatedAt, &connector.UpdatedAt,
		); err != nil {
			return nil, err
		}
		connector.BearerToken, err = r.openBearerToken(connector.ID, sealed)
		if err != nil {
			return nil, err
		}
		items = append(items, &connector)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return items, nil
}

// Create 在显式租户下插入 connector。Bearer 令牌先封成信封密文，再进入 SQL。
func (r Repository) Create(ctx context.Context, organizationID string, connector *domainconnectors.Connector) error {
	if connector == nil {
		return apperrors.NewInvalidArgument("connector is required")
	}
	if organizationID == "" {
		return apperrors.NewInvalidArgument("organizationID is required")
	}
	if connector.OrganizationID != "" && connector.OrganizationID != organizationID {
		return apperrors.NewInvalidArgument("connector organization mismatch")
	}

	sealed, err := r.sealBearerToken(connector.ID, connector.BearerToken)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if connector.CreatedAt.IsZero() {
		connector.CreatedAt = now
	}
	if connector.UpdatedAt.IsZero() {
		connector.UpdatedAt = now
	}
	if connector.Revision == 0 {
		connector.Revision = 1
	}
	connector.OrganizationID = organizationID

	_, err = r.db.Exec(ctx, queryConnectorInsert,
		connector.ID, organizationID, connector.Name, connector.SCIMBaseURL,
		sealed, connector.IsEnabled, connector.TimeoutMs, connector.MaxRatePerMin,
		connector.Revision, connector.CreatedAt, connector.UpdatedAt,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "create connector")
	}
	return nil
}

// Update 以 revision 乐观锁写入显式租户下的 connector。revision 不匹配或
// 跨租户命中零行时返回 Conflict，调用方实体与数据库都不发生写入。
func (r Repository) Update(ctx context.Context, organizationID string, connector *domainconnectors.Connector) error {
	if connector == nil {
		return apperrors.NewInvalidArgument("connector is required")
	}
	if organizationID == "" {
		return apperrors.NewInvalidArgument("organizationID is required")
	}
	if connector.OrganizationID != "" && connector.OrganizationID != organizationID {
		return apperrors.NewInvalidArgument("connector organization mismatch")
	}
	if connector.Revision <= 0 {
		return apperrors.NewInvalidArgument("connector revision is required")
	}

	sealed, err := r.sealBearerToken(connector.ID, connector.BearerToken)
	if err != nil {
		return err
	}
	connector.OrganizationID = organizationID
	connector.UpdatedAt = time.Now().UTC()

	var newRevision int64
	err = r.db.QueryRow(ctx, queryConnectorUpdate,
		connector.Name, connector.SCIMBaseURL, sealed, connector.IsEnabled,
		connector.TimeoutMs, connector.MaxRatePerMin, connector.UpdatedAt,
		connector.ID, organizationID, connector.Revision,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict(connectorRevisionConflict)
		}
		return err
	}
	connector.Revision = newRevision
	return nil
}

// Delete 删除显式租户下的 connector；跨租户与不存在同为 NotFound。
func (r Repository) Delete(ctx context.Context, organizationID, connectorID string) error {
	if organizationID == "" || connectorID == "" {
		return apperrors.NewInvalidArgument("organizationID and connectorID are required")
	}
	tag, err := r.db.Exec(ctx, queryConnectorDelete, connectorID, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("connector not found")
	}
	return nil
}

func (r Repository) scanConnector(row pgx.Row) (*domainconnectors.Connector, error) {
	var (
		connector domainconnectors.Connector
		sealed    string
	)
	if err := row.Scan(
		&connector.ID, &connector.OrganizationID, &connector.Name,
		&connector.SCIMBaseURL, &sealed, &connector.IsEnabled,
		&connector.TimeoutMs, &connector.MaxRatePerMin,
		&connector.Revision, &connector.CreatedAt, &connector.UpdatedAt,
	); err != nil {
		return nil, err
	}
	plaintext, err := r.openBearerToken(connector.ID, sealed)
	if err != nil {
		return nil, err
	}
	connector.BearerToken = plaintext
	return &connector, nil
}

// sealBearerToken 把明文封成绑定 (table, column, connector id) 的 v2 信封。
// 空令牌保持空串，表示“未配置”；错误串不包含明文或密文。
func (r Repository) sealBearerToken(connectorID, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if r.encryptor == nil {
		return "", apperrors.NewInternal("connector field encryptor is not configured")
	}
	sealed, err := r.encryptor.EncryptColumn(bearerTokenAAD(connectorID), plaintext)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt %s.%s", tableConnectors, columnBearerTokenSealed)
	}
	return sealed, nil
}

// openBearerToken 只接受信封密文（v1/v2）；明文行 fail-closed，指向回填命令。
// 错误串不包含明文或密文。
func (r Repository) openBearerToken(connectorID, sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	if r.encryptor == nil {
		return "", apperrors.NewInternal("connector field encryptor is not configured")
	}
	plaintext, err := r.encryptor.DecryptEnvelopeFieldWithAAD(bearerTokenAAD(connectorID), sealed)
	if err == nil {
		return plaintext, nil
	}
	if errors.Is(err, crypto.ErrNotEnvelope) {
		return "", apperrors.NewInternalf(
			"%s.%s for connector %s is not envelope-encrypted",
			tableConnectors, columnBearerTokenSealed, connectorID)
	}
	return "", apperrors.WrapInternalf(err, "decrypt %s.%s for connector %s",
		tableConnectors, columnBearerTokenSealed, connectorID)
}

func bearerTokenAAD(connectorID string) crypto.AADContext {
	return crypto.AADContext{
		Table:   tableConnectors,
		Column:  columnBearerTokenSealed,
		PKValue: connectorID,
	}
}

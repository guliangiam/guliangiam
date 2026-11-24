package certs

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 certs 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// certRow 是 certs 表扫描目标。
// Domain PG jsonb → MySQL TEXT（JSON 字符串）。
type certRow struct {
	ID                     string
	OrganizationID         string
	Name                   string
	DisplayName            string
	Scope                  string
	Type                   string
	CryptoAlgorithm        string
	BitSize                int
	ExpireInYears          int
	ExpireTime             *sql.NullTime
	Certificate            string
	PrivateKey             string
	AuthorityPublicKey     string
	AuthorityRootPublicKey string
	AcmeIssuer             string
	AcmeEndpoint           string
	Provider               string
	Account                string
	AccessKey              string
	AccessSecret           string
	Domain                 string // JSON 字符串 → []string
	DomainExpireTime       *sql.NullTime
	CreatedAt              sql.NullTime
	UpdatedAt              sql.NullTime
	Revision               int64
}

// GetByID 按 id + organization_id 查 Cert。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Cert, error) {
	var row certRow
	err := r.db.QueryRowContext(ctx, queryGetCertByID, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
		&row.Scope, &row.Type, &row.CryptoAlgorithm, &row.BitSize, &row.ExpireInYears,
		&row.ExpireTime, &row.Certificate, &row.PrivateKey,
		&row.AuthorityPublicKey, &row.AuthorityRootPublicKey,
		&row.AcmeIssuer, &row.AcmeEndpoint,
		&row.Provider, &row.Account, &row.AccessKey, &row.AccessSecret, &row.Domain,
		&row.DomainExpireTime,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("cert not found")
		}
		return nil, err
	}

	cert, err := toCert(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode cert")
	}
	return cert, nil
}

// List 分页列出 Cert。MySQL 版需要手动展开参数重复占位符。
// listArgs: org_id×2 + keyword×3 + scope×2 + type×2 + limit + offset = 11
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Cert, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	countArgs := []interface{}{
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		query.Scope, query.Scope,
		query.Type, query.Type,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountCerts, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, queryListCerts, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Cert, 0)
	for rows.Next() {
		var row certRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.Scope, &row.Type, &row.CryptoAlgorithm, &row.BitSize, &row.ExpireInYears,
			&row.ExpireTime, &row.Certificate, &row.PrivateKey,
			&row.AuthorityPublicKey, &row.AuthorityRootPublicKey,
			&row.AcmeIssuer, &row.AcmeEndpoint,
			&row.Provider, &row.Account, &row.AccessKey, &row.AccessSecret, &row.Domain,
			&row.DomainExpireTime,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		cert, err := toCert(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode cert")
		}
		items = append(items, cert)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 Cert，domain 字段用 JSON 字符串。
func (r Repository) Create(ctx context.Context, cert *domain.Cert) error {
	domainJSON, err := json.Marshal(cert.Domain)
	if err != nil {
		return err
	}
	var domainStr string
	if domainJSON != nil {
		domainStr = string(domainJSON)
	}
	privateKey, accessSecret, err := certSecretsForWrite(cert)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryInsertCert,
		cert.ID, cert.OrganizationID, cert.Name, cert.DisplayName,
		cert.Scope, cert.Type, cert.CryptoAlgorithm, cert.BitSize, cert.ExpireInYears,
		cert.ExpireTime, cert.Certificate, privateKey,
		cert.AuthorityPublicKey, cert.AuthorityRootPublicKey,
		cert.AcmeIssuer, cert.AcmeEndpoint,
		cert.Provider, cert.Account, cert.AccessKey, accessSecret, domainStr,
		cert.DomainExpireTime,
		cert.CreatedAt, cert.UpdatedAt, cert.Revision,
	)
	return err
}

// Update 更新 Cert。MySQL 没有 RETURNING，拆成 UPDATE + SELECT revision。
// App 层调用前已 Revision++（与 PG 一致），因此 WHERE 用 Revision-1 做旧值守卫；
// affected rows == 0 说明 revision 冲突。
func (r Repository) Update(ctx context.Context, cert *domain.Cert) error {
	domainJSON, err := json.Marshal(cert.Domain)
	if err != nil {
		return err
	}
	var domainStr string
	if domainJSON != nil {
		domainStr = string(domainJSON)
	}
	privateKey, accessSecret, err := certSecretsForWrite(cert)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, queryUpdateCert,
		cert.DisplayName, cert.Scope, cert.Type, cert.CryptoAlgorithm,
		cert.BitSize, cert.ExpireInYears, cert.ExpireTime, cert.Certificate,
		privateKey,
		cert.AuthorityPublicKey, cert.AuthorityRootPublicKey,
		cert.AcmeIssuer, cert.AcmeEndpoint,
		cert.Provider, cert.Account, cert.AccessKey,
		accessSecret, domainStr, cert.DomainExpireTime,
		cert.UpdatedAt, cert.ID, cert.Revision-1, cert.OrganizationID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("cert revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectCertRevision, cert.ID, cert.OrganizationID).Scan(&newRevision); err != nil {
		return err
	}
	cert.Revision = newRevision
	return nil
}

// Delete 删除 Cert（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	result, err := r.db.ExecContext(ctx, queryDeleteCert, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("cert not found")
	}
	return nil
}

// toCert 把 MySQL certRow 转成 domain.Cert，处理 domain JSON 反序列化。
func toCert(row certRow) (*domain.Cert, error) {
	cert := &domain.Cert{
		ID:                     row.ID,
		OrganizationID:         row.OrganizationID,
		Name:                   row.Name,
		DisplayName:            row.DisplayName,
		Scope:                  row.Scope,
		Type:                   row.Type,
		CryptoAlgorithm:        row.CryptoAlgorithm,
		BitSize:                row.BitSize,
		ExpireInYears:          row.ExpireInYears,
		Certificate:            row.Certificate,
		PrivateKey:             row.PrivateKey,
		AuthorityPublicKey:     row.AuthorityPublicKey,
		AuthorityRootPublicKey: row.AuthorityRootPublicKey,
		AcmeIssuer:             row.AcmeIssuer,
		AcmeEndpoint:           row.AcmeEndpoint,
		Provider:               row.Provider,
		Account:                row.Account,
		AccessKey:              row.AccessKey,
		AccessSecret:           row.AccessSecret,
		Revision:               row.Revision,
	}

	if row.ExpireTime != nil && row.ExpireTime.Valid {
		t := row.ExpireTime.Time
		cert.ExpireTime = &t
	}
	if row.DomainExpireTime != nil && row.DomainExpireTime.Valid {
		t := row.DomainExpireTime.Time
		cert.DomainExpireTime = &t
	}
	if row.CreatedAt.Valid {
		cert.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		cert.UpdatedAt = row.UpdatedAt.Time
	}

	if row.Domain != "" {
		if err := json.Unmarshal([]byte(row.Domain), &cert.Domain); err != nil {
			return nil, err
		}
	}
	if cert.Domain == nil {
		cert.Domain = []string{}
	}

	if err := decryptCertSecrets(cert); err != nil {
		return nil, err
	}

	return cert, nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}

// ListExpiring 扫描即将过期的证书。仅供维护作业在系统作用域调用：不做组织
// 过滤，跨租户返回候选行由调用方按 organization_id 分组投递。
func (r Repository) ListExpiring(ctx context.Context, now, deadline time.Time, limit int) ([]*domain.Cert, error) {
	rows, err := r.db.QueryContext(ctx, queryListExpiringCerts, now, deadline, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.Cert, 0)
	for rows.Next() {
		var row certRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.Scope, &row.Type, &row.CryptoAlgorithm, &row.BitSize, &row.ExpireInYears,
			&row.ExpireTime, &row.Certificate, &row.PrivateKey,
			&row.AuthorityPublicKey, &row.AuthorityRootPublicKey,
			&row.AcmeIssuer, &row.AcmeEndpoint,
			&row.Provider, &row.Account, &row.AccessKey, &row.AccessSecret, &row.Domain,
			&row.DomainExpireTime,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, err
		}
		cert, err := toCert(row)
		if err != nil {
			return nil, apperrors.NewInternal("decode cert")
		}
		items = append(items, cert)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

var _ domain.ExpiringScanner = Repository{}

package consents

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/consents"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 consents 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// consentRow 是 SELECT 的扫描目标。
// GrantedScopes MySQL 存 TEXT（JSON 字符串），PG 是 JSONB → []byte。
// 这里用 string 更方便 Scan。
type consentRow struct {
	ID               string
	UserID           string
	Username         string
	OrganizationID   string
	OrganizationSlug string
	ApplicationID    string
	ApplicationSlug  string
	GrantedScopes    string // JSON 数组字符串 → []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RevokedAt        *time.Time
	Revision         int64
}

// GetByUserAndApplication 按 user_id + application_id 查 consent。
func (r Repository) GetByUserAndApplication(ctx context.Context, userID, applicationID string) (*domain.Consent, error) {
	var row consentRow
	err := r.db.QueryRowContext(ctx, queryGetConsentByUserAndApplication, userID, applicationID).Scan(
		&row.ID,
		&row.UserID,
		&row.Username,
		&row.OrganizationID,
		&row.OrganizationSlug,
		&row.ApplicationID,
		&row.ApplicationSlug,
		&row.GrantedScopes,
		&row.CreatedAt,
		&row.UpdatedAt,
		&row.RevokedAt,
		&row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("consent not found")
		}
		return nil, err
	}

	return toConsent(row)
}

// CountActiveByApplication 按 application_id 统计未撤销的 consent。
func (r Repository) CountActiveByApplication(ctx context.Context, applicationID string) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, queryCountActiveConsentsByApplication, applicationID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Upsert 创建或更新 consent。MySQL 用 ON DUPLICATE KEY UPDATE 实现 PG ON CONFLICT DO UPDATE。
func (r Repository) Upsert(ctx context.Context, consent *domain.Consent) error {
	grantedScopesJSON, err := mysqlshared.MarshalJSON(consent.GrantedScopes)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryUpsertConsent,
		consent.ID,
		consent.UserID,
		consent.Username,
		consent.OrganizationID,
		consent.OrganizationSlug,
		consent.ApplicationID,
		consent.ApplicationSlug,
		string(grantedScopesJSON),
		consent.CreatedAt,
		consent.UpdatedAt,
		consent.RevokedAt,
		consent.Revision,
	)
	return err
}

// Revoke 撤销指定 user+app 的 consent。零 affected rows 报告未找到。
func (r Repository) Revoke(ctx context.Context, userID, applicationID string) error {
	result, err := r.db.ExecContext(ctx, queryRevokeConsent, userID, applicationID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("consent not found")
	}
	return nil
}

// toConsent 把 DB 扫描行（GrantedScopes 是 JSON 字符串）转成 domain 对象。
func toConsent(row consentRow) (*domain.Consent, error) {
	c := &domain.Consent{
		ID:               row.ID,
		UserID:           row.UserID,
		Username:         row.Username,
		OrganizationID:   row.OrganizationID,
		OrganizationSlug: row.OrganizationSlug,
		ApplicationID:    row.ApplicationID,
		ApplicationSlug:  row.ApplicationSlug,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		RevokedAt:        row.RevokedAt,
		Revision:         row.Revision,
	}
	if row.GrantedScopes != "" {
		if err := mysqlshared.UnmarshalJSON([]byte(row.GrantedScopes), &c.GrantedScopes); err != nil {
			return nil, err
		}
	} else {
		c.GrantedScopes = []string{}
	}
	return c, nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}

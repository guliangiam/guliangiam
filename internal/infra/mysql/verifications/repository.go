package verifications

import (
	"context"
	"database/sql"
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 verifications 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// verificationRow 是 verifications 表扫描目标。
type verificationRow struct {
	ID             string
	OrganizationID string
	Type           string
	Recipient      string
	Code           string
	Status         string
	ExpiresAt      sql.NullTime
	CreatedAt      sql.NullTime
}

// GetByID 按 id 查 Verification。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Verification, error) {
	return r.scanVerification(r.db.QueryRowContext(ctx, queryGetVerificationByID, id))
}

// GetByIDForOrganization 按 id + organization_id 查 Verification（防止跨租户读取）。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Verification, error) {
	return r.scanVerification(r.db.QueryRowContext(ctx, queryGetVerificationByIDForOrganization, id, organizationID))
}

func (r Repository) scanVerification(scanner interface{ Scan(...any) error }) (*domain.Verification, error) {
	var row verificationRow
	if err := scanner.Scan(
		&row.ID, &row.OrganizationID, &row.Type, &row.Recipient, &row.Code,
		&row.Status, &row.ExpiresAt, &row.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("verification not found")
		}
		return nil, err
	}
	v := toVerification(row)
	return &v, nil
}

// List 分页列出 Verification。
// countArgs: org×2 + type×2 + status×2 + recipient×3 = 9
// listArgs: countArgs + 2 = 11
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Verification, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	countArgs := []interface{}{
		query.OrganizationID, query.OrganizationID,
		query.Type, query.Type,
		query.Status, query.Status,
		query.Recipient, query.Recipient, query.Recipient,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountVerifications, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, queryListVerifications, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Verification, 0)
	for rows.Next() {
		var row verificationRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Type, &row.Recipient, &row.Code,
			&row.Status, &row.ExpiresAt, &row.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		v := toVerification(row)
		items = append(items, &v)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}
	return items, total, nil
}

// Create 插入 Verification。
func (r Repository) Create(ctx context.Context, verification *domain.Verification) error {
	_, err := r.db.ExecContext(ctx, queryCreateVerification,
		verification.ID, verification.OrganizationID, verification.Type,
		verification.Recipient, verification.Code, verification.Status,
		verification.ExpiresAt, verification.CreatedAt,
	)
	return err
}

// UpdateStatus 将 pending verification 转换到新状态。
// affected rows == 0 说明 verification 已被使用或过期。
func (r Repository) UpdateStatus(ctx context.Context, id, status string) error {
	result, err := r.db.ExecContext(ctx, queryUpdateVerificationStatus, status, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("verification code has already been used or expired")
	}
	return nil
}

// GetActiveByRecipientAndCode 查活跃的 (pending + 未过期) verification。
func (r Repository) GetActiveByRecipientAndCode(ctx context.Context, recipient, code, verificationType string) (*domain.Verification, error) {
	var row verificationRow
	err := r.db.QueryRowContext(ctx, queryGetActiveVerificationByRecipientAndCode,
		recipient, code, verificationType,
	).Scan(
		&row.ID, &row.OrganizationID, &row.Type, &row.Recipient, &row.Code,
		&row.Status, &row.ExpiresAt, &row.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("verification not found or expired")
		}
		return nil, err
	}
	v := toVerification(row)
	return &v, nil
}

// GetActiveByRecipient 查活跃的 (pending + 未过期) verification，不要求 code 匹配。
func (r Repository) GetActiveByRecipient(ctx context.Context, recipient, verificationType string) (*domain.Verification, error) {
	var row verificationRow
	err := r.db.QueryRowContext(ctx, queryGetActiveVerificationByRecipient, recipient, verificationType).Scan(
		&row.ID, &row.OrganizationID, &row.Type, &row.Recipient, &row.Code,
		&row.Status, &row.ExpiresAt, &row.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("verification not found or expired")
		}
		return nil, err
	}
	v := toVerification(row)
	return &v, nil
}

// UpdateCode 持久化 provider 已确认的验证码，后续同一流程回到普通 code 匹配路径。
func (r Repository) UpdateCode(ctx context.Context, id, code string) error {
	_, err := r.db.ExecContext(ctx, queryUpdateVerificationCode, code, id)
	return err
}

// RecordApplicationBinding 记录投递该验证码的应用与提供商；空串写入 NULL。
func (r Repository) RecordApplicationBinding(ctx context.Context, id, applicationID, providerID string) error {
	_, err := r.db.ExecContext(ctx, queryRecordVerificationBinding, applicationID, providerID, id)
	return err
}

// ApplicationBinding 返回验证码记录的应用/提供商绑定；未记录时为空串。
func (r Repository) ApplicationBinding(ctx context.Context, id string) (string, string, error) {
	var applicationID, providerID string
	err := r.db.QueryRowContext(ctx, queryGetVerificationApplicationBinding, id).Scan(&applicationID, &providerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", apperrors.NewNotFound("verification not found")
		}
		return "", "", err
	}
	return applicationID, providerID, nil
}

// toVerification 把 MySQL row 转成 domain.Verification。
func toVerification(row verificationRow) domain.Verification {
	v := domain.Verification{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Type:           row.Type,
		Recipient:      row.Recipient,
		Code:           row.Code,
		Status:         row.Status,
	}
	if row.ExpiresAt.Valid {
		v.ExpiresAt = row.ExpiresAt.Time
	}
	if row.CreatedAt.Valid {
		v.CreatedAt = row.CreatedAt.Time
	}
	return v
}

// 编译期接口断言
var _ domain.Repository = Repository{}

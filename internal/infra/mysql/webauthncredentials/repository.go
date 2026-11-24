package webauthncredentials

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 webauthn 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// Create 插入 WebAuthn credential。
func (r Repository) Create(ctx context.Context, c *domain.Credential) error {
	transport, err := mysqlshared.MarshalJSON(c.Transport)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertCredential,
		c.ID, c.UserID, c.CredentialID, c.PublicKey, c.AttestationType,
		transport, c.SignCount, c.Name, c.AAGUID, string(c.Status), c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// GetByCredentialID 按 credential_id（二进制）查 Credential。
func (r Repository) GetByCredentialID(ctx context.Context, credentialID []byte) (*domain.Credential, error) {
	var row credentialRow
	err := r.db.QueryRowContext(ctx, queryGetByCredentialID, credentialID).Scan(
		&row.ID, &row.UserID, &row.CredentialID, &row.PublicKey, &row.AttestationType,
		&row.Transport, &row.SignCount, &row.Name, &row.AAGUID, &row.Status, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("webauthn credential not found")
		}
		return nil, err
	}
	return rowToDomain(row)
}

// ListByUserID 按 user_id 列出 Credential。
func (r Repository) ListByUserID(ctx context.Context, userID string) ([]*domain.Credential, error) {
	rows, err := r.db.QueryContext(ctx, queryListByUserID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*domain.Credential, 0)
	for rows.Next() {
		var row credentialRow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.CredentialID, &row.PublicKey, &row.AttestationType,
			&row.Transport, &row.SignCount, &row.Name, &row.AAGUID, &row.Status, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		c, err := rowToDomain(row)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return result, nil
}

// UpdateSignCount 单调递增签名计数。
// PG 版用 Exec 检查 RowsAffected == 0 返回 Conflict，
// MySQL 版用同样的策略（WHERE id=? AND sign_count<?）。
func (r Repository) UpdateSignCount(ctx context.Context, id string, signCount int64) error {
	res, err := r.db.ExecContext(ctx, queryUpdateSignCount, signCount, time.Now().UTC(), id, signCount)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("webauthn signature counter is not monotonic: possible assertion replay")
	}
	return nil
}

// Update 持久化凭证的可变元数据（显示名与 updated_at）。状态与签名计数走各自的
// 专用方法，避免绕过状态机与单调计数保护。
func (r Repository) Update(ctx context.Context, c *domain.Credential) error {
	_, err := r.db.ExecContext(ctx, queryUpdateCredential, c.Name, c.UpdatedAt, c.ID)
	return err
}

// UpdateStatus 迁移凭证状态（active/revoked/deactivated），幂等：状态未变化时不写行。
func (r Repository) UpdateStatus(ctx context.Context, id string, status domain.CredentialStatus) error {
	_, err := r.db.ExecContext(ctx, queryUpdateCredentialStatus, string(status), time.Now().UTC(), id, string(status))
	return err
}

// Delete 删除 Credential。
func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteCredential, id)
	return err
}

// 编译期接口断言
var _ domain.Repository = Repository{}

// DeleteExpiredChallenges 删除 expires_at < cutoff 的挑战行（清理作业调用）。
func (r Repository) DeleteExpiredChallenges(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, queryDeleteExpiredChallenges, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CreateChallenge 插入一条待消费的 WebAuthn 挑战（只存哈希）。
func (r Repository) CreateChallenge(ctx context.Context, c *domain.Challenge) error {
	// organization_id 可空：空串必须传 NULL，而不是写进 UUID 列。
	var orgParam any
	if c.OrganizationID != "" {
		orgParam = c.OrganizationID
	}
	_, err := r.db.ExecContext(ctx, queryInsertChallenge,
		c.ID, c.ChallengeHash, c.CeremonyType, c.UserID, orgParam,
		c.RelyingPartyID, c.ExpiresAt, c.CreatedAt, c.UpdatedAt)
	return err
}

// ConsumeChallenge 原子消费挑战：MySQL 没有 UPDATE ... RETURNING，因此先用条件 UPDATE
// 抢占（RowsAffected=0 即已被消费/过期/不存在），再由抢占成功者读回该行。
func (r Repository) ConsumeChallenge(ctx context.Context, id string, now time.Time) (*domain.Challenge, error) {
	res, err := r.db.ExecContext(ctx, queryConsumeChallenge, now, now, id, now)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, apperrors.NewNotFound("webauthn challenge not found, already consumed, or expired")
	}

	var row challengeRow
	err = r.db.QueryRowContext(ctx, queryGetChallengeByID, id).Scan(
		&row.ID, &row.ChallengeHash, &row.CeremonyType, &row.UserID, &row.OrganizationID,
		&row.RelyingPartyID, &row.ExpiresAt, &row.ConsumedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("webauthn challenge not found, already consumed, or expired")
		}
		return nil, err
	}
	return challengeRowToDomain(row), nil
}

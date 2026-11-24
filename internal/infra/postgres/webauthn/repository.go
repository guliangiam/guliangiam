package webauthn

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) Create(ctx context.Context, c *domain.Credential) error {
	_, err := r.db.Exec(ctx, queryInsertCredential,
		c.ID, c.UserID, c.CredentialID, c.PublicKey, c.AttestationType,
		c.Transport, c.SignCount, c.Name, parseUUID(c.AAGUID), string(c.Status), c.CreatedAt, c.UpdatedAt)
	// uq_webauthn_credentials_credential_id 兜住并发/绕过预检的重复注册：
	// 23505 → 409，而不是 raw PgError 变成 500。
	return pgerror.Map(err, "uq_webauthn_credentials_credential_id")
}

func (r Repository) GetByCredentialID(ctx context.Context, credentialID []byte) (*domain.Credential, error) {
	var row credentialRow
	var aaguid pgtype.UUID
	err := r.db.QueryRow(ctx, queryGetByCredentialID, credentialID).Scan(
		&row.ID, &row.UserID, &row.CredentialID, &row.PublicKey, &row.AttestationType,
		&row.Transport, &row.SignCount, &row.Name, &aaguid, &row.Status, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("webauthn credential not found")
		}
		return nil, err
	}
	return rowToDomain(row, aaguid), nil
}

func (r Repository) ListByUserID(ctx context.Context, userID string) ([]*domain.Credential, error) {
	rows, err := r.db.Query(ctx, queryListByUserID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Credential
	for rows.Next() {
		var row credentialRow
		var aaguid pgtype.UUID
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.CredentialID, &row.PublicKey, &row.AttestationType,
			&row.Transport, &row.SignCount, &row.Name, &aaguid, &row.Status, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, rowToDomain(row, aaguid))
	}
	return result, rows.Err()
}

// UpdateSignCount advances the stored signature counter. The `sign_count < $1`
// guard keeps the counter monotonic, which is the precondition FIDO2 clone
// detection relies on: a replayed assertion carries an already-seen counter and
// must not write a lower value back over a higher one.
func (r Repository) UpdateSignCount(ctx context.Context, id string, signCount int64) error {
	result, err := r.db.Exec(ctx, queryUpdateSignCount, signCount, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewConflict("webauthn signature counter is not monotonic: possible assertion replay")
	}
	return nil
}

// Update 持久化凭证的可变元数据（显示名与 updated_at）。状态与签名计数走各自的
// 专用方法，避免绕过状态机与单调计数保护。
func (r Repository) Update(ctx context.Context, c *domain.Credential) error {
	_, err := r.db.Exec(ctx, queryUpdateCredential, c.Name, c.UpdatedAt, c.ID)
	return err
}

// UpdateStatus 迁移凭证状态（active/revoked/deactivated）。幂等：目标状态与当前一致时
// 不写行（RowsAffected=0），因此克隆嫌疑与管理员吊销可以重复触发而不会互相报错。
func (r Repository) UpdateStatus(ctx context.Context, id string, status domain.CredentialStatus) error {
	_, err := r.db.Exec(ctx, queryUpdateCredentialStatus, string(status), time.Now().UTC(), id)
	return err
}

func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryDeleteCredential, id)
	return err
}

// DeleteExpiredChallenges 删除 expires_at < cutoff 的挑战行（清理作业调用）。
func (r Repository) DeleteExpiredChallenges(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.Exec(ctx, queryDeleteExpiredChallenges, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r Repository) CreateChallenge(ctx context.Context, c *domain.Challenge) error {
	_, err := r.db.Exec(ctx, queryInsertChallenge,
		c.ID, c.ChallengeHash, c.CeremonyType, c.UserID, parseUUID(c.OrganizationID),
		c.RelyingPartyID, c.ExpiresAt, c.CreatedAt, c.UpdatedAt)
	return err
}

// ConsumeChallenge 原子消费挑战：条件 UPDATE 命中一行才返回，否则 NotFound。
func (r Repository) ConsumeChallenge(ctx context.Context, id string, now time.Time) (*domain.Challenge, error) {
	var c domain.Challenge
	err := r.db.QueryRow(ctx, queryConsumeChallenge, id, now).Scan(
		&c.ID, &c.ChallengeHash, &c.CeremonyType, &c.UserID, &c.OrganizationID,
		&c.RelyingPartyID, &c.ExpiresAt, &c.ConsumedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("webauthn challenge not found, already consumed, or expired")
		}
		return nil, err
	}
	return &c, nil
}

type credentialRow struct {
	ID              string
	UserID          string
	CredentialID    []byte
	PublicKey       []byte
	AttestationType string
	Transport       []string
	SignCount       int64
	Name            string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func rowToDomain(row credentialRow, aaguid pgtype.UUID) *domain.Credential {
	aaguidStr := ""
	if aaguid.Valid {
		aaguidStr = aaguid.String()
	}
	return &domain.Credential{
		ID:              row.ID,
		UserID:          row.UserID,
		CredentialID:    row.CredentialID,
		PublicKey:       row.PublicKey,
		AttestationType: row.AttestationType,
		Transport:       row.Transport,
		SignCount:       row.SignCount,
		Name:            row.Name,
		AAGUID:          aaguidStr,
		Status:          domain.CredentialStatus(row.Status),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func parseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if s == "" {
		u.Valid = false
		return u
	}
	_ = u.Scan(s)
	return u
}

package mfachallenges

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 login_challenges 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// challengeRow 是 login_challenges 表扫描目标。
type challengeRow struct {
	ID             string
	OrganizationID string
	UserID         string
	ApplicationID  string
	State          string
	MethodType     string
	Metadata       []byte
	ExpiresAt      time.Time
	VerifiedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// GetByID 按 id 查 Challenge。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Challenge, error) {
	var row challengeRow
	err := r.db.QueryRowContext(ctx, queryGetChallengeByID, id).Scan(
		&row.ID, &row.OrganizationID, &row.UserID, &row.ApplicationID,
		&row.State, &row.MethodType, &row.Metadata, &row.ExpiresAt, &row.VerifiedAt,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("login challenge not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

// Create 插入 Challenge。
func (r Repository) Create(ctx context.Context, challenge *domain.Challenge) error {
	metadata, err := challengeMetadataJSON(challenge)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertChallenge,
		challenge.ID, challenge.OrganizationID, challenge.UserID, challenge.ApplicationID,
		challenge.State, challenge.MethodType, metadata, challenge.ExpiresAt, challenge.VerifiedAt,
		challenge.CreatedAt, challenge.UpdatedAt, challenge.Revision,
	)
	return err
}

// Update 乐观锁更新 Challenge。
// PG RETURNING revision → MySQL UPDATE + SELECT 两步。
func (r Repository) Update(ctx context.Context, challenge *domain.Challenge) error {
	metadata, err := challengeMetadataJSON(challenge)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, queryUpdateChallenge,
		challenge.State, challenge.MethodType, metadata, challenge.ExpiresAt, challenge.VerifiedAt,
		challenge.UpdatedAt, challenge.ID, challenge.Revision,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("login challenge revision conflict")
	}

	// SELECT 新 revision
	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevisionAfterUpdate, challenge.ID).Scan(&newRevision); err != nil {
		return err
	}
	challenge.Revision = newRevision
	return nil
}

// Delete 按 id 删除 Challenge。
func (r Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, queryDeleteChallenge, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("login challenge not found")
	}
	return nil
}

// toDomain 把 MySQL row 转成 domain.Challenge。
func toDomain(row challengeRow) *domain.Challenge {
	challenge := &domain.Challenge{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		UserID:         row.UserID,
		ApplicationID:  row.ApplicationID,
		State:          row.State,
		MethodType:     row.MethodType,
		ExpiresAt:      row.ExpiresAt,
		VerifiedAt:     row.VerifiedAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	hydrateMetadata(challenge, row.Metadata)
	return challenge
}

func hydrateMetadata(challenge *domain.Challenge, raw []byte) {
	if len(raw) == 0 {
		return
	}
	if err := mysqlshared.UnmarshalJSON(raw, &challenge.Metadata); err != nil {
		return
	}
	if challenge.Metadata == nil {
		return
	}
	if attempts, ok := metadataInt(challenge.Metadata[domain.MetadataKeyFailedAttempts]); ok {
		challenge.FailedAttempts = attempts
	}
}

func metadataInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	default:
		return 0, false
	}
}

func challengeMetadataJSON(challenge *domain.Challenge) ([]byte, error) {
	if len(challenge.Metadata) == 0 {
		return []byte("{}"), nil
	}
	return mysqlshared.MarshalJSON(challenge.Metadata)
}

// 编译期接口断言
var _ domain.Repository = Repository{}

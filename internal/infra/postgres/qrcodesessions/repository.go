package qrcodesessions

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// Repository implements domain.Repository with PostgreSQL.
type Repository struct {
	db shared.DBTX
}

// NewRepository creates a new QR code session repository.
func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByCodeID(ctx context.Context, codeID string) (*domain.Session, error) {
	var row sessionRow
	err := r.db.QueryRow(ctx, queryGetByCodeID, codeID).Scan(
		&row.ID, &row.CodeID, &row.Code, &row.OrganizationID, &row.ApplicationID,
		&row.ExpiresAt, &row.Status, &row.UserID, &row.Username,
		&row.IPAddress, &row.UserAgent, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("qr code session not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r Repository) GetByCode(ctx context.Context, code string) (*domain.Session, error) {
	var row sessionRow
	err := r.db.QueryRow(ctx, queryGetByCode, code).Scan(
		&row.ID, &row.CodeID, &row.Code, &row.OrganizationID, &row.ApplicationID,
		&row.ExpiresAt, &row.Status, &row.UserID, &row.Username,
		&row.IPAddress, &row.UserAgent, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("qr code session not found")
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r Repository) Create(ctx context.Context, session *domain.Session) error {
	// user_id 是 uuid 列：匿名创建时为空串，直接绑定会触发 22P02，必须落 NULL。
	var userID any
	if session.UserID != "" {
		userID = session.UserID
	}
	_, err := r.db.Exec(ctx, queryCreate,
		session.ID, session.CodeID, session.Code, session.OrganizationID,
		session.ApplicationID, session.ExpiresAt, session.Status,
		userID, session.Username, session.IPAddress, session.UserAgent,
		session.CreatedAt, session.UpdatedAt, session.Revision,
	)
	return err
}

func (r Repository) UpdateStatus(ctx context.Context, session *domain.Session) error {
	tag, err := r.db.Exec(ctx, queryUpdateStatus,
		session.CodeID, session.Revision,
		session.Status, session.UserID, session.Username,
		session.IPAddress, session.UserAgent, session.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewConflict("qr code session was modified by another request")
	}
	return nil
}

func (r Repository) DeleteByCodeID(ctx context.Context, codeID string) error {
	_, err := r.db.Exec(ctx, queryDeleteByCodeID, codeID)
	return err
}

func (r Repository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, queryDeleteExpired)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

var _ domain.Repository = Repository{}

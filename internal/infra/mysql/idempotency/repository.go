// Package idempotency is ADR-018 unified idempotency的 MySQL 8.4+ DB 实现。
// Mirrors internal/infra/postgres/idempotency.
package idempotency

import (
	"context"
	"database/sql"
	stderrors "errors"
	"time"

	guuid "github.com/google/uuid"

	domainidem "github.com/guliangiam/guliangiam/internal/domain/idempotency"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 实现的 idempotency.Repository。
type Repository struct {
	db mysqlshared.DBTX
}

func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

var _ domainidem.Repository = Repository{}

// orgIDArg 把 *string orgID 变成 sql driver 可接受的值（nil-safe）。
func orgIDArg(orgID *string) any {
	if orgID == nil {
		return nil
	}
	return *orgID
}

func scanRecord(row interface{ Scan(dest ...any) error }) (*domainidem.Record, error) {
	var r domainidem.Record
	var orgID sql.NullString
	var respStatus sql.NullInt64
	var respBody sql.NullString
	var contentType sql.NullString
	var completedAt sql.NullTime
	var orgPtr *string

	if err := row.Scan(
		&r.ID, &orgID, &r.Scope, &r.Key,
		&r.RequestMethod, &r.RequestPath, &r.RequestHash,
		&r.Status, &respStatus, &respBody, &contentType,
		&completedAt, &r.ExpiresAt, &r.CreatedAt,
	); err != nil {
		return nil, err
	}
	if orgID.Valid {
		orgPtr = &orgID.String
	}
	r.OrganizationID = orgPtr
	if respStatus.Valid {
		v := int(respStatus.Int64)
		r.ResponseStatus = &v
	}
	if respBody.Valid {
		v := respBody.String
		r.ResponseBody = &v
	}
	if contentType.Valid && contentType.String != "" {
		v := contentType.String
		r.ContentType = &v
	}
	if completedAt.Valid {
		t := completedAt.Time
		r.CompletedAt = &t
	}
	return &r, nil
}

func (r Repository) GetByScopeAndKey(ctx context.Context, scope, key string) (*domainidem.Record, error) {
	row := r.db.QueryRowContext(ctx, QueryGetByScopeKey, scope, key)
	rec, err := scanRecord(row)
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, apperrors.WrapInternalf(err, "idempotency get")
	}
	return rec, nil
}

func (r Repository) TryAcquire(ctx context.Context, rec *domainidem.Record) (*domainidem.Record, bool, error) {
	if rec.ID == "" {
		id, err := guuid.NewV7()
		if err != nil {
			return nil, false, apperrors.WrapInternalf(err, "idempotency: generate id")
		}
		rec.ID = id.String()
	}

	row := r.db.QueryRowContext(ctx, QueryTryAcquire,
		rec.ID, orgIDArg(rec.OrganizationID), rec.Scope, rec.Key,
		rec.RequestMethod, rec.RequestPath, rec.RequestHash,
	)
	created, err := scanRecord(row)
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			// MySQL ON DUPLICATE KEY UPDATE scope=scope + RETURNING — 冲突时返回 no rows
			return r.readOnConflict(ctx, rec.Scope, rec.Key)
		}
		return nil, false, apperrors.WrapInternalf(err, "idempotency: try acquire")
	}
	return created, true, nil
}

func (r Repository) readOnConflict(ctx context.Context, scope, key string) (*domainidem.Record, bool, error) {
	existing, err := r.GetByScopeAndKey(ctx, scope, key)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		return nil, false, apperrors.NewConflict("idempotency: key exists but not readable")
	}
	return existing, false, nil
}

func (r Repository) Complete(ctx context.Context, id string, status domainidem.Status, respStatus int, respBody, respContentType string) error {
	// 空 Content-Type 存 NULL：旧行为等价，回放时不设置显式头（BE-E-12）。
	var contentType any
	if respContentType != "" {
		contentType = respContentType
	}
	_, err := r.db.ExecContext(ctx, QueryComplete,
		string(status), respStatus, respBody, contentType, id,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "idempotency complete")
	}
	return nil
}

// DeleteExpired removes one bounded batch of expired idempotency keys.
// Implements domainidem.ExpiredCleaner.
func (r Repository) DeleteExpired(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	res, err := r.db.ExecContext(ctx, QueryDeleteExpired, cutoff, limit)
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "idempotency: delete expired")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "idempotency: delete expired rows")
	}
	return affected, nil
}

var _ domainidem.ExpiredCleaner = Repository{}

package enforcers

import (
	"context"
	"database/sql"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type enforcerRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	ModelID        sql.NullString // nullable FK → models(id) ON DELETE SET NULL
	AdapterID      sql.NullString // nullable FK → adapters(id) ON DELETE SET NULL
	IsEnabled      bool
	Owner          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

type scannable interface {
	Scan(dest ...any) error
}

func scanEnforcerRow(s scannable) (enforcerRow, error) {
	var row enforcerRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description,
		&row.ModelID, &row.AdapterID, &row.IsEnabled, &row.Owner,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	return row, err
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Enforcer, error) {
	return r.getByIDQuery(ctx, queryGetEnforcerByID, id)
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Enforcer, error) {
	return r.getByIDQuery(ctx, queryGetEnforcerByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Enforcer, error) {
	row, err := scanEnforcerRow(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("enforcer not found")
		}
		return nil, err
	}
	return rowToDomain(row), nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Enforcer, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListEnforcers,
		query.OrganizationID, query.Keyword, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Enforcer, 0)
	for rows.Next() {
		row, err := scanEnforcerRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, rowToDomain(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountEnforcers,
		query.OrganizationID, query.Keyword, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, e *domain.Enforcer) error {
	_, err := r.db.Exec(ctx, queryInsertEnforcer,
		e.ID, e.OrganizationID, e.Name, e.DisplayName, e.Description,
		nullableUUID(e.ModelID), nullableUUID(e.AdapterID), e.IsEnabled, e.Owner,
		e.CreatedAt, e.UpdatedAt, e.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, e *domain.Enforcer) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryUpdateEnforcer,
		e.DisplayName, e.Description, nullableUUID(e.ModelID), nullableUUID(e.AdapterID), e.IsEnabled,
		e.Owner, e.UpdatedAt, e.Revision-1, e.ID, e.OrganizationID).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("enforcer revision conflict")
		}
		return err
	}
	e.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteEnforcer, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("enforcer not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

func rowToDomain(row enforcerRow) *domain.Enforcer {
	return &domain.Enforcer{
		ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
		DisplayName: row.DisplayName, Description: row.Description,
		ModelID: nullOrString(row.ModelID), AdapterID: nullOrString(row.AdapterID),
		IsEnabled: row.IsEnabled, Owner: row.Owner,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
	}
}

func nullOrString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// nullableUUID 把空字符串转成 NULL：model_id / adapter_id 是 uuid 列，直接写 ""
// 会触发 SQLSTATE 22P02（invalid input syntax for type uuid）。
func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

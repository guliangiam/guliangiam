package models

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/models"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// pgerrorConstraints is the whitelist of constraint identifiers this repository
// may surface in error metadata; anything else stays in the server log.
var pgerrorConstraints = []string{
	"uq_models_org_name",
	"fk_models_organization_id_organizations",
}

type modelRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	ModelText      string
	// Phase 2: Extended fields
	Owner     string
	IsEnabled bool
	CreatedAt time.Time
	UpdatedAt time.Time
	Revision  int64
}

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

type scannable interface {
	Scan(dest ...any) error
}

func scanModelRow(s scannable) (modelRow, error) {
	var row modelRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description,
		&row.ModelText, &row.Owner, &row.IsEnabled,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	return row, err
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Model, error) {
	return r.getByIDQuery(ctx, queryGetModelByID, id)
}

// GetByIDForOrganization returns the model only when it belongs to the given
// tenant; a foreign or missing row is reported as not found.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Model, error) {
	return r.getByIDQuery(ctx, queryGetModelByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Model, error) {
	row, err := scanModelRow(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("model not found")
		}
		return nil, err
	}
	return &domain.Model{
		ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
		DisplayName: row.DisplayName, Description: row.Description,
		ModelText: row.ModelText, Owner: row.Owner, IsEnabled: row.IsEnabled,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
	}, nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Model, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListModels, query.OrganizationID, query.Keyword, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Model, 0)
	for rows.Next() {
		row, err := scanModelRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, &domain.Model{
			ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
			DisplayName: row.DisplayName, Description: row.Description,
			ModelText: row.ModelText, Owner: row.Owner, IsEnabled: row.IsEnabled,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountModels, query.OrganizationID, query.Keyword, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, m *domain.Model) error {
	_, err := r.db.Exec(ctx, queryInsertModel,
		m.ID, m.OrganizationID, m.Name, m.DisplayName, m.Description,
		m.ModelText, m.Owner, m.IsEnabled, m.CreatedAt, m.UpdatedAt, m.Revision)
	return pgerror.Map(err, pgerrorConstraints...)
}

func (r Repository) Update(ctx context.Context, m *domain.Model) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryUpdateModel,
		m.DisplayName, m.Description, m.ModelText, m.Owner, m.IsEnabled,
		m.UpdatedAt, m.Revision-1, m.ID, m.OrganizationID).Scan(&newRevision)
	if err != nil {
		// The app layer read the entity scoped first: ErrNoRows is only a
		// same-tenant revision conflict.
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("model revision conflict")
		}
		return pgerror.Map(err, pgerrorConstraints...)
	}
	m.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteModel, id, organizationID)
	if err != nil {
		return pgerror.Map(err, pgerrorConstraints...)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("model not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

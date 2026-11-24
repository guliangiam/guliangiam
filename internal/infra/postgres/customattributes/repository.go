package customattributes

import (
	"context"
	"encoding/json"
	"fmt"

	domain "github.com/guliangiam/guliangiam/internal/domain/customattributes"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) ListByOrganization(ctx context.Context, organizationID string, includeDisabled bool) ([]*domain.Definition, error) {
	rows, err := r.db.Query(ctx, queryListByOrganization, organizationID, includeDisabled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	definitions := make([]*domain.Definition, 0)
	for rows.Next() {
		definition, err := scanDefinition(rows)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	return definitions, rows.Err()
}

func (r Repository) GetByOrganizationAndKey(ctx context.Context, organizationID, key string) (*domain.Definition, error) {
	definition, err := scanDefinition(r.db.QueryRow(ctx, queryGetByOrganizationAndKey, organizationID, key))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("custom attribute definition not found")
		}
		return nil, err
	}
	return definition, nil
}

func (r Repository) Create(ctx context.Context, definition *domain.Definition) error {
	options, err := marshalOptions(definition.Options)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsert,
		definition.ID, definition.OrganizationID, definition.Key, string(definition.DataType), definition.Required,
		options, definition.Writable, string(definition.Visibility), definition.Description, definition.Enabled,
		definition.CreatedAt, definition.UpdatedAt, definition.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, definition *domain.Definition) error {
	options, err := marshalOptions(definition.Options)
	if err != nil {
		return err
	}
	previousRevision := definition.Revision - 1
	tag, err := r.db.Exec(ctx, queryUpdate,
		definition.ID, definition.OrganizationID, definition.Key, string(definition.DataType), definition.Required, options,
		definition.Writable, string(definition.Visibility), definition.Description, definition.Enabled, definition.UpdatedAt,
		previousRevision)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewConflict("custom attribute definition was modified concurrently")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, organizationID, id string) error {
	tag, err := r.db.Exec(ctx, queryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("custom attribute definition not found")
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDefinition(row rowScanner) (*domain.Definition, error) {
	var definition domain.Definition
	var dataType, visibility string
	var options []byte
	if err := row.Scan(&definition.ID, &definition.OrganizationID, &definition.Key, &dataType, &definition.Required,
		&options, &definition.Writable, &visibility, &definition.Description, &definition.Enabled,
		&definition.CreatedAt, &definition.UpdatedAt, &definition.Revision); err != nil {
		return nil, err
	}
	definition.DataType = domain.DataType(dataType)
	definition.Visibility = domain.Visibility(visibility)
	if len(options) > 0 {
		if err := json.Unmarshal(options, &definition.Options); err != nil {
			return nil, fmt.Errorf("decode custom attribute options: %w", err)
		}
	}
	if definition.Options == nil {
		definition.Options = []string{}
	}
	return &definition, nil
}

func marshalOptions(options []string) ([]byte, error) {
	if options == nil {
		options = []string{}
	}
	return json.Marshal(options)
}

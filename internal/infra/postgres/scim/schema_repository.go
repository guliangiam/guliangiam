package scim

import (
	"context"
	"encoding/json"
	"errors" // stdlib errors retained for errors.Is/errors.As only; apperrors used for error creation
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/domain/scim"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetSchemaBySchemaID(ctx context.Context, schemaID string) (*scim.ScimSchema, error) {
	var model ScimSchemaModel
	err := r.pool.QueryRow(ctx, QueryGetScimSchemaBySchemaID, schemaID).Scan(
		&model.ID,
		&model.OrganizationID,
		&model.SchemaID,
		&model.Name,
		&model.Description,
		&model.Attributes,
		&model.CreatedAt,
		&model.UpdatedAt,
		&model.Revision,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSchemaNotFound
		}
		return nil, apperrors.WrapInternalf(err, "failed to get scim schema")
	}

	return r.modelToSchema(&model)
}

func (r *Repository) ListSchemas(ctx context.Context, organizationID string, filter *scim.ListQuery) ([]*scim.ScimSchema, int64, error) {
	baseQuery := `FROM guliangiam.scim_schemas WHERE organization_id = $1`
	countQuery := `SELECT COUNT(*) ` + baseQuery
	selectQuery := `SELECT id, organization_id, schema_id, name, description, attributes, created_at, updated_at, revision ` + baseQuery

	// Apply filter if provided
	args := []interface{}{organizationID}
	argPos := 2

	if filter != nil && filter.Filter != "" {
		// Basic filter implementation (can be extended)
		whereClause := r.buildFilterWhere(filter.Filter, argPos)
		selectQuery += " AND " + whereClause.clause
		countQuery += " AND " + whereClause.clause
		args = append(args, whereClause.args...)
		argPos = whereClause.nextPos
	}

	// Get total count
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "failed to count scim schemas")
	}

	// Add ordering and pagination
	selectQuery += " ORDER BY name"
	if filter != nil && filter.SortOrder == "descending" {
		selectQuery += " DESC"
	} else {
		selectQuery += " ASC"
	}

	if filter != nil && filter.Count > 0 {
		selectQuery += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Count)
		argPos++
	}

	if filter != nil && filter.StartIndex > 0 {
		selectQuery += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.StartIndex-1) // SCIM uses 1-based index
	}

	// Execute query
	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "failed to list scim schemas")
	}
	defer rows.Close()

	var schemas []*scim.ScimSchema
	for rows.Next() {
		var model ScimSchemaModel
		err := rows.Scan(
			&model.ID,
			&model.OrganizationID,
			&model.SchemaID,
			&model.Name,
			&model.Description,
			&model.Attributes,
			&model.CreatedAt,
			&model.UpdatedAt,
			&model.Revision,
		)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "failed to scan scim schema")
		}

		schema, err := r.modelToSchema(&model)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "failed to convert scim schema")
		}
		schemas = append(schemas, schema)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return schemas, total, nil
}

func (r *Repository) CreateSchema(ctx context.Context, schema *scim.ScimSchema) error {
	attributesJSON, err := json.Marshal(schema.Attributes)
	if err != nil {
		return apperrors.WrapInternalf(err, "failed to marshal attributes")
	}

	now := time.Now()
	_, err = r.pool.Exec(ctx, QueryCreateScimSchema,
		schema.ID,
		schema.OrganizationID,
		schema.SchemaID,
		schema.Name,
		schema.Description,
		attributesJSON,
		now,
		now,
		schema.Revision,
	)

	if err != nil {
		return apperrors.WrapInternalf(err, "failed to create scim schema")
	}

	schema.CreatedAt = now
	schema.UpdatedAt = now

	return nil
}

func (r *Repository) UpdateSchema(ctx context.Context, schema *scim.ScimSchema) error {
	attributesJSON, err := json.Marshal(schema.Attributes)
	if err != nil {
		return apperrors.WrapInternalf(err, "failed to marshal attributes")
	}

	var newRevision int64
	err = r.pool.QueryRow(ctx, QueryUpdateScimSchema,
		schema.SchemaID,
		schema.Name,
		schema.Description,
		attributesJSON,
		time.Now(),
		schema.OrganizationID,
	).Scan(&newRevision)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSchemaNotFound
		}
		return apperrors.WrapInternalf(err, "failed to update scim schema")
	}

	schema.Revision = newRevision
	schema.UpdatedAt = time.Now()

	return nil
}

func (r *Repository) DeleteSchema(ctx context.Context, schemaID string) error {
	result, err := r.pool.Exec(ctx, QueryDeleteScimSchema, schemaID)
	if err != nil {
		return apperrors.WrapInternalf(err, "failed to delete scim schema")
	}

	if result.RowsAffected() == 0 {
		return ErrSchemaNotFound
	}

	return nil
}

// ==================== ScimUser Operations ====================

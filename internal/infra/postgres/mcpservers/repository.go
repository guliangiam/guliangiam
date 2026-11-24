package mcpservers

import (
	"context"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id string) (*domain.McpServer, error) {
	s, err := scanMcpServer(r.db.QueryRow(ctx, QueryGetByID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("mcp server not found")
		}
		return nil, apperrors.WrapInternalf(err, "decode mcp server")
	}
	return s, nil
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.McpServer, error) {
	s, err := scanMcpServer(r.db.QueryRow(ctx, QueryGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("mcp server not found")
		}
		return nil, apperrors.WrapInternalf(err, "decode mcp server")
	}
	return s, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.McpServer, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.IsEnabled, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.McpServer, 0)
	for rows.Next() {
		s, err := scanMcpServer(rows)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "decode mcp server")
		}
		items = append(items, s)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, s *domain.McpServer) error {
	tools, _ := json.Marshal(s.Tools)
	token, err := sealMcpServerToken(s)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, QueryInsert, s.ID, s.OrganizationID, s.Name, s.DisplayName, s.URL, token, s.Application, tools, s.IsEnabled, s.CreatedAt, s.UpdatedAt, s.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, s *domain.McpServer) error {
	tools, _ := json.Marshal(s.Tools)
	token, err := sealMcpServerToken(s)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, QueryUpdate, s.DisplayName, s.URL, token, s.Application, tools, s.IsEnabled, s.UpdatedAt, s.Revision, s.ID, s.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("mcp server not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("mcp server not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

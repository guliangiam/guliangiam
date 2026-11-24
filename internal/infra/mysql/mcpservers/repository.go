package mcpservers

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 mcp_servers 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 McpServer。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.McpServer, error) {
	s, err := scanMcpServerRow(r.db.QueryRowContext(ctx, queryGetMcpServerByID, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("mcp server not found")
		}
		return nil, apperrors.WrapInternalf(err, "decode mcp server")
	}
	return s, nil
}

// GetByIDForOrganization 按 id + 可信 organization_id 查 McpServer。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.McpServer, error) {
	s, err := scanMcpServerRow(r.db.QueryRowContext(ctx, queryGetMcpServerByIDForOrganization, id, organizationID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("mcp server not found")
		}
		return nil, apperrors.WrapInternalf(err, "decode mcp server")
	}
	return s, nil
}

// List 分页列出 McpServer。MySQL 版需要手动展开参数重复占位符。
// countArgs: org_id×2 + keyword×3 + is_enabled×2 = 7
// listArgs: 同上 + limit + offset = 9
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.McpServer, int64, error) {
	var enabledArg interface{}
	if q.IsEnabled != nil {
		enabledArg = *q.IsEnabled
	} // 否则传 nil

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		enabledArg, enabledArg,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountMcpServers, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListMcpServers, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.McpServer, 0)
	for rows.Next() {
		s, err := scanMcpServerRow(rows)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "decode mcp server")
		}
		items = append(items, s)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 McpServer，tools 用 JSON 字符串，token 写入前封装。
func (r Repository) Create(ctx context.Context, s *domain.McpServer) error {
	toolsJSON, err := json.Marshal(s.Tools)
	if err != nil {
		return err
	}
	token, err := sealMcpServerToken(s)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertMcpServer,
		s.ID, s.OrganizationID, s.Name, s.DisplayName, s.URL, token, s.Application,
		string(toolsJSON), s.IsEnabled, s.CreatedAt, s.UpdatedAt, s.Revision,
	)
	return err
}

// Update 更新 McpServer。PG 版没有 RETURNING，MySQL 直接 ExecContext。
func (r Repository) Update(ctx context.Context, s *domain.McpServer) error {
	toolsJSON, err := json.Marshal(s.Tools)
	if err != nil {
		return err
	}
	token, err := sealMcpServerToken(s)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, queryUpdateMcpServer,
		s.DisplayName, s.URL, token, s.Application, string(toolsJSON),
		s.IsEnabled, s.UpdatedAt, s.Revision, s.ID, s.OrganizationID,
	)
	if err != nil {
		return err
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return rowsErr
	}
	if rows == 0 {
		return apperrors.NewNotFound("mcp server not found")
	}
	return nil
}

// Delete 删除 McpServer，organization_id 参与谓词。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	result, err := r.db.ExecContext(ctx, queryDeleteMcpServer, id, organizationID)
	if err != nil {
		return err
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return rowsErr
	}
	if rows == 0 {
		return apperrors.NewNotFound("mcp server not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}

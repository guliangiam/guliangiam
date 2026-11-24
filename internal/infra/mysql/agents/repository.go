package agents

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/agents"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 agents 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按主键取 agent（organization_id 谓词防止跨租户读取）。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Agent, error) {
	var row agentRow
	err := r.db.QueryRowContext(ctx, queryGetAgentByID, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Type,
		&row.Host, &row.Port, &row.Url, &row.Token, &row.ApplicationID,
		&row.IsEnabled, &row.LastPingAt,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("agent not found")
		}
		return nil, err
	}
	return toAgent(row)
}

// List 分页列出 agents。
// queryListAgents 参数：orgID×2, keyword×3, is_enabled×2, limit, offset = 9
// queryCountAgents 参数：orgID×2, keyword×3, is_enabled×2 = 7
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Agent, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	var isEnabled any
	if query.IsEnabled != nil {
		isEnabled = *query.IsEnabled
	}

	rows, err := r.db.QueryContext(ctx, queryListAgents,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Agent, 0)
	for rows.Next() {
		var row agentRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Type,
			&row.Host, &row.Port, &row.Url, &row.Token, &row.ApplicationID,
			&row.IsEnabled, &row.LastPingAt,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		agent, err := toAgent(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, agent)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountAgents,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create 插入 agent。
func (r Repository) Create(ctx context.Context, agent *domain.Agent) error {
	token, err := agentTokenForWrite(agent)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertAgent,
		agent.ID, agent.OrganizationID, agent.Name, agent.DisplayName, agent.Type,
		agent.Host, agent.Port, agent.Url, token, agent.ApplicationID,
		agent.IsEnabled, agent.LastPingAt,
		agent.CreatedAt, agent.UpdatedAt, agent.Revision,
	)
	return err
}

// Update 乐观锁更新。UPDATE + SELECT revision。
func (r Repository) Update(ctx context.Context, agent *domain.Agent) error {
	token, err := agentTokenForWrite(agent)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, queryUpdateAgent,
		agent.DisplayName, agent.Host, agent.Port,
		agent.Url, token, agent.ApplicationID,
		agent.IsEnabled, agent.LastPingAt,
		agent.UpdatedAt, agent.ID, agent.Revision, agent.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// 实体已在 app 层按租户读取：0 affected 只代表 revision 冲突。
		return apperrors.NewConflict("agent revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, agent.ID, agent.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("agent not found")
		}
		return err
	}
	agent.Revision = newRevision
	return nil
}

// Delete 删除 agent（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteAgent, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("agent not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

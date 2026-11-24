package actions

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/actions"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// Repository 是 MySQL 8.4+ 实现的 actions 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// ── Actions ────────────────────────────────────────────────────────

// List 分页列出 actions。
func (r Repository) List(ctx context.Context, q domain.ListActionsQuery) ([]*domain.Action, int64, error) {
	selectSQL, countSQL, args := buildListActionsSQL(q, false)

	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	pagedArgs := append(append([]any{}, args...), limit, offset)
	selectSQL += paginationSuffix

	rows, err := r.db.QueryContext(ctx, selectSQL, pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Action, 0, limit)
	for rows.Next() {
		var a domain.Action
		var triggerStr, statusStr string
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Name, &triggerStr, &statusStr,
			&a.CreatedAt, &a.UpdatedAt, &a.Revision); err != nil {
			return nil, 0, err
		}
		a.TriggerType = domain.TriggerType(triggerStr)
		a.Status = domain.ActionStatus(statusStr)
		items = append(items, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Get 按 id + orgID 取 action。
func (r Repository) Get(ctx context.Context, id, organizationID string) (*domain.Action, error) {
	var a domain.Action
	var triggerStr, statusStr string
	if err := r.db.QueryRowContext(ctx, queryGetAction, id, organizationID).Scan(
		&a.ID, &a.OrganizationID, &a.Name, &triggerStr, &statusStr,
		&a.CreatedAt, &a.UpdatedAt, &a.Revision,
	); err != nil {
		return nil, err
	}
	a.TriggerType = domain.TriggerType(triggerStr)
	a.Status = domain.ActionStatus(statusStr)
	return &a, nil
}

// Create 插入 action + 可选的初始 version。
func (r Repository) Create(ctx context.Context, action *domain.Action, version *domain.ActionVersion) error {
	_, err := r.db.ExecContext(ctx, queryInsertAction,
		action.ID, action.OrganizationID, action.Name, string(action.TriggerType), string(action.Status),
	)
	if err != nil {
		return err
	}
	if version != nil {
		_, err = r.db.ExecContext(ctx, queryInsertVersion,
			version.ID, version.ActionID, version.Version, version.Code,
		)
	}
	return err
}

// UpdateStatus 更新 action 的 status。
func (r Repository) UpdateStatus(ctx context.Context, cmd domain.UpdateStatusCommand) error {
	_, err := r.db.ExecContext(ctx, queryUpdateActionStatus,
		string(cmd.Status), cmd.ActionID, cmd.OrganizationID,
	)
	return err
}

// ─── Versions ──────────────────────────────────────────────────────

// ListVersions 分页列出某个 action 的 versions。
func (r Repository) ListVersions(ctx context.Context, q domain.ListActionVersionsQuery) ([]*domain.ActionVersion, int64, error) {
	countSQL := `SELECT COUNT(*) ` + queryListVersionsBase
	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, q.ActionID, q.OrganizationID).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	selectSQL := queryListVersionsSelect + queryListVersionsBase + " ORDER BY version DESC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, selectSQL, q.ActionID, q.OrganizationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.ActionVersion, 0, limit)
	for rows.Next() {
		var v domain.ActionVersion
		if err := rows.Scan(&v.ID, &v.ActionID, &v.Version, &v.Code, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// PublishVersion 发布新版本（= 插入一条 version 记录）。
func (r Repository) PublishVersion(ctx context.Context, actionID, organizationID string, version *domain.ActionVersion) error {
	_ = organizationID
	_, err := r.db.ExecContext(ctx, queryInsertVersion,
		version.ID, version.ActionID, version.Version, version.Code,
	)
	return err
}

// GetLatestVersion 取某个 action 的最新 version。
func (r Repository) GetLatestVersion(ctx context.Context, actionID, organizationID string) (*domain.ActionVersion, error) {
	_ = organizationID
	var v domain.ActionVersion
	if err := r.db.QueryRowContext(ctx, queryLatestVersion, actionID).Scan(
		&v.ID, &v.ActionID, &v.Version, &v.Code, &v.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &v, nil
}

// ─── Execution Logs ────────────────────────────────────────────────

// ListExecutionLogs 分页列出 execution logs（动态拼接 status filter）。
func (r Repository) ListExecutionLogs(ctx context.Context, q domain.ListExecutionLogsQuery) ([]*domain.ExecutionLog, int64, error) {
	selectSQL, countSQL, args := buildListExecLogsSQL(q, false)

	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	pagedArgs := append(append([]any{}, args...), limit, offset)
	selectSQL += paginationSuffix

	rows, err := r.db.QueryContext(ctx, selectSQL, pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.ExecutionLog, 0, limit)
	for rows.Next() {
		l, err := scanExecLogRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CreateExecutionLog 插入一条 execution log。
func (r Repository) CreateExecutionLog(ctx context.Context, log *domain.ExecutionLog) error {
	_, err := r.db.ExecContext(ctx, queryInsertExecLog,
		log.ID, log.ActionID, log.Version,
		string(log.Input),
	)
	return err
}

// UpdateExecutionLog 更新 execution log 结果（status/output/error）。
func (r Repository) UpdateExecutionLog(ctx context.Context, cmd domain.UpdateExecutionLogCommand) error {
	var outputJSON string
	if cmd.Output != nil {
		outputJSON = string(cmd.Output)
	}
	_, err := r.db.ExecContext(ctx, queryUpdateExecLog,
		string(cmd.Status), outputJSON, cmd.Error, cmd.LogID,
	)
	return err
}

// scanExecLogRow 扫描 execution_logs 行，处理 input/output JSON 和 executed_at  nullable 时间。
func scanExecLogRow(s scannable) (*domain.ExecutionLog, error) {
	var l domain.ExecutionLog
	var inputStr, outputStr sql.NullString
	var statusStr string
	var executedAt sql.NullTime
	var err *string

	if err := s.Scan(
		&l.ID, &l.ActionID, &l.Version, &statusStr,
		&inputStr, &outputStr, &err, &executedAt, &l.CreatedAt,
	); err != nil {
		return nil, err
	}
	l.Status = domain.ExecutionStatus(statusStr)

	if inputStr.Valid && inputStr.String != "" {
		l.Input = json.RawMessage(inputStr.String)
	}
	if outputStr.Valid && outputStr.String != "" {
		l.Output = json.RawMessage(outputStr.String)
	}
	l.Error = err
	if executedAt.Valid {
		t := executedAt.Time
		l.ExecutedAt = &t
	}
	return &l, nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}

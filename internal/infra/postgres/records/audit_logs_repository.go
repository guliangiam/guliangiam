package records

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// ListAuditLogs 按业务分类（登录/操作/错误）查询审计记录。
//
// 分类与结果由 action 约定推导，SQL 中不拼接客户端输入。
func (r Repository) ListAuditLogs(ctx context.Context, query domain.AuditLogQuery) ([]*domain.Record, int64, error) {
	limit, offset := platformpagination.ClampPageSize(query.PageSize), platformpagination.Offset(query.Page, query.PageSize)
	// 参数与 queryListAuditLogs 的 $1..$9 严格同序；显式传参供 check-sql-args 静态核对。
	rows, err := r.db.Query(ctx, queryListAuditLogs,
		query.OrganizationID,
		string(query.Category),
		string(query.Result),
		query.ApplicationID,
		query.StartTime,
		query.EndTime,
		query.Keyword,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Record, 0)
	for rows.Next() {
		var row recordRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.ActorID, &row.Action, &row.ResourceType,
			&row.ResourceID, &row.IP, &row.UserAgent, &row.Surface, &row.RequestID,
			&row.TraceID, &row.Metadata, &row.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		record := toRecord(row)
		items = append(items, &record)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	// 参数与 queryCountAuditLogs 的 $1..$7 严格同序（与列表过滤一致）。
	if err := r.db.QueryRow(ctx, queryCountAuditLogs,
		query.OrganizationID,
		string(query.Category),
		string(query.Result),
		query.ApplicationID,
		query.StartTime,
		query.EndTime,
		query.Keyword,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListLoginLogs 合并成功登录（sessions）与失败登录（records）。
func (r Repository) ListLoginLogs(ctx context.Context, query domain.LoginLogQuery) ([]*domain.LoginLogEntry, int64, error) {
	limit, offset := platformpagination.ClampPageSize(query.PageSize), platformpagination.Offset(query.Page, query.PageSize)
	// 参数与 queryListLoginLogs 的 $1..$8 严格同序；显式传参供 check-sql-args 静态核对。
	rows, err := r.db.Query(ctx, queryListLoginLogs,
		query.OrganizationID,
		query.ApplicationID,
		string(query.Result),
		query.Keyword,
		query.StartTime,
		query.EndTime,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.LoginLogEntry, 0)
	for rows.Next() {
		var item domain.LoginLogEntry
		var result string
		if err := rows.Scan(
			&item.ID, &result, &item.Username, &item.OrganizationID, &item.ApplicationID,
			&item.IP, &item.UserAgent, &item.Reason, &item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		item.Result = domain.AuditLogResult(result)
		items = append(items, &item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	// 参数与 queryCountLoginLogs 的 $1..$6 严格同序（与列表过滤一致）。
	if err := r.db.QueryRow(ctx, queryCountLoginLogs,
		query.OrganizationID,
		query.ApplicationID,
		string(query.Result),
		query.Keyword,
		query.StartTime,
		query.EndTime,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

var _ domain.AuditLogReader = Repository{}

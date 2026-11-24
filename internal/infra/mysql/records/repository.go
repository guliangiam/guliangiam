package records

import (
	"context"
	"strings"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	platformsorting "github.com/guliangiam/guliangiam/internal/platform/sorting"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/requestmeta"
)

// Repository 是 MySQL 8.0+ 实现的 records 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// recordSortSpec 是 GET /api/v1/audit/records 和 POST /api/v1/audit/records:search 的排序白名单。
var recordSortSpec = platformsorting.NewSpec(map[string]string{
	"createdAt":    "created_at",
	"action":       "action",
	"resourceType": "resource_type",
	"surface":      "surface",
})

// scanRecordRow 扫描单条 records 行（List / Search 共用）。
// 列顺序必须与 sql.go queryListRecords / querySearchRecords 的 SELECT 严格一致（13 列）。
func scanRecordRow(s interface{ Scan(dest ...any) error }) (recordRow, error) {
	var row recordRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.ActorID, &row.Action, &row.ResourceType,
		&row.ResourceID, &row.IP, &row.UserAgent, &row.Surface, &row.RequestID,
		&row.TraceID, &row.Metadata, &row.CreatedAt,
	)
	return row, err
}

// Create 插入审计记录。
// MySQL 版简化处理：不计算 hash chain linkage，chain 相关字段传零值。
// 参数：17 个 ?（metadata 传 JSON bytes）
func (r Repository) Create(ctx context.Context, record *domain.Record) error {
	// 列宽防御：与 PostgreSQL 方言一致，user_agent / request_id / trace_id 均为
	// VARCHAR(255)，超长会让 INSERT 失败并回滚调用方的业务事务。
	record.UserAgent = requestmeta.TruncateRunes(record.UserAgent, requestmeta.MaxUserAgentLen)
	record.RequestID = requestmeta.TruncateRunes(record.RequestID, requestmeta.MaxRequestIDLen)
	record.TraceID = requestmeta.TruncateRunes(record.TraceID, requestmeta.MaxTraceIDLen)

	metadataJSON, err := mysqlshared.MarshalJSON(record.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryCreateRecord,
		record.ID, record.OrganizationID, record.ActorID, record.Action, record.ResourceType,
		record.ResourceID, record.IP, record.UserAgent, domain.NormalizeSurface(record.Surface),
		record.RequestID, record.TraceID, metadataJSON, record.CreatedAt,
		"", int64(0), nil, nil, // chain_id, chain_seq, prev_hash, entry_hash（暂时空值）
	)
	return err
}

// keywordArgs 返回 n 个 keyword 值的 slice。
// MySQL 版 recordsKeyword 包含 6 个 ?（1 个 CASE WHEN + 5 个 LIKE CONCAT，每个 CONCAT 只有 1 个 ?）。
func keywordArgs(k string) []interface{} {
	args := make([]interface{}, 6)
	for i := range args {
		args[i] = k
	}
	return args
}

// List 分页列出审计记录。
// MySQL 版参数绑定（占位符重复）：
//
//	recordsFilter: org_id×2(2), actor_id×2(2), action×2(2), resource_type×2(2) = 8
//	recordsKeyword: keyword×6
//	limit, offset = 2
//	总计：16 个参数；Count 去掉 limit/offset = 14
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Record, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	orderBy, err := recordSortSpec.Resolve(q.SortField, q.SortOrder, "created_at")
	if err != nil {
		return nil, 0, err
	}

	listSQL := strings.Replace(queryListRecords, "{ORDER_BY}", orderBy, 1)
	args := make([]interface{}, 0, 16)
	args = append(args,
		q.OrganizationID, q.OrganizationID,
		q.ActorID, q.ActorID,
		q.Action, q.Action,
		q.ResourceType, q.ResourceType,
	)
	args = append(args, keywordArgs(q.Keyword)...)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Record, 0)
	for rows.Next() {
		row, err := scanRecordRow(rows)
		if err != nil {
			return nil, 0, err
		}
		record, err := toRecord(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, record)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count 参数：去掉 limit/offset
	countArgs := make([]interface{}, 0, 14)
	countArgs = append(countArgs,
		q.OrganizationID, q.OrganizationID,
		q.ActorID, q.ActorID,
		q.Action, q.Action,
		q.ResourceType, q.ResourceType,
	)
	countArgs = append(countArgs, keywordArgs(q.Keyword)...)

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountRecords, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Search 高级搜索审计记录。
// MySQL 版参数绑定（占位符重复）：
//
//	recordsFilter: org_id×2(2), actor_id×2(2), action×2(2), resource_type×2(2) = 8
//	resource_id: ×2
//	recordsTimeRange: start_time×2(2), end_time×2(2) = 4
//	recordsKeyword: keyword×6
//	limit, offset = 2
//	总计：22 个参数；Count 去掉 limit/offset = 20
func (r Repository) Search(ctx context.Context, c domain.SearchCriteria) ([]*domain.Record, int64, error) {
	offset := mysqlshared.PageOffset(c.Page, c.PageSize)
	limit := mysqlshared.ClampPageSize(c.PageSize)

	orderBy, err := recordSortSpec.Resolve(c.SortField, c.SortOrder, "created_at")
	if err != nil {
		return nil, 0, err
	}

	searchSQL := strings.Replace(querySearchRecords, "{ORDER_BY}", orderBy, 1)
	args := make([]interface{}, 0, 20)
	args = append(args,
		c.OrganizationID, c.OrganizationID,
		c.ActorID, c.ActorID,
		c.Action, c.Action,
		c.ResourceType, c.ResourceType,
		c.ResourceID, c.ResourceID,
		c.StartTime, c.StartTime,
		c.EndTime, c.EndTime,
	)
	args = append(args, keywordArgs(c.Keyword)...)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, searchSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Record, 0)
	for rows.Next() {
		row, err := scanRecordRow(rows)
		if err != nil {
			return nil, 0, err
		}
		record, err := toRecord(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, record)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count 参数：去掉 limit/offset
	countArgs := make([]interface{}, 0, 18)
	countArgs = append(countArgs,
		c.OrganizationID, c.OrganizationID,
		c.ActorID, c.ActorID,
		c.Action, c.Action,
		c.ResourceType, c.ResourceType,
		c.ResourceID, c.ResourceID,
		c.StartTime, c.StartTime,
		c.EndTime, c.EndTime,
	)
	countArgs = append(countArgs, keywordArgs(c.Keyword)...)

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountSearchRecords, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ── 哈希链维护 — mysql 后端不支持审计链（Postgres 专属 hash-chain） ──

func (r Repository) LoadLastChainEntry(ctx context.Context, chainID string) (seqOut int64, hashOut []byte, err error) {
	return 0, nil, apperrors.NewUnimplemented("records.LoadLastChainEntry 在 MySQL 驱动上尚未实现（Postgres hash-chain）")
}

func (r Repository) LoadAnchorSeq(ctx context.Context, organizationID, chainID string) (int64, error) {
	return 0, apperrors.NewUnimplemented("records.LoadAnchorSeq 在 MySQL 驱动上尚未实现（Postgres hash-chain）")
}

func (r Repository) InsertAnchor(ctx context.Context, anchor *domain.ChainAnchor) error {
	return apperrors.NewUnimplemented("records.InsertAnchor 在 MySQL 驱动上尚未实现（Postgres hash-chain）")
}

func (r Repository) ListChainEntries(ctx context.Context, chainID string) ([]domain.ChainEntry, error) {
	return nil, apperrors.NewUnimplemented("records.ListChainEntries 在 MySQL 驱动上尚未实现（Postgres hash-chain）")
}

// 编译期接口断言
var _ domain.Repository = Repository{}

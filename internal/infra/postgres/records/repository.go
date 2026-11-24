package records

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/platform/requestmeta"
	platformsorting "github.com/guliangiam/guliangiam/internal/platform/sorting"
)

type Repository struct {
	db          shared.DBTX
	chainWriter *audit.AuditChainWriter
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{
		db:          db,
		chainWriter: audit.NewAuditChainWriter(db, DefaultChainSQL()),
	}
}

func (r Repository) Create(ctx context.Context, record *domain.Record) error {
	// 列宽防御：user_agent / request_id / trace_id 都是 VARCHAR(255)，超长会让
	// INSERT 失败并把调用方的业务事务一起回滚。截断必须发生在算哈希链之前，
	// 否则链上哈希与落库值不一致。
	record.UserAgent = requestmeta.TruncateRunes(record.UserAgent, requestmeta.MaxUserAgentLen)
	record.RequestID = requestmeta.TruncateRunes(record.RequestID, requestmeta.MaxRequestIDLen)
	record.TraceID = requestmeta.TruncateRunes(record.TraceID, requestmeta.MaxTraceIDLen)

	// Compute hash-chain linkage within the same transaction so chain_seq
	// assignment is atomic and gap-free (audit tamper-evidence).
	chain, err := r.chainWriter.Compute(ctx,
		record.OrganizationID, record.ActorID, record.Action, record.ResourceType, record.ResourceID,
		record.IP, record.UserAgent, domain.NormalizeSurface(record.Surface),
		record.RequestID, record.TraceID, record.Metadata, record.CreatedAt,
	)
	if err != nil {
		return err
	}

	// 空 IP 传 NULL（inet 列不接受空字符串）。
	var ipParam any
	if record.IP != "" {
		ipParam = record.IP
	}

	// 空 ActorID 传 NULL（uuid nullable 列不接受空字符串）。
	var actorIDParam any
	if record.ActorID != "" {
		actorIDParam = record.ActorID
	}

	// nil Metadata 传空 map（jsonb NOT NULL 列有 default '{}'，但显式传参覆盖不了）。
	meta := record.Metadata
	if meta == nil {
		meta = map[string]any{}
	}

	_, err = r.db.Exec(ctx, queryCreateRecord,
		record.ID, record.OrganizationID, actorIDParam, record.Action, record.ResourceType,
		record.ResourceID, ipParam, record.UserAgent, domain.NormalizeSurface(record.Surface),
		record.RequestID, record.TraceID, meta, record.CreatedAt,
		chain.ChainID, chain.ChainSeq, chain.PrevHash, chain.EntryHash, chain.ChainAlgo,
	)
	return err
}

// recordSortSpec is the whitelist for GET /api/v1/audit/records and
// POST /api/v1/audit/records:search (手册 4.8.1).
//
// Keys are the API names a client sends; values are the physical columns. The
// audit trail is append-only and time-ordered by default, so `createdAt` is the
// module default as well as the common case.
var recordSortSpec = platformsorting.NewSpec(map[string]string{
	"createdAt":    "created_at",
	"action":       "action",
	"resourceType": "resource_type",
	"surface":      "surface",
})

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Record, int64, error) {
	limit, offset := platformpagination.ClampPageSize(query.PageSize), platformpagination.Offset(query.Page, query.PageSize)
	orderBy, err := recordSortSpec.Resolve(query.SortField, query.SortOrder, "created_at")
	if err != nil {
		return nil, 0, err
	}
	// The clause is substituted, the values are not: every part of orderBy comes
	// from the whitelist and a closed direction set, so nothing the client sent
	// reaches the statement text. A token replace rather than Sprintf, because
	// these statements carry literal `%` (ILIKE wildcards).
	listSQL := strings.Replace(queryListRecords, "{ORDER_BY}", orderBy, 1)
	rows, err := r.db.Query(ctx, listSQL, query.OrganizationID, query.ActorID, query.Action, query.ResourceType, query.RequestID, query.Keyword, limit, offset)
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
	if err := r.db.QueryRow(ctx, queryCountRecords, query.OrganizationID, query.ActorID, query.Action, query.ResourceType, query.RequestID, query.Keyword).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Search(ctx context.Context, criteria domain.SearchCriteria) ([]*domain.Record, int64, error) {
	limit, offset := platformpagination.ClampPageSize(criteria.PageSize), platformpagination.Offset(criteria.Page, criteria.PageSize)
	orderBy, err := recordSortSpec.Resolve(criteria.SortField, criteria.SortOrder, "created_at")
	if err != nil {
		return nil, 0, err
	}
	searchSQL := strings.Replace(querySearchRecords, "{ORDER_BY}", orderBy, 1)
	rows, err := r.db.Query(ctx, searchSQL,
		criteria.OrganizationID, criteria.ActorID, criteria.Action, criteria.ResourceType,
		criteria.ResourceID, criteria.StartTime, criteria.EndTime, criteria.RequestID, criteria.Keyword,
		limit, offset,
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
	if err := r.db.QueryRow(ctx, queryCountSearchRecords,
		criteria.OrganizationID, criteria.ActorID, criteria.Action, criteria.ResourceType,
		criteria.ResourceID, criteria.StartTime, criteria.EndTime, criteria.RequestID, criteria.Keyword,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ── 哈希链维护（AuditChainWriter 内部调用） ──────────────────────────

func (r Repository) LoadLastChainEntry(ctx context.Context, chainID string) (seqOut int64, hashOut []byte, err error) {
	if _, err := r.db.Exec(ctx, queryAdvisoryLock, chainID); err != nil {
		return 0, nil, apperrors.WrapInternal(err, "acquire chain advisory lock")
	}
	var seq int64
	var hash []byte
	err = r.db.QueryRow(ctx, queryLoadLastChainEntry, chainID).Scan(&seq, &hash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	return seq, hash, nil
}

func (r Repository) LoadAnchorSeq(ctx context.Context, organizationID, chainID string) (int64, error) {
	var seq int64
	err := r.db.QueryRow(ctx, queryLoadAnchorSeq, organizationID, chainID).Scan(&seq)
	return seq, err
}

func (r Repository) InsertAnchor(ctx context.Context, anchor *domain.ChainAnchor) error {
	_, err := r.db.Exec(ctx, queryInsertAnchor,
		uuid.NewString(), anchor.OrganizationID, anchor.ChainID, anchor.AnchorSeq, anchor.EntryHash, anchor.Proof,
	)
	return err
}

func (r Repository) ListChainEntries(ctx context.Context, chainID string) ([]domain.ChainEntry, error) {
	rows, err := r.db.Query(ctx, queryListChainEntries, chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.ChainEntry
	for rows.Next() {
		var e domain.ChainEntry
		if err := rows.Scan(&e.ChainSeq, &e.PrevHash, &e.EntryHash, &e.ChainAlgo); err != nil {
			return nil, err
		}
		e.ChainID = chainID
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

var _ domain.Repository = Repository{}

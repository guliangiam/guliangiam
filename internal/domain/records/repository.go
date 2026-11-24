package records

import (
	"context"
	"time"
)

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	RequestID      string
	Keyword        string
	SortField      string
	SortOrder      string
}

type SearchCriteria struct {
	Page           int
	PageSize       int
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     string
	RequestID      string
	Keyword        string
	SortField      string
	SortOrder      string
	StartTime      *time.Time
	EndTime        *time.Time
}

// Repository 审计日志 CRUD + 哈希链维护。
//
// 所有方法通过 shared.DBTX 执行 — 调用方负责事务边界。
// AuditChainWriter.ComputeAndPersist 必须在 INSERT 同一个 records 的事务内调用，
// 以保证 chain_seq 原子无间隙。
type Repository interface {
	// ── CRUD（handler + service 层） ────────────────────────────────

	Create(ctx context.Context, record *Record) error
	List(ctx context.Context, query ListQuery) ([]*Record, int64, error)
	Search(ctx context.Context, criteria SearchCriteria) ([]*Record, int64, error)

	// ── 哈希链维护（AuditChainWriter 内部调用） ──────────────────────

	// LoadLastChainEntry 返回指定 chain 的最新 (chain_seq, entry_hash)。
	// 空链返回 (0, nil, nil)。
	LoadLastChainEntry(ctx context.Context, chainID string) (int64, []byte, error)

	// LoadAnchorSeq 返回指定 chain 当前最大 anchor_seq（0 表示无锚点）。
	LoadAnchorSeq(ctx context.Context, organizationID, chainID string) (int64, error)

	// InsertAnchor 写入一个新锚点（anchor_seq 由调用方计算）。
	InsertAnchor(ctx context.Context, anchor *ChainAnchor) error

	// ListChainEntries 按 chain_seq ASC 拉指定 chain 的全部条目（用于 VerifyChain）。
	ListChainEntries(ctx context.Context, chainID string) ([]ChainEntry, error)
}

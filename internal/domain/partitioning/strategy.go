package partitioning

import (
	"context"
	"time"
)

// Granularity 分区粒度。
type Granularity string

const (
	GranularityDay   Granularity = "day"
	GranularityWeek  Granularity = "week"
	GranularityMonth Granularity = "month"
)

// TableStrategy 声明单张表的分区策略（仅当真实数据量证据满足 ADR-011 出口条件后启用）。
type TableStrategy struct {
	Table       string
	Column      string // 时间列，用作分区键（通常是 created_at）
	Granularity Granularity
	// 分区策略启用条件（ADR-011 出口条件）——未满足时 PartitioningStrategy 应返回 IsEnabled=false。
	MinSizeBytes  int64     // pg_total_relation_size 阈值（默认 50 GB）
	MinIndexBytes int64     // pg_indexes_size 阈值（默认 20 GB）
	Evidence      []string  // 已收集的 EXPLAIN (ANALYZE, BUFFERS) 对比证据引用
	Enabled       bool      // 是否已启用声明式分区
	EnabledAt     time.Time // 启用时间（零值表示未启用）
}

// RelationSize is the physical footprint of one table (total relation size and
// its indexes) as reported by PostgreSQL. Values are bytes.
type RelationSize struct {
	TotalBytes int64
	IndexBytes int64
}

// RelationSizer probes the physical size of a table. The PostgreSQL adapter
// uses pg_total_relation_size/pg_indexes_size; tests can inject a fake.
type RelationSizer interface {
	Size(ctx context.Context, table string) (RelationSize, error)
}

// PartitioningStrategy 是时间分区的 domain port。
// schema YAML 中不声明 partition_by（gen_schema.py 不支持 PARTITION BY DDL），
// 分区策略由 Go 代码层 registry 声明 + 运维探测函数驱动。
type PartitioningStrategy interface {
	// Strategies 返回所有已声明的分区策略。
	Strategies() []TableStrategy
	// StrategyFor 返回指定表的策略（未声明时返回 nil, false）。
	StrategyFor(table string) (*TableStrategy, bool)
	// ShouldPartition 返回指定表是否应启用分区（当前状态 + 证据评估）。
	ShouldPartition(table string) bool
}

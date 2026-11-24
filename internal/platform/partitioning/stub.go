package partitioning

import (
	domainpartitioning "github.com/guliangiam/guliangiam/internal/domain/partitioning"
)

// StubStrategy 是 domainpartitioning.PartitioningStrategy 的合约层存根——声明高增长表的分区策略模板，
// 但 Enabled=false（ADR-011 要求"先度量后分区"，当前无真实数据量证据）。
// 真实 adapter 将查 pg_total_relation_size / pg_indexes_size 并在满足阈值时启用。
type StubStrategy struct{}

func NewStubStrategy() *StubStrategy { return &StubStrategy{} }

// columnCreatedAt 是分区表的创建时间列。
const columnCreatedAt = "created_at"

// defaultStrategies 是首批候选高增长表的分区策略模板。
// 表名必须存在于 contracts/schema/*.yaml（stub_test.go 以契约层为事实源拦截幽灵表名）。
// llm_metering_events 使用 recorded_at 作为时间列（不是 created_at），已单列为候选；
// access_tokens/sessions 属低行数、高更新表，暂不入列（若容量指标证明否则再评审）。
// 真实容量探测与裁决见 platform/partitioning/advisor.go（pg_total_relation_size 等）。
// 全部 Enabled=false——需达到 ADR-011 出口条件（容量阈值 + EXPLAIN 证据 + 迁移锁评估 + 归档演练）后才启用。
func defaultStrategies() []domainpartitioning.TableStrategy {
	return []domainpartitioning.TableStrategy{
		{
			Table:         "records",
			Column:        columnCreatedAt,
			Granularity:   domainpartitioning.GranularityMonth,
			MinSizeBytes:  50 << 30, // 50 GB
			MinIndexBytes: 20 << 30, // 20 GB
			Enabled:       false,
		},
		{
			Table:         "entries",
			Column:        columnCreatedAt,
			Granularity:   domainpartitioning.GranularityMonth,
			MinSizeBytes:  50 << 30,
			MinIndexBytes: 20 << 30,
			Enabled:       false,
		},
		{
			Table:         "llm_usage_logs",
			Column:        columnCreatedAt,
			Granularity:   domainpartitioning.GranularityMonth,
			MinSizeBytes:  50 << 30,
			MinIndexBytes: 20 << 30,
			Enabled:       false,
		},
		{
			Table:         "webhook_events",
			Column:        columnCreatedAt,
			Granularity:   domainpartitioning.GranularityDay,
			MinSizeBytes:  50 << 30,
			MinIndexBytes: 20 << 30,
			Enabled:       false,
		},
		{
			Table:         "metering_events",
			Column:        columnCreatedAt,
			Granularity:   domainpartitioning.GranularityDay,
			MinSizeBytes:  50 << 30,
			MinIndexBytes: 20 << 30,
			Enabled:       false,
		},
		{
			Table:         "llm_metering_events",
			Column:        "recorded_at",
			Granularity:   domainpartitioning.GranularityDay,
			MinSizeBytes:  50 << 30,
			MinIndexBytes: 20 << 30,
			Enabled:       false,
		},
	}
}

func (s *StubStrategy) Strategies() []domainpartitioning.TableStrategy {
	return defaultStrategies()
}

func (s *StubStrategy) StrategyFor(table string) (*domainpartitioning.TableStrategy, bool) {
	for i := range defaultStrategies() {
		if defaultStrategies()[i].Table == table {
			t := defaultStrategies()[i]
			return &t, true
		}
	}
	return nil, false
}

func (s *StubStrategy) ShouldPartition(table string) bool {
	t, ok := s.StrategyFor(table)
	if !ok {
		return false
	}
	return t.Enabled && !t.EnabledAt.IsZero()
}

package partitioning

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	domainpartitioning "github.com/guliangiam/guliangiam/internal/domain/partitioning"
	"github.com/stretchr/testify/assert"
)

// schemaTableName 匹配 contracts/schema/*.yaml 的顶层表声明，忽略列名同名项。
var schemaTableName = regexp.MustCompile(`(?m)^table_name:\s+guliangiam\.([a-z0-9_]+)\s*$`)

func TestDefaultStrategies_AllDisabled(t *testing.T) {
	s := NewStubStrategy()
	strategies := s.Strategies()
	assert.Len(t, strategies, 6)
	for _, st := range strategies {
		if st.Table == "llm_metering_events" {
			assert.Equal(t, "recorded_at", st.Column)
		} else {
			assert.Equal(t, columnCreatedAt, st.Column)
		}
		assert.False(t, st.Enabled)
	}
}

func TestStrategyFor_KnownAndUnknown(t *testing.T) {
	s := NewStubStrategy()
	st, ok := s.StrategyFor("records")
	assert.True(t, ok)
	assert.Equal(t, domainpartitioning.GranularityMonth, st.Granularity)

	st, ok = s.StrategyFor("entries")
	assert.True(t, ok)
	assert.Equal(t, domainpartitioning.GranularityMonth, st.Granularity)

	st, ok = s.StrategyFor("metering_events")
	assert.True(t, ok)
	assert.Equal(t, domainpartitioning.GranularityDay, st.Granularity)

	_, ok = s.StrategyFor("metering_logs")
	assert.False(t, ok, "metering_logs 是生命周期策略组键，不是物理表")

	_, ok = s.StrategyFor("not_a_table")
	assert.False(t, ok)
}

func TestShouldPartition_FalseWhileDisabled(t *testing.T) {
	s := NewStubStrategy()
	assert.False(t, s.ShouldPartition("records"))
	assert.False(t, s.ShouldPartition("unknown"))
}

// TestDefaultStrategies_TablesExistInSchema 防止幽灵表名回潮：策略里的每张表都必须能
// 在 contracts/schema/*.yaml 中找到 table_name 声明，契约层是表名唯一事实源。
func TestDefaultStrategies_TablesExistInSchema(t *testing.T) {
	tables := loadSchemaTables(t, filepath.Join(repoRoot(t), "contracts", "schema"))
	if len(tables) == 0 {
		t.Fatal("contracts/schema/*.yaml 未解析到任何 table_name 声明")
	}
	for _, st := range NewStubStrategy().Strategies() {
		assert.Truef(t, tables[st.Table], "分区策略表 %q 不在 contracts/schema/*.yaml 中（幽灵表名）", st.Table)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("未找到 go.mod，无法定位仓库根")
		}
		dir = parent
	}
}

func loadSchemaTables(t *testing.T, dir string) map[string]bool {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatalf("glob schema: %v", err)
	}
	tables := make(map[string]bool)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, match := range schemaTableName.FindAllStringSubmatch(string(data), -1) {
			tables[match[1]] = true
		}
	}
	return tables
}

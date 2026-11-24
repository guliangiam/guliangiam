package partitioning

import (
	"context"
	"errors"
	"testing"
	"time"

	domainpartitioning "github.com/guliangiam/guliangiam/internal/domain/partitioning"
	"github.com/stretchr/testify/require"
)

type fakeStrategy struct {
	strategies []domainpartitioning.TableStrategy
}

func (f fakeStrategy) Strategies() []domainpartitioning.TableStrategy { return f.strategies }
func (f fakeStrategy) StrategyFor(table string) (*domainpartitioning.TableStrategy, bool) {
	for i := range f.strategies {
		if f.strategies[i].Table == table {
			matched := f.strategies[i]
			return &matched, true
		}
	}
	return nil, false
}
func (f fakeStrategy) ShouldPartition(string) bool { return false }

type fakeSizer struct {
	sizes map[string]domainpartitioning.RelationSize
	err   error
}

func (f fakeSizer) Size(_ context.Context, table string) (domainpartitioning.RelationSize, error) {
	if f.err != nil {
		return domainpartitioning.RelationSize{}, f.err
	}
	return f.sizes[table], nil
}

func enabledStrategy(table string) domainpartitioning.TableStrategy {
	return domainpartitioning.TableStrategy{
		Table:         table,
		Column:        "created_at",
		Granularity:   domainpartitioning.GranularityMonth,
		MinSizeBytes:  100,
		MinIndexBytes: 50,
		Evidence:      []string{"explain: benefits"},
		Enabled:       true,
		EnabledAt:     time.Now(),
	}
}

func TestAdvisorEvaluate_MarksExitConditionsMet(t *testing.T) {
	strategy := enabledStrategy("records")
	advisor := NewAdvisor(fakeStrategy{strategies: []domainpartitioning.TableStrategy{strategy}}, fakeSizer{
		sizes: map[string]domainpartitioning.RelationSize{"records": {TotalBytes: 100, IndexBytes: 50}},
	}, nil)

	decisions := advisor.Evaluate(context.Background())
	require.Len(t, decisions, 1)
	require.True(t, decisions[0].ShouldPartition)
	require.Equal(t, "adr011_exit_conditions_met", decisions[0].Reason)

	should, err := advisor.ShouldPartition(context.Background(), "records")
	require.NoError(t, err)
	require.True(t, should)
}

func TestAdvisorEvaluate_DisabledAndBelowThreshold(t *testing.T) {
	disabled := enabledStrategy("records")
	disabled.Enabled = false
	below := enabledStrategy("entries")
	advisor := NewAdvisor(fakeStrategy{strategies: []domainpartitioning.TableStrategy{disabled, below}}, fakeSizer{
		sizes: map[string]domainpartitioning.RelationSize{
			"records": {TotalBytes: 1 << 40, IndexBytes: 1 << 39},
			"entries": {TotalBytes: 10, IndexBytes: 1},
		},
	}, nil)

	decisions := advisor.Evaluate(context.Background())
	require.Len(t, decisions, 2)
	require.False(t, decisions[0].ShouldPartition)
	require.Equal(t, "disabled", decisions[0].Reason)
	require.False(t, decisions[1].ShouldPartition)
	require.Equal(t, "below_total_size_threshold", decisions[1].Reason)

	should, err := advisor.ShouldPartition(context.Background(), "unknown")
	require.NoError(t, err)
	require.False(t, should)
}

func TestAdvisorEvaluate_ProbeFailureDoesNotAbort(t *testing.T) {
	advisor := NewAdvisor(fakeStrategy{strategies: []domainpartitioning.TableStrategy{enabledStrategy("records")}},
		fakeSizer{err: errors.New("db down")}, nil)
	decisions := advisor.Evaluate(context.Background())
	require.Len(t, decisions, 1)
	require.False(t, decisions[0].ShouldPartition)
	require.Equal(t, "size_probe_failed", decisions[0].Reason)
}

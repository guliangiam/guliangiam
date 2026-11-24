package audit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	stubListChainsSQL   = "stub list active chains"
	stubCountPendingSQL = "stub count pending"
)

type chainIDRows struct {
	chains []string
	idx    int
}

func (r *chainIDRows) Close()                                       {}
func (r *chainIDRows) Err() error                                   { return nil }
func (r *chainIDRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *chainIDRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *chainIDRows) TypeMap() *pgtype.Map                         { return nil }
func (r *chainIDRows) Conn() *pgx.Conn                              { return nil }
func (r *chainIDRows) RawValues() [][]byte                          { return nil }

func (r *chainIDRows) Next() bool {
	if r.idx >= len(r.chains) {
		return false
	}
	r.idx++
	return true
}

func (r *chainIDRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.chains) {
		return errors.New("no current row")
	}
	*(dest[0].(*string)) = r.chains[r.idx-1]
	return nil
}

func (r *chainIDRows) Values() ([]any, error) {
	if r.idx == 0 || r.idx > len(r.chains) {
		return nil, errors.New("no current row")
	}
	return []any{r.chains[r.idx-1]}, nil
}

type chainMetricsStubDB struct {
	chains       []string
	pending      int64
	pendingErr   error
	pendingCalls int
}

func (f *chainMetricsStubDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (f *chainMetricsStubDB) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if sql != stubListChainsSQL {
		return nil, errors.New("unexpected Query: " + sql)
	}
	return &chainIDRows{chains: f.chains}, nil
}

func (f *chainMetricsStubDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch sql {
	case stubCountPendingSQL:
		f.pendingCalls++
		if f.pendingErr != nil {
			return scanFuncRow{fn: func(...any) error { return f.pendingErr }}
		}
		return scanFuncRow{fn: func(dest ...any) error {
			*(dest[0].(*int64)) = f.pending
			return nil
		}}
	case stubLoadLastEntrySQL:
		// An empty chain makes Anchor fail before any anchor row is written,
		// so the test observes result=error without touching real SQL.
		return scanFuncRow{fn: func(dest ...any) error {
			*(dest[0].(*int64)) = 0
			*(dest[1].(*[]byte)) = nil
			*(dest[2].(*string)) = ""
			return nil
		}}
	default:
		return scanFuncRow{fn: func(...any) error { return errors.New("unexpected QueryRow: " + sql) }}
	}
}

func newChainMetricsWorker(db *chainMetricsStubDB) *ChainWorker {
	writer := NewAuditChainWriter(db, ChainSQL{
		AdvisoryLock:     stubAdvisoryLockSQL,
		LoadLastEntry:    stubLoadLastEntrySQL,
		ListActiveChains: stubListChainsSQL,
		CountPending:     stubCountPendingSQL,
	})
	return NewChainWorker(writer, time.Minute, time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func auditChainGaugeValue(t *testing.T) float64 {
	t.Helper()
	metric := &dto.Metric{}
	require.NoError(t, auditChainPendingEntries.Write(metric))
	return metric.GetGauge().GetValue()
}

func auditChainBuildSamples(t *testing.T, result string) uint64 {
	t.Helper()
	reg := prometheus.NewRegistry()
	require.NoError(t, registerAuditChainMetrics(reg))
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != "guliangiam_audit_chain_build_duration_seconds" {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "result" && label.GetValue() == result {
					return metric.GetHistogram().GetSampleCount()
				}
			}
		}
	}
	return 0
}

func TestRegisterAuditChainMetrics_IsIdempotent(t *testing.T) {
	reg := prometheus.NewRegistry()
	require.NoError(t, registerAuditChainMetrics(reg))
	require.NoError(t, registerAuditChainMetrics(reg), "registering the same collectors twice must not panic")

	families, err := reg.Gather()
	require.NoError(t, err)
	var names []string
	for _, family := range families {
		names = append(names, family.GetName())
	}
	assert.Contains(t, names, "guliangiam_audit_chain_pending_entries")
}

func TestAnchorAll_RefreshesPendingGaugeAndObservesBuildDuration(t *testing.T) {
	db := &chainMetricsStubDB{chains: []string{"org-a", "org-b"}, pending: 5}
	worker := newChainMetricsWorker(db)

	before := auditChainBuildSamples(t, metricResultError)
	worker.anchorAll(context.Background())

	assert.Equal(t, 1, db.pendingCalls, "every anchor cycle must refresh the pending gauge")
	assert.Equal(t, 5.0, auditChainGaugeValue(t))
	assert.Equal(t, before+uint64(len(db.chains)), auditChainBuildSamples(t, metricResultError),
		"every failed Anchor call must be timed as result=error")
}

func TestRefreshPendingGauge_QueryFailureKeepsPreviousValue(t *testing.T) {
	db := &chainMetricsStubDB{pending: 5}
	worker := newChainMetricsWorker(db)

	worker.refreshPendingGauge(context.Background())
	assert.Equal(t, 5.0, auditChainGaugeValue(t))

	db.pending = 0
	db.pendingErr = errors.New("count query failed")
	worker.refreshPendingGauge(context.Background())
	assert.Equal(t, 5.0, auditChainGaugeValue(t), "a failed count must keep the last good value, not publish zero")

	db.pendingErr = nil
	worker.refreshPendingGauge(context.Background())
	assert.Equal(t, 0.0, auditChainGaugeValue(t), "a successful empty count must publish zero")
}

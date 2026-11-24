package casbin

import (
	"context"
	"testing"
	"time"

	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestField(t *testing.T) {
	tests := []struct {
		name string
		rule []string
		idx  int
		want string
	}{
		{"index in range", []string{"a", "b", "c"}, 0, "a"},
		{"index in range mid", []string{"a", "b", "c"}, 1, "b"},
		{"index at end", []string{"a", "b", "c"}, 2, "c"},
		{"index out of range", []string{"a", "b"}, 3, ""},
		{"empty rule", []string{}, 0, ""},
		{"negative concept - large idx", []string{"x"}, 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := field(tt.rule, tt.idx)
			if got != tt.want {
				t.Errorf("field() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want []string
	}{
		{"all non-empty", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"with empty strings", []string{"a", "", "c", ""}, []string{"a", "c"}},
		{"all empty", []string{"", "", ""}, []string{}},
		{"single non-empty", []string{"x"}, []string{"x"}},
		{"single empty", []string{""}, []string{}},
		{"nil input", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nonEmpty(tt.vals...)
			if len(got) != len(tt.want) {
				t.Errorf("nonEmpty() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("nonEmpty()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAdapter_WithEnforcerID(t *testing.T) {
	a := Adapter{enforcerID: "original"}
	b := a.WithEnforcerID("new")
	if b.enforcerID != "new" {
		t.Errorf("WithEnforcerID() enforcerID = %q, want %q", b.enforcerID, "new")
	}
	if a.enforcerID != "original" {
		t.Errorf("original adapter should not be modified, got %q", a.enforcerID)
	}
}

func TestNewAdapter(t *testing.T) {
	a := NewAdapter(nil)
	if a.pool != nil {
		t.Error("expected nil pool")
	}
	if a.enforcerID != "" {
		t.Errorf("expected empty enforcerID, got %q", a.enforcerID)
	}
}

type stubQueryRower struct {
	allowed           bool
	requiredLifecycle string
	sql               string
	args              []any
}

func (s *stubQueryRower) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	s.sql = sql
	s.args = args
	allowed := s.allowed
	if s.requiredLifecycle != "" && (len(args) != 7 || args[6] != s.requiredLifecycle) {
		allowed = false
	}
	return shared.NewMockRow([]any{allowed}, nil)
}

type stubPolicyQuerier struct {
	rows pgx.Rows
	sql  string
	args []any
}

func (s *stubPolicyQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.sql = sql
	s.args = args
	return s.rows, nil
}

func TestLoadPolicy_PermanentPolicyWithNullMetadataIsLoaded(t *testing.T) {
	db := &stubPolicyQuerier{rows: shared.NewMockRows([][]any{{
		"p", "alice", "acme", "orders", "get", "", nil,
	}})}
	m, err := casbinmodel.NewModelFromString(`[request_definition]
r = sub, dom, obj, act
[policy_definition]
p = sub, dom, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && r.dom == p.dom && r.obj == p.obj && r.act == p.act`)
	require.NoError(t, err)

	require.NoError(t, loadPolicy(context.Background(), db, "enforcer-1", m))
	require.Equal(t, qLoadPolicy, db.sql)
	require.Equal(t, []any{"enforcer-1"}, db.args)
	require.Contains(t, db.sql, "v5 IS NULL OR v5 NOT LIKE")
	policies, err := m.GetPolicy("p", "p")
	require.NoError(t, err)
	require.Equal(t, [][]string{{"alice", "acme", "orders", "get"}}, policies)
}

func TestEnforceJIT_BindsLifecycleAndHardTimeWindow(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	db := &stubQueryRower{allowed: true}
	allowed, err := enforceJIT(context.Background(), db, "enforcer-1", "lifecycle-a", now,
		"alice", "acme", "orders", "get")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, qEnforceJITPolicy, db.sql)
	require.Equal(t, []any{"enforcer-1", "alice", "acme", "orders", "get", now, "lifecycle-a"}, db.args)
	require.Contains(t, db.sql, "ptype = 'p'")
	require.Contains(t, db.sql, "v4::timestamptz END > $6")
	require.Contains(t, db.sql, "(v5::jsonb ->> 'startsAt')::timestamptz END <= $6")
	require.Contains(t, db.sql, "CASE WHEN v5 LIKE '%\"source\":\"iga_jit\"%' THEN v5::jsonb ->> 'lifecycleId' END = $7")
}

func TestEnforceJIT_DatabaseDenialFailsClosed(t *testing.T) {
	db := &stubQueryRower{allowed: false}
	allowed, err := enforceJIT(context.Background(), db, "enforcer-1", "lifecycle-a", time.Now(),
		"alice", "acme", "orders", "get")
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestEnforceJIT_InvalidRequestDoesNotQuery(t *testing.T) {
	db := &stubQueryRower{allowed: true}
	allowed, err := enforceJIT(context.Background(), db, "enforcer-1", "lifecycle-a", time.Now(), "alice", 42, "orders", "get")
	require.NoError(t, err)
	require.False(t, allowed)
	require.Empty(t, db.sql)
}

func TestEnforceJIT_EmptyLifecycleDoesNotQuery(t *testing.T) {
	for _, lifecycleID := range []string{"", "   "} {
		db := &stubQueryRower{allowed: true}
		allowed, err := enforceJIT(context.Background(), db, "enforcer-1", lifecycleID, time.Now(), "alice", "acme", "orders", "get")
		require.NoError(t, err)
		require.False(t, allowed)
		require.Empty(t, db.sql)
	}
}

func TestEnforceJIT_SessionCannotBorrowAnotherLifecycle(t *testing.T) {
	for _, tt := range []struct {
		name        string
		lifecycleID string
		want        bool
	}{
		{name: "same lifecycle and window", lifecycleID: "lifecycle-a", want: true},
		{name: "different lifecycle", lifecycleID: "lifecycle-b", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := &stubQueryRower{allowed: true, requiredLifecycle: "lifecycle-a"}
			allowed, err := enforceJIT(context.Background(), db, "enforcer-1", tt.lifecycleID,
				time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC), "alice", "acme", "orders", "get")
			require.NoError(t, err)
			require.Equal(t, tt.want, allowed)
			require.Equal(t, tt.lifecycleID, db.args[6])
		})
	}
}

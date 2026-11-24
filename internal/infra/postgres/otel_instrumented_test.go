package postgres

import (
	"context"
	"os"
	"testing"

	sessionsrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// testDSN returns a DSN to the local GuliangIAM Postgres, or skips the test
// if no Postgres is available. Prefers POSTGRES_DSN / DATABASE_URL env.
func testDSN(t *testing.T) string {
	t.Helper()
	for _, env := range []string{"POSTGRES_DSN", "DATABASE_URL"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return "postgres://guliangiam:guliangiam@/guliangiam?host=/tmp&sslmode=disable"
}

// newTestTracerProvider wires an InMemoryExporter so the test can assert on
// span names, attributes, status and parent-child relationships.
func newTestTracerProvider(t *testing.T) (*sdktrace.TracerProvider, *tracetest.InMemoryExporter) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp), // Syncer → flush immediately, no batch lag
	)
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { tp.ForceFlush(context.Background()) })
	return tp, exp
}

// connectPool opens a pgxpool and wraps it with the instrumentedDBTX — exactly
// what bootstrap/runtime.go does for production.
func connectPool(t *testing.T) *instrumentedDBTX {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN(t))
	require.NoError(t, err, "pgxpool.New")
	require.NoError(t, pool.Ping(context.Background()), "pgxpool.Ping — Postgres not reachable")
	t.Cleanup(func() { pool.Close() })
	return NewInstrumentedDBTX(pool)
}

// parentSpan starts an HTTP-style root span whose context we pass into repo
// calls — every DB span must show up as a child of this root.
func parentSpan(ctx context.Context, tp *sdktrace.TracerProvider, name string) (context.Context, trace.Span) {
	tracer := tp.Tracer("test")
	return tracer.Start(ctx, name)
}

// findSpan returns the first exported SpanStub matching the predicate, or nil.
func findSpan(stubs []tracetest.SpanStub, match func(tracetest.SpanStub) bool) *tracetest.SpanStub {
	for i := range stubs {
		if match(stubs[i]) {
			return &stubs[i]
		}
	}
	return nil
}

// ── Tests ────────────────────────────────────────────────────────────────

// TestDBTX_SpanNames verifies auto-derived span naming for the four SQL types.
// Pure unit — no DB.
func TestDBTX_SpanNames(t *testing.T) {
	cases := []struct {
		sql      string
		expected string
	}{
		{`SELECT * FROM guliangiam.users WHERE id = $1`, "SELECT users"},
		{`INSERT INTO sessions (id, user_id) VALUES ($1, $2)`, "INSERT sessions"},
		{`UPDATE guliangiam.sessions SET revoked_at = $1 WHERE id = $2`, "UPDATE sessions"},
		{`DELETE FROM login_records WHERE id = $1`, "DELETE login_records"},
		{`DELETE FROM tmp`, "DELETE tmp"},
		{`WITH cte AS (SELECT 1) SELECT * FROM cte`, "WITH cte"},
		{`SELECT u.id FROM users u JOIN orgs o ON u.org_id = o.id`, "SELECT users"},
		{`SELECT * FROM public.foo`, "SELECT foo"}, // strip schema
		{`SELECT * FROM "Users"`, "SELECT users"},  // extractTable lowercases
		{`INSERT INTO sessions VALUES ($1)`, "INSERT sessions"},
	}
	for _, c := range cases {
		got := dbSpanName(c.sql)
		assert.Equal(t, c.expected, got, "sql=%q", c.sql)
	}
}

// TestDBTX_DBTX_Exec_Integration: instrumentedDBTX.Exec with real pgxpool →
// DB span emitted, parented under HTTP root, attributes present, status = Ok.
func TestDBTX_DBTX_Exec_Integration(t *testing.T) {
	tp, exp := newTestTracerProvider(t)
	wrapped := connectPool(t)

	httpCtx, httpSpan := parentSpan(context.Background(), tp, "POST /api/v1/test")

	_, err := wrapped.Exec(httpCtx, "SELECT 1")
	require.NoError(t, err)
	httpSpan.End()
	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exp.GetSpans()
	logSpans(t, spans)
	require.GreaterOrEqual(t, len(spans), 2)

	httpRoot := findSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "POST /api/v1/test" })
	dbSpan := findSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "SELECT" })

	require.NotNil(t, httpRoot, "HTTP root span missing")
	require.NotNil(t, dbSpan, "DB span missing — instrumentedDBTX did not emit")

	// Parent check: DB span's parent SpanID == HTTP root's SpanID.
	assert.Equal(t, httpRoot.SpanContext.SpanID(), dbSpan.Parent.SpanID(),
		"DB span must be child of HTTP root")

	assert.Equal(t, trace.SpanKindClient, dbSpan.SpanKind)
	assert.Equal(t, codes.Ok, dbSpan.Status.Code)

	assertAttrKV(t, dbSpan, semconv.DBSystemPostgreSQL)
	assertAttrPresent(t, dbSpan, semconv.DBOperationNameKey)
}

// TestDBTX_DBTX_QueryRow_Integration verifies QueryRow produces a span and
// records the Scan-time error status — pgx.Row only surfaces errors at Scan().
func TestDBTX_DBTX_QueryRow_Integration(t *testing.T) {
	tp, exp := newTestTracerProvider(t)
	wrapped := connectPool(t)

	httpCtx, httpSpan := parentSpan(context.Background(), tp, "GET /api/v1/resource/404")

	// QueryRow returning no row → Scan returns pgx.ErrNoRows (a real error).
	var v int
	row := wrapped.QueryRow(httpCtx, "SELECT 1 WHERE 1=0")
	scanErr := row.Scan(&v)
	httpSpan.End()
	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exp.GetSpans()
	dbSpan := findSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "SELECT" })
	require.NotNil(t, dbSpan, "DB span for QueryRow missing")

	assert.Equal(t, codes.Error, dbSpan.Status.Code,
		"Scan returning pgx.ErrNoRows must mark the DB span as Error")
	assert.NotEmpty(t, dbSpan.Status.Description, "error span should have description")

	// Original Scan error is still propagated — wrapper never swallows errors.
	assert.Error(t, scanErr)
}

// TestDBTX_SessionsRepo_Integration drives the whole chain:
//
//	HTTP root span → Sessions repo GetByID → instrumentedDBTX.QueryRow →
//	DB span "SELECT sessions" parented under the HTTP root.
//
// This proves the cached repo → instrumentedDBTX combo produces correct spans
// end-to-end without any repo changes.
func TestDBTX_SessionsRepo_Integration(t *testing.T) {
	tp, exp := newTestTracerProvider(t)
	wrapped := connectPool(t)
	repo := sessionsrepo.NewRepository(wrapped)

	httpCtx, httpSpan := parentSpan(context.Background(), tp, "GET /api/v1/sessions/not-exist")

	// Non-existent session → repo returns NotFound → QueryRow Scan ErrNoRows.
	_, err := repo.GetByID(httpCtx, "00000000-0000-0000-0000-000000000000")
	require.Error(t, err)
	httpSpan.End()
	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exp.GetSpans()
	logSpans(t, spans)
	require.GreaterOrEqual(t, len(spans), 2)

	httpRoot := findSpan(spans, func(s tracetest.SpanStub) bool {
		return s.Name == "GET /api/v1/sessions/not-exist"
	})
	require.NotNil(t, httpRoot)

	dbSpan := findSpan(spans, func(s tracetest.SpanStub) bool {
		return s.Name == "SELECT sessions"
	})
	require.NotNil(t, dbSpan, "DB span 'SELECT sessions' missing — repo query did not produce span")

	assert.Equal(t, httpRoot.SpanContext.SpanID(), dbSpan.Parent.SpanID(),
		"DB span must be child of HTTP root")
	assert.Equal(t, trace.SpanKindClient, dbSpan.SpanKind)
	assertAttrKV(t, dbSpan, semconv.DBSystemPostgreSQL)

	// Scan ErrNoRows → Error status.
	assert.Equal(t, codes.Error, dbSpan.Status.Code)
}

// TestDBTX_Transaction_Integration: Begin → Tx.Exec → Commit produces a DB
// span inside the tx with db.transaction=true.
func TestDBTX_Transaction_Integration(t *testing.T) {
	tp, exp := newTestTracerProvider(t)
	wrapped := connectPool(t)

	httpCtx, httpSpan := parentSpan(context.Background(), tp, "POST /api/v1/tx-demo")

	tx, err := wrapped.Begin(httpCtx)
	require.NoError(t, err, "Begin")

	_, err = tx.Exec(httpCtx, "SELECT 1")
	require.NoError(t, err)
	require.NoError(t, tx.Commit(httpCtx))

	httpSpan.End()
	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exp.GetSpans()
	logSpans(t, spans)

	dbSpan := findSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "SELECT" })
	require.NotNil(t, dbSpan, "DB span inside tx missing")

	assert.Equal(t, trace.SpanKindClient, dbSpan.SpanKind)
	assert.Equal(t, codes.Ok, dbSpan.Status.Code)
	assertAttrKV(t, dbSpan, attribute.String("db.transaction", "true"))
}

// ── helpers ───────────────────────────────────────────────────────────────

func logSpans(t *testing.T, spans []tracetest.SpanStub) {
	t.Helper()
	t.Logf("exported spans (%d):", len(spans))
	for i := range spans {
		s := &spans[i]
		t.Logf("  %-35s kind=%-7s status=%-5s parent=%s",
			s.Name, s.SpanKind, s.Status.Code, s.Parent.SpanID())
	}
}

// assertAttrKV checks that the span has an attribute matching the given
// KeyValue (both key and value must match).
func assertAttrKV(t *testing.T, span *tracetest.SpanStub, want attribute.KeyValue) {
	t.Helper()
	for _, a := range span.Attributes {
		if a.Key == want.Key {
			assert.Equal(t, want.Value.AsString(), a.Value.AsString(),
				"span %q attribute %q", span.Name, want.Key)
			return
		}
	}
	t.Errorf("span %q missing attribute %q", span.Name, want.Key)
}

func assertAttrPresent(t *testing.T, span *tracetest.SpanStub, key attribute.Key) {
	t.Helper()
	for _, a := range span.Attributes {
		if a.Key == key {
			return
		}
	}
	t.Errorf("span %q missing attribute %q", span.Name, key)
}

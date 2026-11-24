package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	casbininfra "github.com/guliangiam/guliangiam/internal/infra/casbin"
	infrapg "github.com/guliangiam/guliangiam/internal/infra/postgres"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
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

// ── fixtures ─────────────────────────────────────────────────────────────

func e2eDSN(t *testing.T) string {
	t.Helper()
	for _, env := range []string{"POSTGRES_DSN", "DATABASE_URL"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	t.Skip("POSTGRES_DSN / DATABASE_URL not set — e2e trace test skipped")
	return ""
}

// newInMemoryTP installs a test-only TracerProvider backed by InMemoryExporter.
// Returns the exporter (for span inspection). The caller MUST call
// setInMemoryTPAfterRuntime(runtime.DB.Pool) *after* NewRuntime — NewRuntime
// unconditionally calls initTelemetry which overwrites the TP.
func newInMemoryTP(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		tp.ForceFlush(context.Background())
		_ = tp.Shutdown(context.Background())
	})
	return exp
}

// setInMemoryTPAfterRuntime re-installs the InMemoryExporter-backed TP after
// NewRuntime has overwritten it with the production stdout exporter. The TP
// caches tracers, but otel.GetTracerProvider() is re-evaluated every call —
// so replacing the global provider is enough for all subsequent spans
// (including TracingMiddleware, dbTracer(), and any explicit tracer.Start).
func setInMemoryTPAfterRuntime(exp *tracetest.InMemoryExporter) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	otel.SetTracerProvider(tp)
}

// newInstrumentedUoW wraps a raw pgxpool with instrumentedDBTX exactly as
// runtime.go:323 does, then builds the full postgresuow.
func newInstrumentedUoW(pool *pgxpool.Pool) postgresuow.UnitOfWork {
	return postgresuow.New(infrapg.NewInstrumentedDBTX(pool))
}

// ── Tests ────────────────────────────────────────────────────────────────

// TestE2E_Trace_UserRepo_GetByID: repo → instrumentedDBTX → DB span chain.
// Uses the same postgresuow construction as NewRuntime.
func TestE2E_Trace_UserRepo_GetByID(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set")
	}
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx), "DB not reachable — skip")

	exp := newInMemoryTP(t) // pre-runtime install (no-op here, pool has no init side-effects)

	uow := newInstrumentedUoW(pool)
	tracer := otel.GetTracerProvider().Tracer("e2e-test")

	t.Run("existing user produces SELECT users span parented under HTTP root", func(t *testing.T) {
		exp.Reset()
		// Seed UUIDs change between seed rewrites; take a real row instead of a
		// hard-coded id so the test asserts the instrumentation, not the dataset.
		var userID string
		require.NoError(t, pool.QueryRow(ctx, "SELECT id::text FROM guliangiam.users LIMIT 1").Scan(&userID))
		spanName := "GET /api/v1/users/" + userID
		httpCtx, httpSpan := tracer.Start(ctx, spanName)
		user, err := uow.Users().GetByID(httpCtx, userID)
		httpSpan.End()
		require.NoError(t, err)
		require.NotNil(t, user)

		spans := exp.GetSpans()
		require.GreaterOrEqual(t, len(spans), 2)

		httpRoot := firstSpan(spans, func(s tracetest.SpanStub) bool {
			return s.Name == spanName
		})
		require.NotNil(t, httpRoot)

		dbSpan := firstSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "SELECT users" })
		require.NotNil(t, dbSpan, "DB span 'SELECT users' missing — instrumentedDBTX not emitting?")

		assert.Equal(t, httpRoot.SpanContext.SpanID(), dbSpan.Parent.SpanID(),
			"DB span must be direct child of HTTP root")
		assert.Equal(t, trace.SpanKindClient, dbSpan.SpanKind)
		assertSpanHasAttr(t, dbSpan, semconv.DBSystemPostgreSQL)
		assertSpanStatusOK(t, dbSpan)
	})

	t.Run("missing user marks DB span as Error (Scan ErrNoRows)", func(t *testing.T) {
		exp.Reset()
		httpCtx, httpSpan := tracer.Start(ctx, "GET /api/v1/users/unknown")
		_, err := uow.Users().GetByID(httpCtx, "00000000-0000-0000-0000-000000000000")
		httpSpan.End()
		require.Error(t, err, "want NotFound for unknown id")

		spans := exp.GetSpans()
		dbSpan := firstSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "SELECT users" })
		require.NotNil(t, dbSpan)
		assertSpanStatusError(t, dbSpan, "Scan ErrNoRows must mark span=Error")
	})
}

// TestE2E_Trace_SessionRepo_QueryRow: 我们核心 P99 热点 — Session Lookup。
// 走 Sessions repo → QueryRow → instrumentedRow wrapper。
func TestE2E_Trace_SessionRepo_QueryRow(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, e2eDSN(t))
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx))

	exp := newInMemoryTP(t)
	uow := newInstrumentedUoW(pool)
	tracer := otel.GetTracerProvider().Tracer("e2e-test")

	// Resolve a real session row; the seed does not guarantee one, so skip rather
	// than fail when the dataset has none (instrumentation is covered by
	// TestE2E_Trace_UserRepo_GetByID).
	var sessionID string
	if err := pool.QueryRow(ctx, "SELECT id::text FROM guliangiam.sessions LIMIT 1").Scan(&sessionID); err != nil {
		t.Skip("no session row in the target database")
	}
	spanName := "GET /api/v1/sessions/" + sessionID
	httpCtx, httpSpan := tracer.Start(ctx, spanName)
	sess, err := uow.Sessions().GetByID(httpCtx, sessionID)
	httpSpan.End()
	require.NoError(t, err)
	require.NotNil(t, sess)

	spans := exp.GetSpans()
	dbSpan := firstSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "SELECT sessions" })
	require.NotNil(t, dbSpan, "P99热点 Session Lookup 必须产生 'SELECT sessions' DB span")

	httpRoot := firstSpan(spans, func(s tracetest.SpanStub) bool {
		return s.Name == spanName
	})
	require.NotNil(t, httpRoot)
	assert.Equal(t, httpRoot.SpanContext.SpanID(), dbSpan.Parent.SpanID())
}

// TestE2E_Trace_HTTP_Healthz: 完整 HTTP middleware chain 验证。
// NewRuntimeFromEnv → chi router → httptest.Request → TracingMiddleware。
// 关键断言：TracingMiddleware 必须产生 "GET /healthz" span，不管 handler 做不做 DB 查询。
func TestE2E_Trace_HTTP_Healthz(t *testing.T) {
	ctx := context.Background()
	_ = e2eDSN(t) // Skip if no DB DSN — runtime construction below needs it.

	runtime, err := NewRuntimeFromEnv(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	// initTelemetry 在 NewRuntime 里覆盖了 TP — 重新装 InMemoryExporter
	exp := newInMemoryTP(t)
	setInMemoryTPAfterRuntime(exp)

	router := NewChiRouter(runtime)

	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	spans := exp.GetSpans()
	require.GreaterOrEqual(t, len(spans), 1, "TracingMiddleware 必须产生至少 1 个 span")

	httpRoot := firstSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "GET /healthz" })
	require.NotNil(t, httpRoot, "HTTP root span 'GET /healthz' 缺失 → TracingMiddleware 未接入？")

	// semconv attributes — 这确认 TracingMiddleware 走的是正确的 OTel API。
	// TracingMiddleware uses hand-written attribute keys (legacy OTel style).
	assertSpanHasAttrWithValue(t, httpRoot, attribute.Key("http.method"), attribute.StringValue("GET"))
	assertSpanHasAttrWithValue(t, httpRoot, attribute.Key("http.url"), attribute.StringValue("/healthz"))
}

// TestE2E_Trace_Transaction: Begin → Exec → Commit 走 instrumentedTx。
func TestE2E_Trace_Transaction(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, e2eDSN(t))
	require.NoError(t, err)
	defer pool.Close()

	exp := newInMemoryTP(t)

	// 用 instrumentedDBTX 包装原始 pool（也满足 Begin 接口）
	wrapped := infrapg.NewInstrumentedDBTX(pool)
	tracer := otel.GetTracerProvider().Tracer("e2e-test")

	httpCtx, httpSpan := tracer.Start(ctx, "POST /api/v1/sessions/revoke-all")
	tx, err := wrapped.Begin(httpCtx)
	require.NoError(t, err)

	_, err = tx.Exec(httpCtx, "SELECT 1")
	require.NoError(t, err)
	require.NoError(t, tx.Commit(httpCtx))
	httpSpan.End()

	spans := exp.GetSpans()
	dbSpan := firstSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "SELECT" })
	require.NotNil(t, dbSpan, "Tx 内 Exec 必须产生 DB span")

	assertSpanHasAttrWithValue(t, dbSpan, attribute.Key("db.transaction"), attribute.StringValue("true"))
	assertSpanStatusOK(t, dbSpan)
}

// ── helpers ──────────────────────────────────────────────────────────────

func firstSpan(spans []tracetest.SpanStub, match func(tracetest.SpanStub) bool) *tracetest.SpanStub {
	for i := range spans {
		if match(spans[i]) {
			return &spans[i]
		}
	}
	return nil
}

func assertSpanHasAttr(t *testing.T, span *tracetest.SpanStub, want attribute.KeyValue) {
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

func assertSpanHasAttrWithValue(t *testing.T, span *tracetest.SpanStub, key attribute.Key, want attribute.Value) {
	t.Helper()
	for _, a := range span.Attributes {
		if a.Key == key {
			assert.Equal(t, want.AsString(), a.Value.AsString(),
				"span %q attribute %q", span.Name, key)
			return
		}
	}
	t.Errorf("span %q missing attribute %q", span.Name, key)
}

func assertSpanStatusOK(t *testing.T, span *tracetest.SpanStub) {
	t.Helper()
	assert.Equal(t, codes.Ok, span.Status.Code, "span %q status must be Ok", span.Name)
}

func assertSpanStatusError(t *testing.T, span *tracetest.SpanStub, msg ...string) {
	t.Helper()
	assert.Equal(t, codes.Error, span.Status.Code, "span %q status must be Error — %v", span.Name, msg)
	require.NotEmpty(t, span.Status.Description, "Error span must have description")
}

// TestE2E_Trace_Casbin_Enforce: 真实 Casbin Manager → authz.casbin.enforce span → HTTP parent.
// 验证 OTel 链：HTTP root → [DB span（第一次加载 policy）] → authz.casbin.enforce（带 subject/resource/action/enforcer_id 属性）。
// 种子数据: enforcerID=01a0a59d-42ee-7c98-b832-d45b38475293, modelID=01a0a59d-42d0-76bf-8a9d-2b1228e26c7b
func TestE2E_Trace_Casbin_Enforce(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set")
	}
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx))

	exp := newInMemoryTP(t)

	// 从 models 表拿真实 model_text（Manager.GetOrCreateEnforcer 需要）。
	// 表在 guliangiam schema 下，且种子 UUID 会随种子重写变化，因此按名称解析。
	var modelText string
	var modelID, enforcerID string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT id::text, model_text FROM guliangiam.models WHERE name = 'built-in-model' LIMIT 1").Scan(&modelID, &modelText))

	// 构造真实 Casbin Manager
	adapter := casbininfra.NewAdapter(pool)
	mgr := casbininfra.NewManager(adapter)

	require.NoError(t, pool.QueryRow(ctx,
		"SELECT id::text FROM guliangiam.enforcers WHERE name = 'built-in-enforcer' LIMIT 1").Scan(&enforcerID))

	t.Run("admin 对 built-in 域 GET /users 应产生 authz.casbin.enforce span", func(t *testing.T) {
		exp.Reset()
		tracer := otel.GetTracerProvider().Tracer("e2e-test")

		httpCtx, httpSpan := tracer.Start(ctx, "GET /api/v1/users")

		// 第一次 Enforce 会触发 LoadPolicy → 产生 DB span
		_, err := mgr.Enforce(httpCtx, enforcerID, modelText, "admin", "built-in", "/api/v1/users", "GET")
		httpSpan.End()
		require.NoError(t, err)
		// allow 的正确性由 app 层 Service 测试覆盖；此处只验证 trace 链

		spans := exp.GetSpans()
		require.GreaterOrEqual(t, len(spans), 2, "至少有 HTTP root + DB/Enforce span")

		casbinSpan := firstSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "authz.casbin.enforce" })
		require.NotNil(t, casbinSpan, "缺少 authz.casbin.enforce span → Manager.Enforce 未 emit OTel")

		// span 属性：subject=admin, resource=built-in, action=/api/v1/users, enforcer_id=种子 ID
		assertSpanHasAttrWithValue(t, casbinSpan, attribute.Key("casbin.subject"), attribute.StringValue("admin"))
		assertSpanHasAttrWithValue(t, casbinSpan, attribute.Key("casbin.resource"), attribute.StringValue("built-in"))
		assertSpanHasAttrWithValue(t, casbinSpan, attribute.Key("casbin.action"), attribute.StringValue("/api/v1/users"))
		assertSpanHasAttrWithValue(t, casbinSpan, attribute.Key("casbin.enforcer_id"), attribute.StringValue(enforcerID))

		// span 状态 OK
		assertSpanStatusOK(t, casbinSpan)

		// parent 链：casbin span 的 parent 是 HTTP root span
		httpRoot := firstSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "GET /api/v1/users" })
		require.NotNil(t, httpRoot)
		assert.Equal(t, httpRoot.SpanContext.SpanID(), casbinSpan.Parent.SpanID(),
			"authz.casbin.enforce 必须是 HTTP root span 的子节点")
	})

	t.Run("未知 subject 应拒绝但 span 仍 OK（业务拒绝不是引擎错误）", func(t *testing.T) {
		exp.Reset()
		tracer := otel.GetTracerProvider().Tracer("e2e-test")

		httpCtx, httpSpan := tracer.Start(ctx, "GET /api/v1/users")
		allowed, err := mgr.Enforce(httpCtx, enforcerID, modelText, "unknown-user", "built-in", "/api/v1/users", "GET")
		httpSpan.End()
		require.NoError(t, err, "引擎不应返回 error，只有 allow=false")
		require.False(t, allowed)

		spans := exp.GetSpans()
		casbinSpan := firstSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "authz.casbin.enforce" })
		require.NotNil(t, casbinSpan)
		assertSpanStatusOK(t, casbinSpan)
	})
}

// TestE2E_Trace_Casbin_BatchEnforce: 批量策略检查产生 batch_enforce span + batch_size 属性。
func TestE2E_Trace_Casbin_BatchEnforce(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set")
	}
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx))

	exp := newInMemoryTP(t)
	// Resolve the built-in model/enforcer by name instead of hard-coding seed UUIDs:
	// the ids change with every seed rewrite, and the tables live in the
	// `guliangiam` schema (an unqualified `models` does not resolve on a real DB).
	var modelText string
	var modelID, enforcerID string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT id::text, model_text FROM guliangiam.models WHERE name = 'built-in-model' LIMIT 1").Scan(&modelID, &modelText))
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT id::text FROM guliangiam.enforcers WHERE name = 'built-in-enforcer' LIMIT 1").Scan(&enforcerID))

	adapter := casbininfra.NewAdapter(pool)
	mgr := casbininfra.NewManager(adapter)

	tracer := otel.GetTracerProvider().Tracer("e2e-test")
	httpCtx, httpSpan := tracer.Start(ctx, "POST /api/v1/permissions/batch-enforce")

	results, err := mgr.BatchEnforce(httpCtx, enforcerID, modelText, [][]interface{}{
		{"admin", "built-in", "/api/v1/users", "GET"},
		{"admin", "built-in", "/api/v1/users", "POST"},
	})
	httpSpan.End()
	require.NoError(t, err)
	require.Len(t, results, 2)

	spans := exp.GetSpans()
	batchSpan := firstSpan(spans, func(s tracetest.SpanStub) bool { return s.Name == "authz.casbin.batch_enforce" })
	require.NotNil(t, batchSpan, "缺少 authz.casbin.batch_enforce span")
	assertSpanHasAttrWithValue(t, batchSpan, attribute.Key("casbin.batch_size"), attribute.IntValue(2))
	assertSpanStatusOK(t, batchSpan)
}

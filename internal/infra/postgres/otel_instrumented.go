// Package postgres provides infrastructure wiring for PostgreSQL including
// the OTel-instrumented DBTX wrapper that adds distributed tracing and
// Prometheus metrics to every repository query without touching any repo.
package postgres

import (
	"context"
	"strings"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// dbTracer returns the active tracer for database operations.
// It is evaluated on every call (not cached at init time) so tests can swap
// TracerProviders via otel.SetTracerProvider and have subsequent DB spans
// routed to the new provider. In production otel.GetTracerProvider() is a
// trivial in-memory lookup after the initial bootstrap.
func dbTracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer(
		"github.com/guliangiam/guliangiam/internal/infra/postgres",
		trace.WithInstrumentationVersion("1.0.0"),
	)
}

// dbSpanName derives a readable span name from a SQL statement.
// Examples:
//
//	"SELECT * FROM users WHERE id = $1"        → "SELECT users"
//	"INSERT INTO applications (slug, ...) ..." → "INSERT applications"
//	"UPDATE sessions SET ... WHERE id = $1"    → "UPDATE sessions"
//	"DELETE FROM tmp WHERE ..."                → "DELETE tmp"
func dbSpanName(sql string) string {
	// Collapse whitespace for reliable parsing.
	sql = strings.Join(strings.Fields(sql), " ")
	upper := strings.ToUpper(sql)

	// Find the first keyword.
	var keyword string
	switch {
	case strings.HasPrefix(upper, "SELECT"):
		keyword = "SELECT"
	case strings.HasPrefix(upper, "INSERT"):
		keyword = "INSERT"
	case strings.HasPrefix(upper, "UPDATE"):
		keyword = "UPDATE"
	case strings.HasPrefix(upper, "DELETE"):
		keyword = "DELETE"
	case strings.HasPrefix(upper, "WITH"):
		keyword = "WITH"
	default:
		// Unknown — just return first word.
		parts := strings.SplitN(upper, " ", 2)
		return parts[0]
	}

	// Extract the table name after the keyword (skipping optional INTO/FROM).
	var table string
	rest := upper
	switch keyword {
	case "SELECT":
		// SELECT ... FROM <table>
		if idx := strings.Index(rest, " FROM "); idx != -1 {
			table = extractTable(rest[idx+6:])
		}
	case "INSERT":
		// INSERT INTO <table>  OR  INSERT <table>
		rest = rest[len("INSERT"):]
		rest = strings.TrimSpace(rest)
		rest = strings.TrimPrefix(rest, "INTO ")
		table = extractTable(rest)
	case "UPDATE":
		// UPDATE <table> SET ...
		rest = rest[len("UPDATE"):]
		rest = strings.TrimSpace(rest)
		table = extractTable(rest)
	case "DELETE":
		// DELETE FROM <table>
		if idx := strings.Index(rest, " FROM "); idx != -1 {
			table = extractTable(rest[idx+6:])
		} else {
			rest = rest[len("DELETE"):]
			rest = strings.TrimSpace(rest)
			table = extractTable(rest)
		}
	case "WITH":
		table = "cte"
	}

	if table != "" {
		return keyword + " " + table
	}
	return keyword
}

// extractTable pulls the first identifier from a SQL clause, handling
// both plain names ("users") and schema-qualified ("guliangiam.users").
func extractTable(clause string) string {
	// First token — skip leading whitespace.
	clause = strings.TrimSpace(clause)
	if clause == "" {
		return ""
	}
	// Take until space or paren.
	end := strings.IndexAny(clause, " (,")
	if end == -1 {
		end = len(clause)
	}
	name := clause[:end]
	// Strip schema qualifier (keep last part).
	if dot := strings.LastIndex(name, "."); dot != -1 {
		name = name[dot+1:]
	}
	// Strip quotes.
	name = strings.Trim(name, `"[]`)
	return strings.ToLower(name)
}

// instrumentedDBTX wraps a shared.DBTX with OTel spans and Prometheus metrics.
// Every Exec/Query/QueryRow call creates a named child span under the current
// context and is also routed through shared.MonitorQuery (which records
// query_total, query_duration histograms and slow-query warnings).
type instrumentedDBTX struct {
	inner shared.DBTX
}

// NewInstrumentedDBTX returns a shared.DBTX wrapper that adds OTel tracing
// and Prometheus metrics to every database call.
//
// Usage in bootstrap:
//
//	rawPool, _ := pgxpool.New(ctx, dsn)
//	wrappedPool := postgres.NewInstrumentedDBTX(rawPool)  // implements shared.DBTX
//	uow := postgresuow.New(beginnerAdapter{wrapped: wrappedPool, raw: rawPool})
//
// The wrapper also handles pgx.Tx — Begin() returns an instrumented tx.
func NewInstrumentedDBTX(inner shared.DBTX) *instrumentedDBTX {
	return &instrumentedDBTX{inner: inner}
}

// Unwrap 返回底层 DBTX，支持 infrastructure 层的 runOnConn 等需要直接
// type-switch 到 *pgxpool.Pool 或 pgx.Tx 的场景（绕过 wrapper 的 OTel span 封装）。
func (w *instrumentedDBTX) Unwrap() shared.DBTX {
	return w.inner
}

// Exec implements shared.DBTX.
func (w *instrumentedDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	spanName := dbSpanName(sql)
	ctx, span := dbTracer().Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationName(spanName),
			attribute.String("db.statement", truncateSQL(sql)),
		),
	)
	defer span.End()

	var tag pgconn.CommandTag
	err := shared.MonitorQuery(ctx, sql, func() error {
		var e error
		tag, e = w.inner.Exec(ctx, sql, arguments...)
		return e
	})

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return tag, err
}

// Query implements shared.DBTX.
func (w *instrumentedDBTX) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	spanName := dbSpanName(sql)
	ctx, span := dbTracer().Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationName(spanName),
			attribute.String("db.statement", truncateSQL(sql)),
		),
	)
	defer span.End()

	var rows pgx.Rows
	err := shared.MonitorQuery(ctx, sql, func() error {
		var e error
		rows, e = w.inner.Query(ctx, sql, arguments...)
		return e
	})

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return rows, err
}

// QueryRow implements shared.DBTX. pgx.Row does not surface an error until
// Scan(), so we wrap the call and record the error at that point.
func (w *instrumentedDBTX) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	spanName := dbSpanName(sql)
	_, span := dbTracer().Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationName(spanName),
			attribute.String("db.statement", truncateSQL(sql)),
		),
	)

	// MonitorQuery wraps the row creation + Scan. We intercept Scan
	// via a wrapper pgx.Row to capture the actual error at scan time.
	innerRow := w.inner.QueryRow(ctx, sql, arguments...)
	return &instrumentedRow{
		Row:     innerRow,
		span:    span,
		sql:     sql,
		ctx:     ctx,
		monitor: true,
	}
}

// instrumentedRow wraps pgx.Row so we can capture Scan() errors
// (which is where pgx actually reports the QueryRow error).
type instrumentedRow struct {
	pgx.Row
	span    trace.Span
	sql     string
	ctx     context.Context
	monitor bool
}

func (r *instrumentedRow) Scan(dest ...any) error {
	var scanErr error
	if r.monitor {
		scanErr = shared.MonitorQuery(r.ctx, r.sql, func() error {
			return r.Row.Scan(dest...)
		})
	} else {
		scanErr = r.Row.Scan(dest...)
	}
	if scanErr != nil {
		r.span.SetStatus(codes.Error, scanErr.Error())
		r.span.RecordError(scanErr)
	} else {
		r.span.SetStatus(codes.Ok, "")
	}
	r.span.End()
	return scanErr
}

// truncateSQL limits the statement attribute to a reasonable size
// so long INSERT/UPDATE payloads don't bloat the trace.
const maxStatementLen = 256

func truncateSQL(sql string) string {
	if len(sql) <= maxStatementLen {
		return sql
	}
	return sql[:maxStatementLen] + "…"
}

// Begin starts an instrumented transaction. The returned pgx.Tx
// is itself instrumented — every Exec/Query/QueryRow inside the tx

// Begin starts an instrumented transaction. The returned pgx.Tx is itself
// instrumented — every Exec/Query/QueryRow inside the tx will also produce
// OTel spans and Prometheus metrics.
func (w *instrumentedDBTX) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := w.inner.(interface {
		Begin(ctx context.Context) (pgx.Tx, error)
	}).Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &instrumentedTx{Tx: tx}, nil
}

// instrumentedTx wraps pgx.Tx with OTel spans + MonitorQuery.
type instrumentedTx struct {
	pgx.Tx
}

func (t *instrumentedTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	spanName := dbSpanName(sql)
	ctx, span := dbTracer().Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationName(spanName),
			attribute.String("db.transaction", "true"),
			attribute.String("db.statement", truncateSQL(sql)),
		),
	)
	defer span.End()

	var tag pgconn.CommandTag
	err := shared.MonitorQuery(ctx, sql, func() error {
		var e error
		tag, e = t.Tx.Exec(ctx, sql, arguments...)
		return e
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return tag, err
}

func (t *instrumentedTx) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	spanName := dbSpanName(sql)
	ctx, span := dbTracer().Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationName(spanName),
			attribute.String("db.transaction", "true"),
			attribute.String("db.statement", truncateSQL(sql)),
		),
	)
	defer span.End()

	var rows pgx.Rows
	err := shared.MonitorQuery(ctx, sql, func() error {
		var e error
		rows, e = t.Tx.Query(ctx, sql, arguments...)
		return e
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return rows, err
}

func (t *instrumentedTx) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	spanName := dbSpanName(sql)
	_, span := dbTracer().Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationName(spanName),
			attribute.String("db.transaction", "true"),
			attribute.String("db.statement", truncateSQL(sql)),
		),
	)
	innerRow := t.Tx.QueryRow(ctx, sql, arguments...)
	return &instrumentedRow{
		Row:     innerRow,
		span:    span,
		sql:     sql,
		ctx:     ctx,
		monitor: true,
	}
}

// Confirm Begin satisfies the beginner interface used by the UOW.
var _ interface {
	shared.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
} = (*instrumentedDBTX)(nil)

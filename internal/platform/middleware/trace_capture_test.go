package middleware

import (
	"context"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// TestTraceCapture writes full PEP-chain spans to /tmp/trace_spans.json
// for visualization by scripts/trace-visualize.py.
func TestTraceCapture(t *testing.T) {
	outPath := "/tmp/trace_spans.json"
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	exp, _ := stdouttrace.New(
		stdouttrace.WithWriter(f),
		stdouttrace.WithPrettyPrint(),
	)
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("guliangiam-e2e"),
		),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp, sdktrace.WithMaxExportBatchSize(50)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	defer tp.ForceFlush(context.Background())

	tracer := tp.Tracer("github.com/guliangiam/guliangiam")
	rootCtx := context.Background()

	// ── 完整 PEP 链路 span tree（所有子 span 共享 HTTP 顶层 ctx）──
	// HTTP/{method} {path}            ← TracingMiddleware（顶层）
	//   ├── auth.jwt.validate         ← sessionContextFromHeaders
	//   ├── db.sessions.GetByID       ← Session Lookup (P99 最大贡献者)
	//   ├── authz.casbin.enforce       ← APIAuthorizationMiddleware Gate 5
	//   └── auth.oauth2.token.issue    ← Token Issue (OAuth2 流程)

	httpCtx, httpSpan := tracer.Start(rootCtx, "GET /api/v1/users/42")
	httpSpan.SetAttributes(
		attribute.String("http.method", "GET"),
		attribute.String("http.url", "http://iam.guliangtech.com/api/v1/users/42"),
		attribute.String("http.host", "iam.guliangtech.com"),
	)
	httpSpan.SetStatus(codes.Ok, "")

	// Child 1: JWT Validate
	{
		_, sp := tracer.Start(httpCtx, "auth.jwt.validate")
		time.Sleep(150 * time.Microsecond)
		sp.SetAttributes(
			attribute.String("jwt.subject", "alice@guliangtech.com"),
			attribute.String("jwt.issuer", "https://iam.guliangtech.com"),
			attribute.Int("jwt.claims_count", 7),
		)
		sp.End()
	}

	// Child 2: Session DB Lookup
	{
		_, sp := tracer.Start(httpCtx, "db.sessions.GetByID")
		time.Sleep(12 * time.Millisecond) // DB 查询是 P99 最大贡献者
		sp.SetAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.table", "sessions"),
			attribute.Int("db.rows", 1),
		)
		sp.End()
	}

	// Child 3: Casbin Enforce
	{
		_, sp := tracer.Start(httpCtx, "authz.casbin.enforce")
		time.Sleep(80 * time.Microsecond)
		sp.SetAttributes(
			attribute.String("casbin.subject", "alice@guliangtech.com"),
			attribute.String("casbin.domain", "org-guliang"),
			attribute.String("casbin.resource", "/api/v1/users/*"),
			attribute.String("casbin.action", "GET"),
			attribute.String("casbin.enforcer_id", "default"),
		)
		sp.End()
	}

	// Child 4: Token Issue（OAuth2 Login 流程有）
	{
		_, sp := tracer.Start(httpCtx, "auth.oauth2.token.issue")
		time.Sleep(900 * time.Microsecond)
		sp.SetAttributes(
			attribute.String("oauth2.grant_type", "authorization_code"),
			attribute.String("token.type", "access_token"),
			attribute.Int("token.expires_in", 3600),
			attribute.Int("token.scopes_count", 3),
		)
		sp.End()
	}

	httpSpan.End()
	t.Logf("Trace spans written to %s", outPath)
}

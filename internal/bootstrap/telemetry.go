package bootstrap

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// buildTelemetryResource merges the SDK's default resource with our service name.
//
// The service attribute must be schemaless: resource.Merge refuses to join two
// resources whose semconv schema URLs differ, and resource.Default() already
// pins the schema version shipped with otel/sdk. Passing semconv.SchemaURL here
// (from a semconv package older than the SDK's) made Merge fail, so the fallback
// silently dropped service.name from every span.
func buildTelemetryResource() (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewSchemaless(semconv.ServiceNameKey.String("guliangiam")),
	)
}

func initTelemetry() func(context.Context) error {
	res, err := buildTelemetryResource()
	if err != nil {
		slog.Warn("telemetry: failed to create resource", "error", err)
		res = resource.Default()
	}

	// ⚠️ 不使用 stdouttrace.WithPrettyPrint() —— 每个 span 输出完整 JSON
	// OTel 文档到 stdout，在高频 HTTP+DB 下迅速填充 macOS 日志缓冲区
	// 触发 OOM killer。生产应配置 OTLP exporter，开发环境用 no-op。
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
}

package bootstrap

import (
	"testing"

	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// resource.Merge 拒绝合并 semconv schema 版本不同的两个 resource；此前用
// semconv.SchemaURL 构造服务属性，与 SDK 默认 resource 的 schema 版本不一致，
// 合并失败后静默回落到 resource.Default()，所有 span 都丢失 service.name。
func TestBuildTelemetryResourceKeepsServiceName(t *testing.T) {
	res, err := buildTelemetryResource()
	if err != nil {
		t.Fatalf("merge must succeed: %v", err)
	}

	for _, kv := range res.Attributes() {
		if kv.Key == semconv.ServiceNameKey {
			if got := kv.Value.AsString(); got != "guliangiam" {
				t.Fatalf("service.name = %q, want %q", got, "guliangiam")
			}
			return
		}
	}
	t.Fatal("service.name missing from the merged resource: spans would lose their service identity")
}

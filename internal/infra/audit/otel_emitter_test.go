package audit

import (
	"context"
	"testing"
)

func TestNewOtelEmitter(t *testing.T) {
	emitter := NewOtelEmitter("test-service")
	if emitter == nil {
		t.Error("expected non-nil emitter")
	}
	if emitter.serviceName != "test-service" {
		t.Errorf("serviceName = %q, want test-service", emitter.serviceName)
	}
}

func TestOtelEmitter_EmitTrace(t *testing.T) {
	emitter := NewOtelEmitter("test-service")
	err := emitter.EmitTrace(context.Background(), "test-event", map[string]interface{}{"key": "value"})
	if err != nil {
		t.Errorf("EmitTrace() error = %v", err)
	}
}

func TestOtelEmitter_EmitTrace_NilAttributes(t *testing.T) {
	emitter := NewOtelEmitter("test-service")
	err := emitter.EmitTrace(context.Background(), "test-event", nil)
	if err != nil {
		t.Errorf("EmitTrace() error = %v", err)
	}
}

func TestOtelEmitter_EmitMetric(t *testing.T) {
	emitter := NewOtelEmitter("test-service")
	err := emitter.EmitMetric(context.Background(), "test-metric", 42.0, map[string]interface{}{"key": "value"})
	if err != nil {
		t.Errorf("EmitMetric() error = %v", err)
	}
}

func TestOtelEmitter_EmitMetric_NilAttributes(t *testing.T) {
	emitter := NewOtelEmitter("test-service")
	err := emitter.EmitMetric(context.Background(), "test-metrics", 0.0, nil)
	if err != nil {
		t.Errorf("EmitMetric() error = %v", err)
	}
}

func TestOtelEmitter_EmitLog(t *testing.T) {
	emitter := NewOtelEmitter("test-service")
	err := emitter.EmitLog(context.Background(), "info", "test message", map[string]interface{}{"key": "value"})
	if err != nil {
		t.Errorf("EmitLog() error = %v", err)
	}
}

func TestOtelEmitter_EmitLog_NilAttributes(t *testing.T) {
	emitter := NewOtelEmitter("test-service")
	err := emitter.EmitLog(context.Background(), "error", "error msg", nil)
	if err != nil {
		t.Errorf("EmitLog() error = %v", err)
	}
}

func TestOtelEmitter_Close(t *testing.T) {
	emitter := NewOtelEmitter("test-service")
	err := emitter.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestNewMultiEmitter(t *testing.T) {
	e1 := *NewOtelEmitter("service-1")
	e2 := *NewOtelEmitter("service-2")
	multi := NewMultiEmitter(e1, e2)
	if multi == nil {
		t.Error("expected non-nil multi emitter")
	}
	if len(multi.emitters) != 2 {
		t.Errorf("expected 2 emitters, got %d", len(multi.emitters))
	}
}

func TestMultiEmitter_EmitTrace(t *testing.T) {
	e1 := *NewOtelEmitter("service-1")
	e2 := *NewOtelEmitter("service-2")
	multi := NewMultiEmitter(e1, e2)
	err := multi.EmitTrace(context.Background(), "test-event", map[string]interface{}{"key": "val"})
	if err != nil {
		t.Errorf("EmitTrace() error = %v", err)
	}
}

func TestMultiEmitter_EmitMetric(t *testing.T) {
	e1 := *NewOtelEmitter("service-1")
	multi := NewMultiEmitter(e1)
	err := multi.EmitMetric(context.Background(), "test-metric", 1.0, nil)
	if err != nil {
		t.Errorf("EmitMetric() error = %v", err)
	}
}

func TestMultiEmitter_EmitLog(t *testing.T) {
	e1 := *NewOtelEmitter("service-1")
	multi := NewMultiEmitter(e1)
	err := multi.EmitLog(context.Background(), "info", "msg", nil)
	if err != nil {
		t.Errorf("EmitLog() error = %v", err)
	}
}

func TestMultiEmitter_Empty(t *testing.T) {
	multi := NewMultiEmitter()
	err := multi.EmitTrace(context.Background(), "event", nil)
	if err != nil {
		t.Errorf("EmitTrace() with empty emitters error = %v", err)
	}
}

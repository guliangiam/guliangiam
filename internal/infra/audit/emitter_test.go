package audit

import (
	"context"
	"testing"
)

func TestNoopEmitter_Emit(t *testing.T) {
	emitter := NoopEmitter{}
	err := emitter.Emit(context.Background(), Event{
		Type:       "test",
		ActorID:    "user-1",
		Resource:   "resource",
		ResourceID: "id-1",
	})
	if err != nil {
		t.Errorf("NoopEmitter.Emit() error = %v", err)
	}
}

func TestNoopEmitter_Emit_WithPayload(t *testing.T) {
	emitter := NoopEmitter{}
	err := emitter.Emit(context.Background(), Event{
		Type:    "test",
		Payload: map[string]any{"key": "value"},
	})
	if err != nil {
		t.Errorf("NoopEmitter.Emit() error = %v", err)
	}
}

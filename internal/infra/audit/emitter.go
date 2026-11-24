package audit

import "context"

type Event struct {
	Type       string
	ActorID    string
	Resource   string
	ResourceID string
	Payload    map[string]any
}

type Emitter interface {
	Emit(ctx context.Context, event Event) error
}

type NoopEmitter struct{}

func (NoopEmitter) Emit(context.Context, Event) error {
	return nil
}

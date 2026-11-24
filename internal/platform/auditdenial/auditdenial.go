// Package auditdenial carries the cross-cutting contract for denial audit
// events. Implementations live in internal/app/auditdenial so platform code
// stays free of application/UoW dependencies.
package auditdenial

import "context"

// Event is a denial audit record request. OrganizationID is the source tenant
// (the caller's session tenant); TargetTenant is the rejected target, if any.
type Event struct {
	OrganizationID string
	ActorID        string
	TargetTenant   string
	Action         string
	ResourceType   string
	ResourceID     string
	Reason         string
	IP             string
	UserAgent      string
	Surface        string
	RequestID      string
	TraceID        string
	Metadata       map[string]any
}

// Recorder records one denial event. Implementations must be best-effort:
// failures never alter the HTTP response.
type Recorder interface {
	Record(ctx context.Context, event Event)
}

type recorderCtxKey struct{}

func WithRecorder(ctx context.Context, recorder Recorder) context.Context {
	return context.WithValue(ctx, recorderCtxKey{}, recorder)
}

func FromContext(ctx context.Context) (Recorder, bool) {
	recorder, ok := ctx.Value(recorderCtxKey{}).(Recorder)
	return recorder, ok
}

// Record records through the context recorder, if one is configured. It is a
// no-op in unit tests and on public routes without a recorder.
func Record(ctx context.Context, event Event) {
	if recorder, ok := FromContext(ctx); ok && recorder != nil {
		recorder.Record(ctx, event)
	}
}

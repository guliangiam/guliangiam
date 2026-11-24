package route

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"

	"github.com/guliangiam/guliangiam/internal/platform/logging"
)

type requestMetadataKey struct{}

// RequestMetadata carries the routing/telemetry identity of an in-flight
// request so that audit writers do not have to re-derive it.
type RequestMetadata struct {
	Surface   Surface
	RequestID string
	TraceID   string
}

// WithRequestMetadata returns a context carrying meta.
func WithRequestMetadata(ctx context.Context, meta RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataKey{}, meta)
}

// RequestMetadataFrom returns the metadata stamped on ctx. When no middleware
// stamped it (unit tests, background jobs) it falls back to the management
// surface and an empty request id, which matches the DDL defaults.
func RequestMetadataFrom(ctx context.Context) RequestMetadata {
	if meta, ok := ctx.Value(requestMetadataKey{}).(RequestMetadata); ok {
		return meta
	}
	return RequestMetadata{Surface: SurfaceManagement}
}

// traceIDFromContext returns the W3C trace id of the active span, or an empty
// string when tracing is not recording (matches the DDL default for
// records.trace_id).
func traceIDFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}

// WithSurface returns middleware that stamps the given API face onto every
// request in the group it is mounted on, together with the request id and the
// trace id taken from their respective middlewares.
//
// Stamping the surface per route group (rather than matching the path at
// request time) keeps the value exact: 手册 4.1.3 spreads the protocol face
// over six prefix families, so pattern matching is fragile.
func WithSurface(surface Surface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(stampRequestMetadata(r.Context(), surface)))
		})
	}
}

// RequestMetadataMiddleware stamps the metadata for requests that are not
// mounted on a surface group (root-level protocol routes such as
// /oauth2/*, /.well-known/* and /scim/v2/*). The surface is derived from the
// path so those routes stay correctly tagged without a per-route group.
func RequestMetadataMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := stampRequestMetadata(r.Context(), SurfaceFor(r.URL.Path))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// stampRequestMetadata keeps route.RequestMetadata and the platform logging
// context in lockstep: audit and route error logs must see the same ids the
// error response carries. The logging context is the platform-level hand-off
// that lets platform middleware avoid importing transport.
func stampRequestMetadata(ctx context.Context, surface Surface) context.Context {
	meta := RequestMetadata{
		Surface:   surface,
		RequestID: middleware.GetReqID(ctx),
		TraceID:   traceIDFromContext(ctx),
	}
	ctx = WithRequestMetadata(ctx, meta)

	ids := logging.RequestIDs{
		RequestID: meta.RequestID,
		TraceID:   meta.TraceID,
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasSpanID() {
		ids.SpanID = sc.SpanID().String()
	}
	return logging.WithRequestIDs(ctx, ids)
}

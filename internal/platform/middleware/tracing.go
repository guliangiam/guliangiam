package middleware

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracer returns the active OTel tracer. Evaluated on every call (not cached
// at package init time) so tests can swap TracerProviders via
// otel.SetTracerProvider and production sees the real provider installed by
// initTelemetry. The previous package-level `var tracer = ...` was locked to
// the no-op TracerProvider during import — meaning TracingMiddleware (and
// all the Record* helpers) never emitted real spans in either env.
func tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer("github.com/guliangiam/guliangiam")
}

// TracingMiddleware adds OpenTelemetry tracing to HTTP requests.
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer().Start(r.Context(), r.Method+" "+r.URL.Path,
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.host", r.Host),
				attribute.String("http.scheme", r.URL.Scheme),
				attribute.String("http.user_agent", r.UserAgent()),
			),
		)
		defer span.End()

		next.ServeHTTP(w, r.WithContext(ctx))

		// If the span hasn't been set with an error, mark as OK
		if span.IsRecording() {
			span.SetStatus(codes.Ok, "")
		}
	})
}

// RecordAuthSpan creates a child span for authentication operations.
// ctx should come from the incoming HTTP request (so this span becomes a
// child of the top-level TracingMiddleware span).
func RecordAuthSpan(ctx context.Context, operation string, attrs ...attribute.KeyValue) trace.Span {
	_, span := tracer().Start(ctx, "auth."+operation,
		trace.WithAttributes(attrs...),
	)
	return span
}

// RecordDBSpan creates a child span for database operations.
// ctx should come from the incoming HTTP request.
func RecordDBSpan(ctx context.Context, operation string, attrs ...attribute.KeyValue) trace.Span {
	_, span := tracer().Start(ctx, "db."+operation,
		trace.WithAttributes(attrs...),
	)
	return span
}

// RecordSpan creates a generic child span under the given context.
// Useful for cross-cutting concerns that don't fit auth/db categories.
func RecordSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) trace.Span {
	_, span := tracer().Start(ctx, name,
		trace.WithAttributes(attrs...),
	)
	return span
}

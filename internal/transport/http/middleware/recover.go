package middleware

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"

	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// stackLimit keeps a pathological panic from writing an unbounded stack into
// the log; the first frames are the ones that identify the call site.
const stackLimit = 1500

// Recoverer returns middleware that turns a handler panic into the standard
// error envelope instead of letting chi's default Recoverer write a plain-text
// body. The response goes through route.WriteError, so management/authentication
// faces get {error:{code,message},requestId,traceId} and the protocol face keeps
// its RFC-shaped body.
//
// The log line carries request_id/trace_id/method/path only: headers, query and
// request body may hold credentials or PII and are never logged here.
//
// A panic after the handler has already committed a response is logged but not
// rewritten: appending a JSON error to an in-flight body (SSE, download) would
// corrupt it and cannot change the status that is already on the wire.
func Recoverer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &recoverResponseWriter{ResponseWriter: w}
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				logRecoveredPanic(r, rec)
				if rw.committed {
					return
				}
				route.WriteError(rw, r, apperrors.NewInternal("panic recovered"))
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

func logRecoveredPanic(r *http.Request, rec any) {
	stack := string(debug.Stack())
	if len(stack) > stackLimit {
		stack = stack[:stackLimit] + "\n...[truncated]"
	}
	slog.ErrorContext(r.Context(), "panic recovered in HTTP handler",
		"request_id", chimw.GetReqID(r.Context()),
		"trace_id", trace.SpanFromContext(r.Context()).SpanContext().TraceID().String(),
		"method", r.Method,
		"path", r.URL.Path,
		"panic", fmt.Sprint(rec),
		"stack", stack,
	)
}

// recoverResponseWriter tracks whether the header/body has already been
// written and forwards the optional interfaces handlers rely on (SSE flush,
// WebSocket hijack, HTTP/2 push). Unwrap keeps http.ResponseController able to
// reach the real writer.
type recoverResponseWriter struct {
	http.ResponseWriter
	committed bool
}

func (w *recoverResponseWriter) WriteHeader(code int) {
	w.committed = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *recoverResponseWriter) Write(p []byte) (int, error) {
	w.committed = true
	return w.ResponseWriter.Write(p)
}

func (w *recoverResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *recoverResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *recoverResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (w *recoverResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

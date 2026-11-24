// Package logging owns the process-wide structured log handler: resource
// attributes, request correlation, §12.10 sampling and the 32KB record cap.
//
// It deliberately has no dependency on transport: the request ids are carried
// in a platform-level context key that transport writes and every layer reads.
package logging

import (
	"bytes"
	"context"
	"hash/fnv"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

const (
	// ServiceName is the fixed service.name resource attribute (规范 12.10).
	ServiceName = "guliangiam"
	// MaxRecordBytes is the single-record cap from 规范 12.10: 32KB.
	MaxRecordBytes = 32 * 1024
	// SlowRequestThreshold is the §12.10 slow-request boundary: >500ms keeps
	// the request-level INFO access log at 100%.
	SlowRequestThreshold = 500 * time.Millisecond
	// InfoSamplePercent is the fixed normal-INFO access-log sampling rate.
	// §12.10 fixes the ratio; keeping it a constant avoids a new env var.
	InfoSamplePercent = 10

	truncatedMessageLimit = 2 * 1024
	truncatedValueLimit   = 256
	requestIDLimit        = 128
	traceIDLimit          = 32
	spanIDLimit           = 16
	callerLimit           = 128
)

// Options configures NewJSONHandler. ServiceVersion is expected to come from
// SENTRY_RELEASE (规范 12.10: same source as the release marker).
type Options struct {
	ServiceVersion string
	Environment    string
	Level          slog.Level
}

// RequestIDs is the platform-level correlation triple for one request.
type RequestIDs struct {
	RequestID string
	TraceID   string
	SpanID    string
}

type requestIDsKey struct{}

// WithRequestIDs stores the correlation ids for downstream log calls.
func WithRequestIDs(ctx context.Context, ids RequestIDs) context.Context {
	return context.WithValue(ctx, requestIDsKey{}, ids)
}

// RequestIDsFrom returns the correlation ids for ctx. Values stamped by the
// router win; the chi RequestID and active OTel span are fallbacks for layers
// that run before RequestMetadataMiddleware (for example the access logger,
// which is outside the router group that stamps the metadata).
func RequestIDsFrom(ctx context.Context) RequestIDs {
	ids, _ := ctx.Value(requestIDsKey{}).(RequestIDs)
	if ids.RequestID == "" {
		ids.RequestID = chimw.GetReqID(ctx)
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		if ids.TraceID == "" {
			ids.TraceID = sc.TraceID().String()
		}
		if ids.SpanID == "" && sc.HasSpanID() {
			ids.SpanID = sc.SpanID().String()
		}
	}
	return ids
}

type accessLogKey struct{}

type accessLogMarker struct {
	duration time.Duration
}

// WithAccessLog marks one record as the request-level access log and carries
// the response duration. The handler samples only marked INFO records, so
// application INFO logs and §12.5 security events stay at 100%.
func WithAccessLog(ctx context.Context, duration time.Duration) context.Context {
	return context.WithValue(ctx, accessLogKey{}, accessLogMarker{duration: duration})
}

func accessLogDuration(ctx context.Context) (time.Duration, bool) {
	marker, ok := ctx.Value(accessLogKey{}).(accessLogMarker)
	if !ok {
		return 0, false
	}
	return marker.duration, true
}

// KeepInfoSample reports whether a normal INFO access log with the given
// trace id survives the fixed 10% sample. Hashing makes the decision stable
// for one trace, which is what lets a sampled request keep all of its lines.
func KeepInfoSample(traceID string) bool {
	if traceID == "" {
		// No correlation id: keep the line rather than hide it behind a bucket
		// we cannot reproduce from a trace search.
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(traceID))
	return h.Sum32()%100 < InfoSamplePercent
}

type prefixOp struct {
	group string
	attrs []slog.Attr
}

// Handler composes the JSON sink with the §12.10 rules. It is safe for
// concurrent use: record writes are serialized on a shared mutex.
type Handler struct {
	out   io.Writer
	level slog.Level
	ops   []prefixOp
	mu    *sync.Mutex
}

// NewJSONHandler returns a JSON handler that injects the resource attributes
// (service.name, service.version, env) and per-record caller/correlation ids.
func NewJSONHandler(w io.Writer, opts Options) *Handler {
	if w == nil {
		w = os.Stderr
	}
	resource := prefixOp{attrs: resourceAttrs(opts)}
	return &Handler{
		out:   w,
		level: opts.Level,
		ops:   []prefixOp{resource},
		mu:    &sync.Mutex{},
	}
}

func resourceAttrs(opts Options) []slog.Attr {
	version := strings.TrimSpace(opts.ServiceVersion)
	if version == "" {
		version = "unknown"
	}
	environment := strings.TrimSpace(opts.Environment)
	if environment == "" {
		environment = "development"
	}
	return []slog.Attr{
		slog.String("service.name", ServiceName),
		slog.String("service.version", truncateUTF8(version, truncatedValueLimit)),
		slog.String("env", truncateUTF8(environment, 64)),
	}
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	if !h.Enabled(ctx, record.Level) {
		return nil
	}
	if !h.shouldKeep(ctx, record) {
		return nil
	}

	userAttrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		userAttrs = append(userAttrs, attr)
		return true
	})

	identity := identityAttrs(ctx, record.PC, userAttrs)
	body, err := h.render(ctx, record, record.Message, concatAttrs(identity, userAttrs))
	if err != nil {
		return err
	}
	if len(body) > MaxRecordBytes {
		body, err = h.renderTruncated(ctx, record, identity, userAttrs, len(body))
		if err != nil {
			return err
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err = h.out.Write(body)
	return err
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	next := h.clone()
	if last := len(next.ops) - 1; last >= 0 && next.ops[last].group == "" {
		next.ops[last].attrs = concatAttrs(next.ops[last].attrs, attrs)
		return next
	}
	next.ops = append(next.ops, prefixOp{attrs: attrs})
	return next
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := h.clone()
	next.ops = append(next.ops, prefixOp{group: name})
	return next
}

func (h *Handler) clone() *Handler {
	next := *h
	next.ops = append([]prefixOp(nil), h.ops...)
	for i := range next.ops {
		next.ops[i].attrs = append([]slog.Attr(nil), h.ops[i].attrs...)
	}
	return &next
}

func (h *Handler) render(ctx context.Context, record slog.Record, message string, attrs []slog.Attr) ([]byte, error) {
	var buf bytes.Buffer
	var base slog.Handler = slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: h.level})
	for _, op := range h.ops {
		if op.group != "" {
			base = base.WithGroup(op.group)
			continue
		}
		base = base.WithAttrs(op.attrs)
	}
	next := slog.NewRecord(record.Time, record.Level, message, record.PC)
	next.AddAttrs(attrs...)
	if err := base.Handle(ctx, next); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderTruncated keeps the resource attributes and error fields first, drops
// or shortens the remaining attributes until the line fits 32KB, and appends
// the 规范 12.10 markers.
func (h *Handler) renderTruncated(ctx context.Context, record slog.Record, identity, user []slog.Attr, original int) ([]byte, error) {
	markers := []slog.Attr{
		slog.Bool("log.truncated", true),
		slog.Int("log.original_bytes", original),
	}
	priority, rest := splitPriority(user)
	message := truncateUTF8(record.Message, truncatedMessageLimit)

	for _, messageLimit := range []int{truncatedMessageLimit, 1024, 256, 64, 16} {
		message = truncateUTF8(record.Message, messageLimit)
		kept := append([]slog.Attr(nil), identity...)
		kept = h.appendFit(ctx, record, message, kept, priority, markers)
		kept = h.appendFit(ctx, record, message, kept, rest, markers)
		body, err := h.render(ctx, record, message, concatAttrs(kept, markers))
		if err != nil {
			return nil, err
		}
		if len(body) <= MaxRecordBytes {
			return body, nil
		}
	}

	// Last resort: identity + markers only. Every value here is length-bounded,
	// so this always fits.
	final := concatAttrs(identity, markers)
	return h.render(ctx, record, truncateUTF8(record.Message, 16), final)
}

func (h *Handler) appendFit(ctx context.Context, record slog.Record, message string, kept, candidates, markers []slog.Attr) []slog.Attr {
	for _, attr := range candidates {
		trial := concatAttrs(kept, []slog.Attr{attr})
		body, err := h.render(ctx, record, message, concatAttrs(trial, markers))
		if err == nil && len(body) <= MaxRecordBytes {
			kept = trial
			continue
		}
		short, ok := shortenAttr(attr)
		if !ok {
			continue
		}
		trial = concatAttrs(kept, []slog.Attr{short})
		body, err = h.render(ctx, record, message, concatAttrs(trial, markers))
		if err == nil && len(body) <= MaxRecordBytes {
			kept = trial
		}
	}
	return kept
}

func shortenAttr(attr slog.Attr) (slog.Attr, bool) {
	switch attr.Value.Kind() {
	case slog.KindString:
		value := attr.Value.String()
		if len(value) <= truncatedValueLimit {
			return slog.Attr{}, false
		}
		return slog.String(attr.Key, truncateUTF8(value, truncatedValueLimit)), true
	case slog.KindAny:
		errValue, ok := attr.Value.Any().(error)
		if !ok || len(errValue.Error()) <= truncatedValueLimit {
			return slog.Attr{}, false
		}
		return slog.String(attr.Key, truncateUTF8(errValue.Error(), truncatedValueLimit)), true
	default:
		return slog.Attr{}, false
	}
}

func splitPriority(attrs []slog.Attr) (priority, rest []slog.Attr) {
	for _, attr := range attrs {
		if attr.Key == "error" || strings.HasPrefix(attr.Key, "error.") {
			priority = append(priority, attr)
			continue
		}
		rest = append(rest, attr)
	}
	return priority, rest
}

func identityAttrs(ctx context.Context, pc uintptr, user []slog.Attr) []slog.Attr {
	present := make(map[string]struct{}, len(user)+4)
	for _, attr := range user {
		present[attr.Key] = struct{}{}
	}
	ids := RequestIDsFrom(ctx)
	attrs := make([]slog.Attr, 0, 4)
	if ids.RequestID != "" && !hasKey(present, "request_id") {
		attrs = append(attrs, slog.String("request_id", truncateUTF8(ids.RequestID, requestIDLimit)))
	}
	if ids.TraceID != "" && !hasKey(present, "trace_id") {
		attrs = append(attrs, slog.String("trace_id", truncateUTF8(ids.TraceID, traceIDLimit)))
	}
	if ids.SpanID != "" && !hasKey(present, "span_id") {
		attrs = append(attrs, slog.String("span_id", truncateUTF8(ids.SpanID, spanIDLimit)))
	}
	if !hasKey(present, "caller") {
		attrs = append(attrs, slog.String("caller", truncateUTF8(callerName(pc), callerLimit)))
	}
	return attrs
}

func hasKey(present map[string]struct{}, key string) bool {
	_, ok := present[key]
	return ok
}

func (h *Handler) shouldKeep(ctx context.Context, record slog.Record) bool {
	if record.Level >= slog.LevelWarn {
		return true
	}
	if record.Level != slog.LevelInfo {
		return true
	}
	duration, ok := accessLogDuration(ctx)
	if !ok {
		return true
	}
	if duration > SlowRequestThreshold {
		return true
	}
	ids := RequestIDsFrom(ctx)
	if ids.TraceID == "" {
		// §12.10 samples by trace_id. Without one there is no reproducible
		// bucket, so keep the line instead of dropping it silently.
		return true
	}
	return KeepInfoSample(ids.TraceID)
}

// callerName renders the emitting package as `platform.middleware`, matching
// the 规范 12.10 caller example instead of the full runtime symbol.
func callerName(pc uintptr) string {
	if pc == 0 {
		return "unknown"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	name := strings.TrimPrefix(fn.Name(), "github.com/guliangiam/guliangiam/")
	slash := strings.LastIndex(name, "/")
	pkg := name
	if slash >= 0 {
		pkg = name[slash+1:]
	}
	if dot := strings.IndexByte(pkg, '.'); dot >= 0 {
		pkg = pkg[:dot]
	}
	if slash < 0 {
		return pkg
	}
	parent := name[:slash]
	if cut := strings.LastIndexByte(parent, '/'); cut >= 0 {
		parent = parent[cut+1:]
	}
	return parent + "." + pkg
}

func concatAttrs(left, right []slog.Attr) []slog.Attr {
	out := make([]slog.Attr, 0, len(left)+len(right))
	out = append(out, left...)
	out = append(out, right...)
	return out
}

func truncateUTF8(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	if limit <= 0 {
		return "…"
	}
	cut := value[:limit]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + "…"
}

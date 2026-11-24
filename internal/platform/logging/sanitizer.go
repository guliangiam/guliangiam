package logging

import (
	"context"
	"log/slog"

	"github.com/guliangiam/guliangiam/internal/platform/redact"
)

// LogSanitizer redacts sensitive attribute values before they reach a log sink.
// The field list lives in internal/platform/redact so the log face and the HTTP
// read faces cannot disagree about what is sensitive.
type LogSanitizer struct{}

func NewLogSanitizer() *LogSanitizer { return &LogSanitizer{} }

func (s *LogSanitizer) SanitizeValue(key string, value any) any {
	if s.isSensitiveKey(key) {
		return redact.Placeholder
	}
	return value
}

func (s *LogSanitizer) isSensitiveKey(key string) bool {
	return redact.IsSensitiveField(key)
}

type SanitizingHandler struct {
	handler   slog.Handler
	sanitizer *LogSanitizer
}

func NewSanitizingHandler(handler slog.Handler) *SanitizingHandler {
	return &SanitizingHandler{
		handler:   handler,
		sanitizer: NewLogSanitizer(),
	}
}

func (h *SanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *SanitizingHandler) Handle(ctx context.Context, record slog.Record) error {
	var sanitizedAttrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		sanitizedAttrs = append(sanitizedAttrs, h.sanitizeAttr(attr))
		return true
	})

	newRecord := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	for _, attr := range sanitizedAttrs {
		newRecord.AddAttrs(attr)
	}
	return h.handler.Handle(ctx, newRecord)
}

func (h *SanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		sanitized[i] = h.sanitizeAttr(attr)
	}
	return &SanitizingHandler{
		handler:   h.handler.WithAttrs(sanitized),
		sanitizer: h.sanitizer,
	}
}

func (h *SanitizingHandler) WithGroup(name string) slog.Handler {
	return &SanitizingHandler{
		handler:   h.handler.WithGroup(name),
		sanitizer: h.sanitizer,
	}
}

func (h *SanitizingHandler) sanitizeAttr(attr slog.Attr) slog.Attr {
	if h.sanitizer.isSensitiveKey(attr.Key) {
		return slog.String(attr.Key, redact.Placeholder)
	}
	kind := attr.Value.Kind()
	if kind == slog.KindGroup {
		groupAttrs := attr.Value.Group()
		sanitizedGroupAttrs := make([]slog.Attr, len(groupAttrs))
		for i, a := range groupAttrs {
			sanitizedGroupAttrs[i] = h.sanitizeAttr(a)
		}
		return slog.Group(attr.Key, groupAttrsToAny(sanitizedGroupAttrs)...)
	}
	return attr
}

func groupAttrsToAny(attrs []slog.Attr) []any {
	result := make([]any, 0, len(attrs)*2)
	for _, attr := range attrs {
		result = append(result, attr.Key, attr.Value.Any())
	}
	return result
}

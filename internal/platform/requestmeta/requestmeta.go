// Package requestmeta carries the observed transport metadata of an in-flight
// request (client IP, user agent, API surface, request id, trace id).
//
// It exists as a platform-level context carrier because the application and
// audit layers must not import transport/route: the HTTP middleware stamps the
// metadata once, and audit writers read it back when a caller passes an empty
// field.
package requestmeta

import (
	"context"
	"unicode/utf8"
)

// Column widths of the audit evidence columns in contracts/schema/60_audit.yaml.
// The middleware and the audit write path truncate to these widths: a longer
// value (a hostile User-Agent, a client-supplied X-Request-Id) would otherwise
// make the audit INSERT fail and roll back the surrounding business
// transaction.
const (
	MaxUserAgentLen = 255
	MaxRequestIDLen = 255
	MaxTraceIDLen   = 255
)

// Metadata is the request-scoped audit origin evidence.
//
// Surface is stored as a plain string rather than registry.Surface so this
// package stays dependency-free; the audit layer normalizes unknown values
// before persisting (domain/records.NormalizeSurface).
type Metadata struct {
	IP        string
	UserAgent string
	Surface   string
	RequestID string
	TraceID   string
}

type contextKey struct{}

// With returns a context carrying meta.
func With(ctx context.Context, meta Metadata) context.Context {
	return context.WithValue(ctx, contextKey{}, meta)
}

// From returns the metadata stamped on ctx, or the zero value when none was
// stamped (background jobs, unit tests without the middleware).
func From(ctx context.Context) Metadata {
	if ctx == nil {
		return Metadata{}
	}
	if meta, ok := ctx.Value(contextKey{}).(Metadata); ok {
		return meta
	}
	return Metadata{}
}

// TruncateRunes shortens s to at most max runes. Truncation is counted in runes
// rather than bytes because the VARCHAR widths of the audit columns count
// characters: cutting at a byte offset would both split a multi-byte character
// and leave the stored value shorter than the column allows.
func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	count := 0
	for i := range s {
		if count == max {
			return s[:i]
		}
		count++
	}
	return s
}

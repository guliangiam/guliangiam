package middleware

import (
	"net/http"

	platformhttp "github.com/guliangiam/guliangiam/internal/platform/http"
	"github.com/guliangiam/guliangiam/internal/platform/requestmeta"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RequestMetadataMiddleware stamps the observed request origin (client IP,
// user agent) together with the surface/request-id/trace-id already resolved by
// route.RequestMetadataMiddleware onto requestmeta for the application and
// audit layers.
//
// It must be mounted after route.RequestMetadataMiddleware: the surface and
// correlation ids are read from route's context, while the IP must come from
// platformhttp.ClientIP so the trusted-proxy verdict is applied (a raw
// X-Forwarded-For header is never trusted).
func RequestMetadataMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			meta := route.RequestMetadataFrom(r.Context())
			ctx := requestmeta.With(r.Context(), requestmeta.Metadata{
				IP: platformhttp.ClientIP(r),
				// records.user_agent / records.request_id are VARCHAR(255): an
				// over-long User-Agent or X-Request-Id would fail the audit INSERT
				// and roll back the business transaction, so the evidence is cut
				// here (the only place arbitrary client input enters the audit
				// trail). Truncation is by rune, never by byte.
				UserAgent: requestmeta.TruncateRunes(r.UserAgent(), requestmeta.MaxUserAgentLen),
				Surface:   string(meta.Surface),
				RequestID: requestmeta.TruncateRunes(meta.RequestID, requestmeta.MaxRequestIDLen),
				TraceID:   requestmeta.TruncateRunes(meta.TraceID, requestmeta.MaxTraceIDLen),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

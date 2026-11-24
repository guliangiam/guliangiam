package auditdenial

import (
	"net/http"
	"regexp"
)

// WithRecorderMiddleware injects the recorder into the request context.
func WithRecorderMiddleware(recorder Recorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(WithRecorder(r.Context(), recorder)))
		})
	}
}

// EventExtractor builds a denial event from the HTTP request. It is supplied by
// bootstrap so platform code does not import auth/transport helpers.
type EventExtractor func(*http.Request) (Event, bool)

// AuthorizationDenialMiddleware is the L2 fallback: only 403 responses are
// recorded here because a 404 cannot distinguish "missing" from "cross-tenant"
// without domain knowledge.
func AuthorizationDenialMiddleware(extract EventExtractor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)
			if extract == nil {
				return
			}
			// 403：授权拒绝。404 + 带 UUID 段的资源路径：无法区分「资源缺失」与
			// 「跨租户 by-ID」，按 S-10 要求同样记审计（reason 标注区分粒度），
			// 由去重窗口与独立限流抑制重复探测带来的量；带 UUID 段可避免把
			// 静态 404（例如未知路径）全部记入（DEF-13）。
			isForbidden := recorder.status == http.StatusForbidden
			isScopedNotFound := recorder.status == http.StatusNotFound && uuidSegment.MatchString(r.URL.Path)
			if !isForbidden && !isScopedNotFound {
				return
			}
			event, ok := extract(r)
			if !ok {
				return
			}
			event.Action = "authz.deny"
			event.ResourceType = "route"
			event.ResourceID = r.URL.Path
			if isForbidden {
				event.Reason = "permission_denied"
			} else {
				event.Reason = "not_found_or_foreign_tenant"
			}
			Record(r.Context(), event)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// uuidSegment 匹配路径中的 UUID 段（跨租户 by-ID 探测的形态）。
var uuidSegment = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

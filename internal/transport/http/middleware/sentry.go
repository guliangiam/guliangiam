package middleware

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

// SentryMiddleware returns a chi-compatible middleware that integrates with
// the Sentry Go SDK. It creates a Sentry hub for each request, captures
// panics, and reports 5xx responses as events.
func SentryMiddleware() func(http.Handler) http.Handler {
	return sentryhttp.New(sentryhttp.Options{
		Repanic:         false,
		WaitForDelivery: false,
	}).Handle
}

// SentrySetUserMiddleware extracts the user ID from the request context
// (set by the auth middleware) and attaches it to the current Sentry scope.
// Place this after the auth middleware in the chain.
func SentrySetUserMiddleware(contextKey any) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
				if uid, ok := r.Context().Value(contextKey).(string); ok && uid != "" {
					hub.Scope().SetUser(sentry.User{ID: uid})
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

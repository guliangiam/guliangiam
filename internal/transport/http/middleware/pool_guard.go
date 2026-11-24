package middleware

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func PoolGuardMiddleware(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Non-PostgreSQL drivers (MySQL mirror) have no pgxpool; the guard
			// protects only pool saturation, so it must pass through instead of
			// dereferencing a nil pool on every request.
			if pool == nil {
				next.ServeHTTP(w, r)
				return
			}
			stat := pool.Stat()
			if stat.AcquiredConns() >= stat.MaxConns() {
				slog.WarnContext(r.Context(), "connection pool exhausted",
					"acquired", stat.AcquiredConns(),
					"max", stat.MaxConns(),
					"idle", stat.IdleConns(),
					"total", stat.TotalConns(),
					"path", r.URL.Path,
					"method", r.Method,
				)
				w.Header().Set("Retry-After", "5")
				route.WriteError(w, r, apperrors.NewUnavailable("connection pool exhausted"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"net/http"
	"strconv"

	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// PaginationMiddleware rejects an out-of-range page/pageSize pair with 422 before
// the handler runs (手册 4.7.1).
//
// It is a middleware rather than a check inside each list handler because the
// bound has to hold for **every** endpoint, including the ones added later. Forty
// call sites each remembering to validate is exactly the shape of defect this
// round is about: the parameter is read, nobody checks it, and the mistake is
// invisible until a client pages wrongly.
//
// Only parameters that are actually **present** are validated. An absent page or
// pageSize is not an error, it is the documented default (1 and 20); rejecting it
// would break every client that just omits them. A present but unusable value is
// the caller's mistake and must not be silently replaced — the old
// `route.QueryInt` returned the fallback for `page=abc`, `page=0` and
// `page=-5` alike, so a client with a broken page counter silently received page
// one forever.
func PaginationMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()

			if raw := query.Get("page"); raw != "" {
				page, err := strconv.Atoi(raw)
				if err != nil || !platformpagination.ValidateQueryPage(page) {
					route.WriteError(w, r, platformpagination.NewInvalidQueryPageError(raw))
					return
				}
			}

			if raw := query.Get("pageSize"); raw != "" {
				pageSize, err := strconv.Atoi(raw)
				if err != nil || !platformpagination.ValidateQueryPageSize(pageSize) {
					route.WriteError(w, r, platformpagination.NewInvalidQueryPageSizeError(raw))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

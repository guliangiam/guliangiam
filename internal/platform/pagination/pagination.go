// Package pagination holds the single definition of this project's page/pageSize
// bounds (手册 4.7.1) and of the LIMIT/OFFSET arithmetic built from them.
//
// It exists as its own package because three layers need the same numbers and
// must not each invent them:
//
//   - the transport middleware rejects an out-of-range request with 422,
//   - the repository computes LIMIT/OFFSET for callers that never went through
//     HTTP (background jobs, exports, internal fan-out),
//   - the presenters echo back the size that was actually applied.
//
// The defect this replaces was `max(pageSize, 20)` written out in 43 repositories:
// that is a **floor**, not a ceiling, so `pageSize=5` returned 20 rows while the
// response said 5, and `pageSize=99999` went to the database untouched. The same
// expression also used `max(pageSize, 1)` for the offset, so for pageSize in
// [1,19] consecutive pages overlapped and rows appeared twice.
package pagination

import (
	"strconv"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DefaultPageSize is the page size used when the request omits pageSize
// (手册 4.7.1).
const DefaultPageSize = 20

// MaxPageSize is the largest page size this server serves. A request asking for
// more is rejected rather than silently truncated, because a caller that receives
// 100 rows believing it asked for 500 will page incorrectly for the rest of the
// run (手册 4.7.1: 「422，不得静默截断」).
const MaxPageSize = 100

// MaxPage is the largest page number this server serves. Page numbers had no
// upper bound: `Offset` computed (page-1)*pageSize unchecked, so `page=MaxInt`
// wrapped to a negative offset (and an in-memory list handler could slice out of
// range). The bound is deliberately far beyond any real result set — one million
// pages at MaxPageSize is just under 100 million rows — while keeping the
// multiplication inside a 32-bit int, so the arithmetic is safe on every build.
const MaxPage = 1_000_000

// ClampPageSize maps any int onto the served range: a non-positive value becomes
// DefaultPageSize, an oversized one becomes MaxPageSize, everything else passes
// through.
//
// It is the repository-side net for internal callers. An HTTP caller never
// reaches it with an invalid value: the middleware rejects the request first.
func ClampPageSize(pageSize int) int {
	if pageSize <= 0 {
		return DefaultPageSize
	}
	if pageSize > MaxPageSize {
		return MaxPageSize
	}
	return pageSize
}

// ClampPage maps any int onto 1..MaxPage. Like ClampPageSize it is the
// repository-side net for callers that never went through the HTTP middleware.
func ClampPage(page int) int {
	if page <= 0 {
		return 1
	}
	if page > MaxPage {
		return MaxPage
	}
	return page
}

// Offset returns the zero-based row offset of page, using the **same** page size
// that will be passed as LIMIT.
//
// Both operands are clamped before the multiplication: passing the clamped size
// in is the point (computing the offset from a different value than the limit is
// what made adjacent pages overlap), and clamping the page first is what keeps
// the product inside int instead of wrapping negative.
func Offset(page, pageSize int) int {
	return (ClampPage(page) - 1) * ClampPageSize(pageSize)
}

// ValidateQueryPage and ValidateQueryPageSize validate a value that the client
// actually sent in the query string: present means the value must be usable.
//
// A present `pageSize=0` is a mistake, not a request for the default, because the
// client had to type the zero to send it.
func ValidateQueryPage(page int) bool {
	return page >= 1 && page <= MaxPage
}

func ValidateQueryPageSize(pageSize int) bool {
	return pageSize >= 1 && pageSize <= MaxPageSize
}

// ValidateBodyPageParams validates pagination carried in a JSON request body
// (POST /api/v1/audit/records:search), where 0 means "the client omitted the
// field" rather than "the client asked for zero".
//
// The query-string validator cannot be reused here: Go's decoder cannot tell an
// absent number from an explicit zero, so treating 0 as invalid would reject
// every request that simply relies on the default. The asymmetry is in the wire
// format, not in the rule — both paths enforce 1..MaxPageSize on a value the
// client actually supplied.
func ValidateBodyPageParams(page, pageSize int) error {
	if page < 0 || page > MaxPage {
		return NewInvalidQueryPageError(strconv.Itoa(page))
	}
	if pageSize < 0 || pageSize > MaxPageSize {
		return NewInvalidQueryPageSizeError(strconv.Itoa(pageSize))
	}
	return nil
}

// NewInvalidQueryPageError builds the canonical 422 for a page value the caller
// supplied. The message names the bound and meta carries the offending value,
// so the frontend can point at the input that produced it (手册 4.14.1).
func NewInvalidQueryPageError(raw string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"page must be an integer between 1 and "+strconv.Itoa(MaxPage),
		map[string]any{"field": "page", "value": raw, "max": MaxPage},
	)
}

// NewInvalidQueryPageSizeError is the pageSize counterpart.
func NewInvalidQueryPageSizeError(raw string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"pageSize must be an integer between 1 and "+strconv.Itoa(MaxPageSize),
		map[string]any{"field": "pageSize", "value": raw, "max": MaxPageSize},
	)
}

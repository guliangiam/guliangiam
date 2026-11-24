package payments

import (
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// paginateList applies the served pagination bounds to a list the service
// already returned in full. An omitted pageSize is served as
// platformpagination.DefaultPageSize — never as "one page with everything" —
// and the response echoes the values actually applied, so `pageSize:0` can no
// longer surface in an empty list (手册 4.7.1).
func paginateList[T any](items []T, page, pageSize int) presenter.ListResponse[T] {
	total := len(items)
	page = platformpagination.ClampPage(page)
	pageSize = platformpagination.ClampPageSize(pageSize)
	// Compare before multiplying so a hostile page value cannot overflow into a
	// negative slice index.
	start := total
	if page <= total/pageSize+1 {
		start = (page - 1) * pageSize
	}
	if start > total {
		start = total
	}
	end := total
	if start < total && pageSize < total-start {
		end = start + pageSize
	}
	pageItems := items[start:end]
	if pageItems == nil {
		pageItems = make([]T, 0)
	}
	return presenter.NewListResponse(pageItems, page, pageSize, int64(total))
}

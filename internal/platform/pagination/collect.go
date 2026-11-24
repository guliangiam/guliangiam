package pagination

// CollectAll returns the whole set behind a paged repository call on behalf of
// an internal consumer (name catalogs, schedulers, SAML NameID lookup).
//
// It walks pages of MaxPageSize instead of asking for an oversized page:
// ClampPageSize silently caps a request at MaxPageSize, so the previous
// `PageSize: 1000` call returned 100 rows and the rest were dropped without an
// error. Asking for exactly the largest served page keeps the request legal and
// makes the number of round trips equal to the real page count.
//
// The walk terminates on an empty page or once the reported total has been
// collected, and page never exceeds MaxPage even if a repository keeps returning
// rows — the same ceiling HTTP callers are held to (手册 4.7.1).
func CollectAll[T any](fetch func(page, pageSize int) ([]T, int64, error)) ([]T, error) {
	return collectAll(fetch, MaxPage)
}

// collectAll is the bound seam: tests exercise a small page ceiling without
// walking a million pages.
func collectAll[T any](fetch func(page, pageSize int) ([]T, int64, error), maxPages int) ([]T, error) {
	all := make([]T, 0)
	for page := 1; page <= maxPages; page++ {
		batch, total, err := fetch(page, MaxPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) == 0 || int64(len(all)) >= total {
			break
		}
	}
	return all, nil
}

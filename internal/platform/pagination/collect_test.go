package pagination

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pagedFetch models a repository that honors page/pageSize: it slices the full
// set and reports the unpaged total, like the postgres repositories do.
func pagedFetch(items []int, calls *int) func(page, pageSize int) ([]int, int64, error) {
	return func(page, pageSize int) ([]int, int64, error) {
		(*calls)++
		offset := Offset(page, pageSize)
		if offset >= len(items) {
			return nil, int64(len(items)), nil
		}
		end := offset + ClampPageSize(pageSize)
		if end > len(items) {
			end = len(items)
		}
		return items[offset:end], int64(len(items)), nil
	}
}

func TestCollectAll_WalksEveryPage(t *testing.T) {
	items := make([]int, 2*MaxPageSize+5)
	for i := range items {
		items[i] = i
	}
	calls := 0

	got, err := CollectAll(pagedFetch(items, &calls))
	require.NoError(t, err)
	assert.Equal(t, items, got)
	assert.Equal(t, 3, calls, "two full pages and the short page that reaches the total")
}

func TestCollectAll_UsesMaxPageSize(t *testing.T) {
	var gotPageSize int
	_, err := CollectAll(func(_ int, pageSize int) ([]int, int64, error) {
		gotPageSize = pageSize
		return []int{1}, 1, nil
	})
	require.NoError(t, err)
	assert.Equal(t, MaxPageSize, gotPageSize)
}

func TestCollectAll_StopsWhenTotalReached(t *testing.T) {
	var pages []int
	got, err := CollectAll(func(page, _ int) ([]int, int64, error) {
		pages = append(pages, page)
		return []int{page}, 2, nil
	})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, got)
	assert.Equal(t, []int{1, 2}, pages)
}

func TestCollectAll_PropagatesError(t *testing.T) {
	stop := assert.AnError
	got, err := CollectAll(func(page, _ int) ([]int, int64, error) {
		if page == 2 {
			return nil, 0, stop
		}
		return []int{page}, math.MaxInt64, nil
	})
	require.ErrorIs(t, err, stop)
	assert.Nil(t, got)
}

// A repository that ignores the page argument must not make the walk run
// forever; the page ceiling ends it.
func TestCollectAll_HasUpperBound(t *testing.T) {
	calls := 0
	got, err := collectAll(func(int, int) ([]int, int64, error) {
		calls++
		return []int{calls}, math.MaxInt64, nil
	}, 3)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, got)
	assert.Equal(t, 3, calls)
}

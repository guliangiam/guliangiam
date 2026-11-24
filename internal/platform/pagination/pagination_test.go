package pagination

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// A hostile page must not wrap (page-1)*pageSize into a negative offset. Both
// operands are clamped before the multiplication, so the result is the deepest
// served offset, not an overflowed int.
func TestOffset_ClampsHostilePageBeforeMultiplying(t *testing.T) {
	want := (MaxPage - 1) * MaxPageSize
	for _, page := range []int{math.MaxInt, MaxPage + 1, MaxPage} {
		assert.Equal(t, want, Offset(page, math.MaxInt), "page=%d", page)
	}
	assert.GreaterOrEqual(t, want, 0, "the deepest offset must stay non-negative")
}

// The normal arithmetic callers rely on is unchanged: same page, same size, same
// offset as LIMIT.
func TestOffset_NormalPages(t *testing.T) {
	cases := []struct {
		page, pageSize, want int
	}{
		{1, 20, 0},
		{2, 20, 20},
		{3, 5, 10},
		{3, 100, 200},
		{0, 20, 0},
		{-5, -1, 0},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, Offset(tc.page, tc.pageSize), "Offset(%d, %d)", tc.page, tc.pageSize)
	}
}

func TestClampPage_Bounds(t *testing.T) {
	cases := map[int]int{
		math.MinInt: 1,
		0:           1,
		1:           1,
		MaxPage:     MaxPage,
		MaxPage + 1: MaxPage,
		math.MaxInt: MaxPage,
	}
	for page, want := range cases {
		assert.Equal(t, want, ClampPage(page), "ClampPage(%d)", page)
	}
}

func TestValidateQueryPage_UpperBound(t *testing.T) {
	assert.True(t, ValidateQueryPage(1))
	assert.True(t, ValidateQueryPage(MaxPage))
	assert.False(t, ValidateQueryPage(MaxPage+1))
	assert.False(t, ValidateQueryPage(0))
	assert.False(t, ValidateQueryPage(-1))
}

// Body pagination keeps its documented 0-means-omitted asymmetry but must apply
// the same page ceiling as the query-string path.
func TestValidateBodyPageParams_PageUpperBound(t *testing.T) {
	require.NoError(t, ValidateBodyPageParams(0, 0))
	require.NoError(t, ValidateBodyPageParams(MaxPage, MaxPageSize))

	err := ValidateBodyPageParams(MaxPage+1, MaxPageSize)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, MaxPage, appErr.Meta["max"])
	assert.Equal(t, strconv.Itoa(MaxPage+1), appErr.Meta["value"])
}

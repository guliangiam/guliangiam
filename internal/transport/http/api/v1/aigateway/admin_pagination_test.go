package aigateway

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type listBody struct {
	Items []int `json:"items"`
	Page  struct {
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Total    int64 `json:"total"`
	} `json:"page"`
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func runWriteList(t *testing.T, target string, items []int) (*httptest.ResponseRecorder, listBody) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	writeList(recorder, request, items)

	var body listBody
	if recorder.Body.Len() > 0 {
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	}
	return recorder, body
}

// Normal paging is untouched: same success envelope, same items, same meta.
func TestWriteList_NormalPaginationUnchanged(t *testing.T) {
	items := []int{0, 1, 2, 3, 4}
	recorder, body := runWriteList(t, "/api/v1/ai/providers?page=2&pageSize=2", items)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []int{2, 3}, body.Items)
	assert.Equal(t, 2, body.Page.Page)
	assert.Equal(t, 2, body.Page.PageSize)
	assert.Equal(t, int64(5), body.Page.Total)
}

// An absent pageSize is served as DefaultPageSize, not as the whole set: the
// previous default made every management list an unbounded response.
func TestWriteList_AbsentPageSizeServesDefaultPage(t *testing.T) {
	items := make([]int, 2*platformpagination.MaxPageSize+1)
	for i := range items {
		items[i] = i
	}
	recorder, body := runWriteList(t, "/api/v1/ai/providers", items)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, items[:platformpagination.DefaultPageSize], body.Items)
	assert.Equal(t, 1, body.Page.Page)
	assert.Equal(t, platformpagination.DefaultPageSize, body.Page.PageSize)
	assert.Equal(t, int64(len(items)), body.Page.Total)
}

// The row beyond the first two pages must still be addressable: page 3 at the
// maximum served page size returns the 201st item.
func TestWriteList_ThirdPageReturnsItem201(t *testing.T) {
	items := make([]int, 2*platformpagination.MaxPageSize+1)
	for i := range items {
		items[i] = i
	}
	recorder, body := runWriteList(t, "/api/v1/ai/providers?page=3&pageSize=100", items)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []int{2 * platformpagination.MaxPageSize}, body.Items)
	assert.Equal(t, 3, body.Page.Page)
	assert.Equal(t, platformpagination.MaxPageSize, body.Page.PageSize)
	assert.Equal(t, int64(len(items)), body.Page.Total)
}

// An empty list must echo the served default instead of pageSize:1.
func TestWriteList_EmptyListEchoesDefaultPageSize(t *testing.T) {
	recorder, body := runWriteList(t, "/api/v1/ai/providers", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, body.Items)
	assert.Equal(t, 1, body.Page.Page)
	assert.Equal(t, platformpagination.DefaultPageSize, body.Page.PageSize)
	assert.Equal(t, int64(0), body.Page.Total)
}

// A page beyond MaxPage is rejected as 422 instead of wrapping the offset.
func TestWriteList_RejectsOversizedPage(t *testing.T) {
	items := []int{0, 1, 2, 3, 4}
	for _, page := range []int{platformpagination.MaxPage + 1, math.MaxInt} {
		recorder, body := runWriteList(t, "/api/v1/ai/providers?page="+itoaInt(page)+"&pageSize=100", items)
		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code, "page=%d", page)
		assert.Equal(t, "unprocessable_entity", body.Error.Code)
	}
}

// An explicit pageSize over the ceiling is rejected, never silently truncated.
func TestWriteList_RejectsOversizedPageSize(t *testing.T) {
	items := []int{0, 1, 2}
	recorder, body := runWriteList(t, "/api/v1/ai/providers?pageSize=101", items)

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Equal(t, "unprocessable_entity", body.Error.Code)
}

// The deepest accepted page is still safe: it clamps to the end of the list
// instead of indexing out of range.
func TestWriteList_MaxPageIsSafe(t *testing.T) {
	items := []int{0, 1, 2}
	recorder, body := runWriteList(t,
		"/api/v1/ai/providers?page="+itoaInt(platformpagination.MaxPage)+"&pageSize=100", items)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, body.Items)
	assert.Equal(t, platformpagination.MaxPage, body.Page.Page)
	assert.Equal(t, 100, body.Page.PageSize)
}

func itoaInt(value int) string {
	return strconv.Itoa(value)
}

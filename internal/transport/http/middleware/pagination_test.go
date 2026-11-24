package middleware

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// nextReached records that the chain let the request through, so a test can tell
// "rejected with 422" from "rejected for some other reason".
type nextReached struct{ called bool }

func (n *nextReached) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n.called = true
		route.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
}

func runPagination(t *testing.T, target string) (*httptest.ResponseRecorder, *nextReached) {
	t.Helper()
	next := &nextReached{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	PaginationMiddleware()(next.handler()).ServeHTTP(recorder, request)
	return recorder, next
}

// An absent parameter is not an error: it is the documented default, and
// rejecting it would break every client that simply omits it (手册 4.7.1).
func TestPaginationMiddleware_AbsentParametersPass(t *testing.T) {
	cases := []string{
		"/api/v1/users",
		"/api/v1/users?keyword=x",
		"/api/v1/users?page=1",
		"/api/v1/users?pageSize=20",
	}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			recorder, next := runPagination(t, target)
			assert.True(t, next.called)
			assert.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}

// The boundary is inclusive on both ends: 1 and MaxPageSize are valid.
func TestPaginationMiddleware_AcceptsTheRange(t *testing.T) {
	cases := []string{
		"/api/v1/users?page=1&pageSize=1",
		"/api/v1/users?page=2&pageSize=20",
		"/api/v1/users?page=999&pageSize=100",
	}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			recorder, next := runPagination(t, target)
			assert.True(t, next.called, "an in-range request must reach the handler")
			assert.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}

// Everything below is a caller mistake and must be answered, not silently
// replaced. The old route.QueryInt returned the fallback for `page=abc`,
// `page=0` and `page=-5` alike, so a client with a broken page counter received
// page one forever and never learned why.
func TestPaginationMiddleware_RejectsInvalidValues(t *testing.T) {
	cases := map[string]struct {
		target string
		field  string
	}{
		"page is not a number":          {"/api/v1/users?page=abc", "page"},
		"page is zero":                  {"/api/v1/users?page=0", "page"},
		"page is negative":              {"/api/v1/users?page=-5", "page"},
		"page is fractional":            {"/api/v1/users?page=1.5", "page"},
		"pageSize is not a number":      {"/api/v1/users?pageSize=abc", "pageSize"},
		"pageSize is zero":              {"/api/v1/users?pageSize=0", "pageSize"},
		"pageSize is negative":          {"/api/v1/users?pageSize=-20", "pageSize"},
		"pageSize is one over the max":  {"/api/v1/users?pageSize=101", "pageSize"},
		"pageSize is far over the max":  {"/api/v1/users?pageSize=99999", "pageSize"},
		"pageSize is fractional":        {"/api/v1/users?pageSize=2.5", "pageSize"},
		"a later bad value still fails": {"/api/v1/users?page=2&pageSize=1000", "pageSize"},
		"page is one over the max":      {"/api/v1/users?page=" + itoa(platformpagination.MaxPage+1), "page"},
		"page is MaxInt":                {"/api/v1/users?page=" + itoa(math.MaxInt), "page"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			recorder, next := runPagination(t, tc.target)

			assert.False(t, next.called, "an invalid request must not reach the handler")
			assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code,
				"422 is the documented code for a semantically invalid request (手册 4.14.1)")

			body := map[string]any{}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))

			errObj, ok := body["error"].(map[string]any)
			require.True(t, ok, "the management face must answer with the application envelope: %v", body)
			// The message must name the bound so a caller can fix the request
			// without reading the source.
			message, _ := errObj["message"].(string)
			assert.Contains(t, message, tc.field)

			// And the offending field must be machine-readable, so the frontend
			// can point at the input that produced it.
			meta, _ := errObj["meta"].(map[string]any)
			require.NotNil(t, meta, "the error must carry the offending field in details")
			assert.Equal(t, tc.field, meta["field"])
		})
	}
}

// A pageSize exactly at the maximum is the boundary the manual names; an
// off-by-one in the comparison would reject a request the contract allows.
func TestPaginationMiddleware_MaxPageSizeBoundary(t *testing.T) {
	_, next := runPagination(t, "/api/v1/users?pageSize="+itoa(platformpagination.MaxPageSize))
	assert.True(t, next.called)

	recorder, next := runPagination(t, "/api/v1/users?pageSize="+itoa(platformpagination.MaxPageSize+1))
	assert.False(t, next.called)
	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
}

// The page ceiling is inclusive exactly like the pageSize ceiling.
func TestPaginationMiddleware_MaxPageBoundary(t *testing.T) {
	_, next := runPagination(t, "/api/v1/users?page="+itoa(platformpagination.MaxPage))
	assert.True(t, next.called)

	recorder, next := runPagination(t, "/api/v1/users?page="+itoa(platformpagination.MaxPage+1))
	assert.False(t, next.called)
	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok)
	meta, _ := errObj["meta"].(map[string]any)
	assert.Equal(t, "page", meta["field"])
	assert.Equal(t, float64(platformpagination.MaxPage), meta["max"])
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

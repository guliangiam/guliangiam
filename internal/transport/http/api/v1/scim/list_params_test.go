package scim

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BE-E-07: non-numeric or negative startIndex/count must yield 400, not a
// silent default.
func TestParseScimListParams(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantStart int
		wantCount int
		wantOK    bool
		wantCode  int
	}{
		{"absent", "", 0, 0, true, http.StatusOK},
		{"valid", "startIndex=2&count=10", 2, 10, true, http.StatusOK},
		{"nonnumeric-start", "startIndex=abc", 0, 0, false, http.StatusBadRequest},
		{"nonnumeric-count", "count=1.5", 0, 0, false, http.StatusBadRequest},
		{"negative-start", "startIndex=-1", 0, 0, false, http.StatusBadRequest},
		{"negative-count", "count=-5", 0, 0, false, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?"+tc.query, http.NoBody)
			rec := httptest.NewRecorder()
			start, count, ok := parseScimListParams(rec, req)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantCount, count)
			if !tc.wantOK {
				assert.Equal(t, tc.wantCode, rec.Code)
			}
		})
	}
}

package entries

import (
	"net/http"
	"testing"
)

// The transcript read face may contain live credentials that were ingested from
// a running system, so the response must never be reusable: not by a browser
// back/forward cache, not by an intermediary. The header has to be declared on
// the way in, before the service is consulted, so that it is present even when
// the read fails (42.3.13 条 5).
func TestTranscriptRoute_DeclaresNoStoreOnEveryOutcome(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	for _, tc := range []struct {
		name  string
		query string
	}{
		{"no arguments", ""},
		{"with paging arguments", "limit=10&cursor=abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequestWithQuery(t, router, http.MethodGet,
				"/api/v1/entries/entry-1/transcript", tc.query, nil)
			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
		})
	}
}

func TestTranscriptRoute_IsReadOnly(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			resp := doRequestWithQuery(t, router, method, "/api/v1/entries/entry-1/transcript", "", nil)
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestTranscriptRoute_IsNotShadowedByEntrySubresourceRoutes(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequestWithQuery(t, router, http.MethodGet, "/api/v1/entries/entry-1/transcript", "", nil)
	if resp.StatusCode == http.StatusNotFound {
		t.Fatal("transcript route is not registered")
	}
}

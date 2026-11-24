package mcpservers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsMcpEndpoint_ProbeStatus(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		wantTrue bool
	}{
		{name: "ok", status: http.StatusOK, wantTrue: true},
		{name: "not_found", status: http.StatusNotFound, wantTrue: true},
		{name: "server_error", status: http.StatusInternalServerError, wantTrue: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/mcp" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			if got := IsMcpEndpoint(context.Background(), srv.URL); got != tc.wantTrue {
				t.Fatalf("IsMcpEndpoint = %v, want %v (status %d)", got, tc.wantTrue, tc.status)
			}
		})
	}
}

func TestIsMcpEndpoint_CancelledContextReturnsFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if IsMcpEndpoint(ctx, srv.URL) {
		t.Fatal("cancelled context must abort the probe instead of reporting a hit")
	}
}

func TestIsMcpEndpoint_RejectsInvalidURL(t *testing.T) {
	cases := []string{"", "ftp://example.com", "not-a-url"}
	for _, raw := range cases {
		if IsMcpEndpoint(context.Background(), raw) {
			t.Fatalf("IsMcpEndpoint(%q) = true, want false", raw)
		}
	}
}

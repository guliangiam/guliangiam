//go:build integration

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// newLoopbackProxy returns a Proxy whose SSRF transport explicitly allows the
// loopback addresses httptest.NewServer binds to. NewProxy() installs the
// default SafeHTTPClient, which blocks 127.0.0.1 by design.
func newLoopbackProxy() *Proxy {
	p := NewProxy()
	p.SetHTTPClient(&http.Client{
		Transport: ssrf.SafeTransportWithAllowlist(5*time.Second, []string{"127.0.0.1/32", "::1/128"}),
	})
	return p
}

func TestProxy_SyncToolsFromServer_WithHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tools":[{"name":"tool1","description":"test tool"}]}`))
	}))
	defer server.Close()

	p := newLoopbackProxy()
	cfg := &MCPServerConfig{
		ID:       "srv1",
		Endpoint: server.URL,
	}
	p.RegisterServer(cfg)

	err := p.SyncToolsFromServer(context.Background(), "srv1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	servers := p.GetServers()
	if len(servers) != 1 || len(servers[0].Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(servers[0].Tools))
	}
}

func TestProxy_CallTool_WithHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	p := newLoopbackProxy()
	p.RegisterServer(&MCPServerConfig{
		ID:          "srv1",
		Endpoint:    server.URL,
		AccessToken: "test-token",
	})

	result, err := p.CallTool(context.Background(), "srv1", "tool1", map[string]interface{}{"arg": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

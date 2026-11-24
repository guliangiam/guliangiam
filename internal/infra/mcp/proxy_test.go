package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewProxy(t *testing.T) {
	p := NewProxy()
	if p == nil {
		t.Fatal("expected non-nil proxy")
	}
	servers := p.GetServers()
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

func TestNewProxyWithTokenURL(t *testing.T) {
	p := NewProxyWithTokenURL("https://oauth.example.com/token")
	if p == nil {
		t.Fatal("expected non-nil proxy")
	}
	if p.tokenURL != "https://oauth.example.com/token" {
		t.Errorf("unexpected tokenURL: %s", p.tokenURL)
	}
}

func TestProxy_RegisterAndRemoveServer(t *testing.T) {
	p := NewProxy()
	cfg := &MCPServerConfig{
		ID:       "srv1",
		Name:     "Test Server",
		Endpoint: "http://localhost:8080",
	}
	p.RegisterServer(cfg)
	servers := p.GetServers()
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].ID != "srv1" {
		t.Errorf("expected srv1, got %s", servers[0].ID)
	}

	p.RemoveServer("srv1")
	servers = p.GetServers()
	if len(servers) != 0 {
		t.Errorf("expected 0 servers after remove, got %d", len(servers))
	}
}

func TestProxy_GetServers_Empty(t *testing.T) {
	p := NewProxy()
	servers := p.GetServers()
	if servers == nil {
		t.Error("expected non-nil slice")
	}
	if len(servers) != 0 {
		t.Errorf("expected 0, got %d", len(servers))
	}
}

func TestProxy_RegisterServer_Overwrite(t *testing.T) {
	p := NewProxy()
	p.RegisterServer(&MCPServerConfig{ID: "srv1", Name: "v1"})
	p.RegisterServer(&MCPServerConfig{ID: "srv1", Name: "v2"})
	servers := p.GetServers()
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Name != "v2" {
		t.Errorf("expected v2, got %s", servers[0].Name)
	}
}

func TestProxy_SyncToolsFromServer_NotFound(t *testing.T) {
	p := NewProxy()
	err := p.SyncToolsFromServer(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestProxy_SyncAllTools_Empty(t *testing.T) {
	p := NewProxy()
	err := p.SyncAllTools(context.Background())
	if err != nil {
		t.Errorf("expected no error for empty proxy, got %v", err)
	}
}

func TestProxy_ListTools_Cached(t *testing.T) {
	p := NewProxy()
	cfg := &MCPServerConfig{
		ID:       "srv1",
		Endpoint: "http://localhost:9999",
		Tools:    []Tool{{Name: "cached-tool", Description: "from cache"}},
	}
	p.RegisterServer(cfg)

	tools, err := p.ListTools(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "cached-tool" {
		t.Errorf("expected cached-tool, got %s", tools[0].Name)
	}
}

func TestProxy_CallTool_NotFound(t *testing.T) {
	p := NewProxy()
	_, err := p.CallTool(context.Background(), "nonexistent", "tool1", nil)
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestProxy_serverWithToken(t *testing.T) {
	p := NewProxy()
	orig := &MCPServerConfig{
		ID:          "srv1",
		AccessToken: "old-token",
	}
	cp := p.serverWithToken(orig, "new-token")
	if cp.AccessToken != "new-token" {
		t.Errorf("expected new-token, got %s", cp.AccessToken)
	}
	if orig.AccessToken != "old-token" {
		t.Error("original should not be modified")
	}
}

func TestProxy_getAccessToken_Existing(t *testing.T) {
	p := NewProxy()
	server := &MCPServerConfig{
		ID:          "srv1",
		AccessToken: "existing-token",
	}
	token, err := p.getAccessToken(context.Background(), server)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "existing-token" {
		t.Errorf("expected existing-token, got %s", token)
	}
}

func TestProxy_getAccessToken_NoOAuthConfig(t *testing.T) {
	p := NewProxy()
	server := &MCPServerConfig{ID: "srv1"}
	token, err := p.getAccessToken(context.Background(), server)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "" {
		t.Errorf("expected empty token, got %s", token)
	}
}

// ── CallTool 错误处理增强测试 ─────────────────────────────────────
//
// 这些测试使用 httptest.NewServer，它绑定到 127.0.0.1，会被 SSRF SafeTransport
// 拦截。所以每个测试都注入 http.DefaultClient 来绕过 SSRF——ValidateURL 本身
// 不会阻断 localhost/127.0.0.1，只有 SafeTransport 的 dial 层会。

func TestProxy_CallTool_HTTP500_ActionableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"model overloaded"}`))
	}))
	defer srv.Close()

	p := NewProxy()
	p.SetHTTPClient(http.DefaultClient) // bypass SSRF for httptest
	p.SetHTTPClient(http.DefaultClient) // bypass SSRF for httptest
	p.RegisterServer(&MCPServerConfig{ID: "srv1", Endpoint: srv.URL})

	_, err := p.CallTool(context.Background(), "srv1", "search", map[string]interface{}{"q": "x"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	got := err.Error()
	if !strings.Contains(got, "upstream returned 500") {
		t.Errorf("expected status in error, got: %s", got)
	}
	if !strings.Contains(got, "model overloaded") {
		t.Errorf("expected response body in error, got: %s", got)
	}
	if !strings.Contains(got, "srv1/search") {
		t.Errorf("expected server/tool path in error, got: %s", got)
	}
}

func TestProxy_CallTool_HTTP404_ActionableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`not found`))
	}))
	defer srv.Close()

	p := NewProxy()
	p.SetHTTPClient(http.DefaultClient) // bypass SSRF for httptest
	p.RegisterServer(&MCPServerConfig{ID: "srv2", Endpoint: srv.URL})

	_, err := p.CallTool(context.Background(), "srv2", "tool", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "upstream returned 404") {
		t.Errorf("expected 404 status, got: %s", err.Error())
	}
}

func TestProxy_CallTool_MCPErrorShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":{"code":-32601,"message":"tool not found"}}`))
	}))
	defer srv.Close()

	p := NewProxy()
	p.SetHTTPClient(http.DefaultClient) // bypass SSRF for httptest
	p.RegisterServer(&MCPServerConfig{ID: "srv3", Endpoint: srv.URL})

	_, err := p.CallTool(context.Background(), "srv3", "ghost", nil)
	if err == nil {
		t.Fatal("expected error for MCP error shape")
	}
	got := err.Error()
	if !strings.Contains(got, "code=-32601") {
		t.Errorf("expected error code in message, got: %s", got)
	}
	if !strings.Contains(got, "tool not found") {
		t.Errorf("expected error message in message, got: %s", got)
	}
}

func TestProxy_CallTool_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`this is not json`))
	}))
	defer srv.Close()

	p := NewProxy()
	p.SetHTTPClient(http.DefaultClient) // bypass SSRF for httptest
	p.RegisterServer(&MCPServerConfig{ID: "srv4", Endpoint: srv.URL})

	_, err := p.CallTool(context.Background(), "srv4", "tool", nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
	// error should contain the body snippet
	if !strings.Contains(err.Error(), "this is not json") {
		t.Errorf("expected body snippet in error, got: %s", err.Error())
	}
}

func TestProxy_CallTool_AcceptHeaderSent(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	p := NewProxy()
	p.SetHTTPClient(http.DefaultClient) // bypass SSRF for httptest
	p.RegisterServer(&MCPServerConfig{ID: "srv5", Endpoint: srv.URL})

	_, err := p.CallTool(context.Background(), "srv5", "tool", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAccept == "" {
		t.Error("Accept header should be sent")
	}
}

func TestProxy_CallTool_BodyTruncatedInError(t *testing.T) {
	longBody := strings.Repeat("x", 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(longBody))
	}))
	defer srv.Close()

	p := NewProxy()
	p.SetHTTPClient(http.DefaultClient) // bypass SSRF for httptest
	p.RegisterServer(&MCPServerConfig{ID: "srv6", Endpoint: srv.URL})

	_, err := p.CallTool(context.Background(), "srv6", "tool", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	got := err.Error()
	if !strings.Contains(got, "...") {
		t.Errorf("expected truncation marker in error, got: %s", got)
	}
}

func TestProxy_CallTool_NetworkError_Context(t *testing.T) {
	p := NewProxy()
	// Register with unreachable endpoint — no httptest server
	p.RegisterServer(&MCPServerConfig{ID: "srv7", Endpoint: "http://127.0.0.1:19999"})

	_, err := p.CallTool(context.Background(), "srv7", "tool", nil)
	if err == nil {
		t.Fatal("expected network error")
	}
	got := err.Error()
	if !strings.Contains(got, "srv7") || !strings.Contains(got, "tool") {
		t.Errorf("expected server/tool context in network error, got: %s", got)
	}
}

// ── truncateBody / extractMCPError helpers ──────────────────────────

func TestTruncateBody(t *testing.T) {
	if truncateBody([]byte("short"), 100) != "short" {
		t.Error("should return full body when shorter than limit")
	}
	long := strings.Repeat("a", 500)
	got := truncateBody([]byte(long), 256)
	if len(got) != 259 { // 256 + 3 for "..."
		t.Errorf("expected 259, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("should end with ...")
	}
}

func TestExtractMCPError(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantOK  bool
		wantMsg string
	}{
		{
			name:    "standard MCP error",
			input:   map[string]interface{}{"error": map[string]interface{}{"code": -32600, "message": "invalid request"}},
			wantOK:  true,
			wantMsg: "invalid request",
		},
		{
			name:   "no error key",
			input:  map[string]interface{}{"content": "hello"},
			wantOK: false,
		},
		{
			name:   "error key but non-object value",
			input:  map[string]interface{}{"error": "string-error"},
			wantOK: false,
		},
		{
			name:   "nil input",
			input:  nil,
			wantOK: false,
		},
		{
			name:   "array input",
			input:  []interface{}{"a", "b"},
			wantOK: false,
		},
		{
			name:   "error object with neither code nor message",
			input:  map[string]interface{}{"error": map[string]interface{}{}},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractMCPError(tt.input)
			if ok != tt.wantOK {
				t.Errorf("extractMCPError() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if tt.wantMsg != "" && !strings.Contains(got, tt.wantMsg) {
				t.Errorf("extractMCPError() got = %q, want containing %q", got, tt.wantMsg)
			}
		})
	}
}

// sanity: errors 包需要至少一次引用
var _ = errors.New

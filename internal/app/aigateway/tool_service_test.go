package aigateway

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

// ── Test ToolExecutor (fakes) ────────────────────────────────────────

type fakeToolExecutor struct {
	results map[string]any // tool name → result
	errs    map[string]error
	calls   []string // tool names called
	servers []string // mcp server names called (for binding verification)
}

func (f *fakeToolExecutor) ExecuteTool(_ context.Context, binding *domain.MCPBinding, toolName string, _ map[string]any) (any, error) {
	f.calls = append(f.calls, toolName)
	if binding != nil {
		f.servers = append(f.servers, binding.ServerName)
	} else {
		f.servers = append(f.servers, "<nil>")
	}
	if err, ok := f.errs[toolName]; ok {
		return nil, err
	}
	if r, ok := f.results[toolName]; ok {
		return r, nil
	}
	return "default-result", nil
}

// toolBindings for tests
var _ = []domain.CanonicalTool{
	{
		Name:        "get_weather",
		Description: "Get current weather",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
		MCPTarget:   &domain.MCPBinding{ServerName: "weather-station", ToolName: "get_weather", OrgSlug: "demo"},
	},
	{
		Name:        "get_time",
		Description: "Get current time",
		MCPTarget:   &domain.MCPBinding{ServerName: "clock-server", ToolName: "get_time", OrgSlug: "demo"},
	},
	{
		Name:        "broken_tool",
		Description: "Tool that fails",
		MCPTarget:   &domain.MCPBinding{ServerName: "bad-server", ToolName: "broken_tool", OrgSlug: "demo"},
	},
	// "unknown_tool" — no MCPTarget → should be skipped
}

// ── Unit tests for ExecuteTools + HasToolUseBlocks ────────────────────

func TestHasToolUseBlocks_Detects(t *testing.T) {
	content := []domain.CanonicalContentBlock{
		{Type: domain.ContentBlockText, Text: "hello"},
		{Type: domain.ContentBlockToolUse, ToolName: "get_weather", ToolCallID: "tc-1"},
	}
	if !HasToolUseBlocks(content) {
		t.Error("expected to detect tool_use block")
	}
}

func TestHasToolUseBlocks_NoToolUse(t *testing.T) {
	content := []domain.CanonicalContentBlock{
		{Type: domain.ContentBlockText, Text: "hello"},
	}
	if HasToolUseBlocks(content) {
		t.Error("should be false when no tool_use blocks")
	}
}

func TestExecuteTools_NilExecutor_Passthrough(t *testing.T) {
	content := []domain.CanonicalContentBlock{
		{Type: domain.ContentBlockToolUse, ToolName: "foo", ToolCallID: "tc-1"},
	}
	out := ExecuteTools(context.Background(), nil, content, nil, "", "")
	if len(out) != 1 {
		t.Fatalf("expected 1 block, got %d", len(out))
	}
	if out[0].Type != domain.ContentBlockToolUse {
		t.Errorf("type should remain tool_use when exec is nil, got %s", out[0].Type)
	}
}

func TestExecuteTools_EmptyContent_Passthrough(t *testing.T) {
	exec := &fakeToolExecutor{results: map[string]any{"foo": "bar"}}
	out := ExecuteTools(context.Background(), exec, nil, nil, "", "")
	if out != nil {
		t.Errorf("nil content should stay nil, got %v", out)
	}
}

func TestExecuteTools_ExecutesWithMCPBinding(t *testing.T) {
	exec := &fakeToolExecutor{
		results: map[string]any{
			"get_weather": map[string]any{"temp": 22, "condition": "sunny"},
		},
	}
	bindings := []domain.CanonicalTool{
		{Name: "get_weather", MCPTarget: &domain.MCPBinding{ServerName: "weather-station", ToolName: "get_weather"}},
	}
	content := []domain.CanonicalContentBlock{
		{Type: domain.ContentBlockText, Text: "What's the weather?"},
		{Type: domain.ContentBlockToolUse, ToolName: "get_weather", ToolCallID: "tc-1",
			ToolInput: map[string]any{"city": "Beijing"}},
		{Type: domain.ContentBlockText, Text: "Thanks"},
	}

	out := ExecuteTools(context.Background(), exec, content, bindings, "", "")

	if len(out) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(out))
	}
	// text passes through
	if out[0].Type != domain.ContentBlockText || out[0].Text != "What's the weather?" {
		t.Errorf("block 0 text mismatch: %+v", out[0])
	}
	// tool_use → tool_result
	if out[1].Type != domain.ContentBlockToolResult {
		t.Fatalf("block 1 should be tool_result, got type=%s", out[1].Type)
	}
	if out[1].ToolCallID != "tc-1" {
		t.Errorf("ToolCallID preserved? got %q", out[1].ToolCallID)
	}
	if out[1].ToolName != "get_weather" {
		t.Errorf("ToolName preserved? got %q", out[1].ToolName)
	}
	wr, ok := out[1].ToolResult.(map[string]any)
	if !ok {
		t.Fatalf("ToolResult type: %T", out[1].ToolResult)
	}
	if wr["temp"].(int) != 22 {
		t.Errorf("temp: %v", wr["temp"])
	}
	// text passes through
	if out[2].Type != domain.ContentBlockText || out[2].Text != "Thanks" {
		t.Errorf("block 2: %+v", out[2])
	}

	// Verify executor was called with correct binding (ServerName)
	if len(exec.calls) != 1 || exec.calls[0] != "get_weather" {
		t.Errorf("calls = %v", exec.calls)
	}
	if len(exec.servers) != 1 || exec.servers[0] != "weather-station" {
		t.Errorf("servers = %v (expected weather-station)", exec.servers)
	}
}

func TestExecuteTools_NoMCPBinding_Skips(t *testing.T) {
	// tool_use with no matching CanonicalTool.MCPTarget → keep tool_use block
	exec := &fakeToolExecutor{}
	// bindings has "get_weather" but content has "unknown_tool_use" → no match
	bindings := []domain.CanonicalTool{
		{Name: "get_weather", MCPTarget: &domain.MCPBinding{ServerName: "weather"}},
	}
	content := []domain.CanonicalContentBlock{
		{Type: domain.ContentBlockToolUse, ToolName: "unknown_tool_use", ToolCallID: "tc-unknown"},
	}

	out := ExecuteTools(context.Background(), exec, content, bindings, "", "")
	if len(out) != 1 {
		t.Fatalf("expected 1 block, got %d", len(out))
	}
	// No binding found → tool_use preserved as-is
	if out[0].Type != domain.ContentBlockToolUse {
		t.Errorf("tool_use without binding should be preserved, got type=%s", out[0].Type)
	}
	if len(exec.calls) != 0 {
		t.Errorf("executor should NOT have been called, calls=%v", exec.calls)
	}
}

func TestExecuteTools_ErrorProducesToolResultWithError(t *testing.T) {
	exec := &fakeToolExecutor{
		errs: map[string]error{
			"broken_tool": context.Canceled,
		},
	}
	bindings := []domain.CanonicalTool{
		{Name: "broken_tool", MCPTarget: &domain.MCPBinding{ServerName: "bad-server"}},
	}
	content := []domain.CanonicalContentBlock{
		{Type: domain.ContentBlockToolUse, ToolName: "broken_tool", ToolCallID: "tc-err"},
	}

	out := ExecuteTools(context.Background(), exec, content, bindings, "", "")
	if len(out) != 1 {
		t.Fatalf("expected 1 block, got %d", len(out))
	}
	if out[0].Type != domain.ContentBlockToolResult {
		t.Fatalf("block should be tool_result even when error, got type=%s", out[0].Type)
	}
	tr, ok := out[0].ToolResult.(map[string]any)
	if !ok {
		t.Fatalf("error ToolResult should be map[string]any, got %T", out[0].ToolResult)
	}
	if errText, ok := tr["error"].(string); !ok || errText == "" {
		t.Errorf("ToolResult should contain 'error': %v", tr)
	}
}

func TestExecuteTools_MixedSuccessSkipAndError(t *testing.T) {
	exec := &fakeToolExecutor{
		results: map[string]any{"good": "ok"},
		errs:    map[string]error{"bad": context.DeadlineExceeded},
	}
	bindings := []domain.CanonicalTool{
		{Name: "good", MCPTarget: &domain.MCPBinding{ServerName: "good-srv"}},
		{Name: "bad", MCPTarget: &domain.MCPBinding{ServerName: "bad-srv"}},
	}
	content := []domain.CanonicalContentBlock{
		{Type: domain.ContentBlockToolUse, ToolName: "good", ToolCallID: "tc-good"},
		{Type: domain.ContentBlockToolUse, ToolName: "skipped", ToolCallID: "tc-skip"},
		{Type: domain.ContentBlockToolUse, ToolName: "bad", ToolCallID: "tc-bad"},
	}

	out := ExecuteTools(context.Background(), exec, content, bindings, "", "")
	if len(out) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(out))
	}
	// good → success tool_result
	if out[0].Type != domain.ContentBlockToolResult || out[0].ToolResult != "ok" {
		t.Errorf("good block: %+v", out[0])
	}
	// skipped → no binding → original tool_use preserved
	if out[1].Type != domain.ContentBlockToolUse || out[1].ToolName != "skipped" {
		t.Errorf("skipped block should remain tool_use: %+v", out[1])
	}
	// bad → error tool_result
	if out[2].Type != domain.ContentBlockToolResult {
		t.Errorf("bad should become tool_result: %+v", out[2])
	}
	tr, ok := out[2].ToolResult.(map[string]any)
	if !ok || tr["error"] == nil {
		t.Errorf("bad ToolResult: %v", out[2].ToolResult)
	}

	// Executor should have been called for good + bad only (2 calls)
	if len(exec.calls) != 2 {
		t.Errorf("expected 2 calls (good+bad), got %d: %v", len(exec.calls), exec.calls)
	}
}

// ── Integration test: GatewayService + ToolExecutor + request Tools ──

type mockUpstreamWithToolUse struct {
	chatResp *domain.CanonicalResponse
}

func (m *mockUpstreamWithToolUse) Name() string              { return "test" }
func (m *mockUpstreamWithToolUse) SupportedProtocol() string { return "openai" }
func (m *mockUpstreamWithToolUse) Chat(_ context.Context, _ *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	return m.chatResp, nil
}
func (m *mockUpstreamWithToolUse) ChatStream(_ context.Context, _ *domain.CanonicalRequest) (<-chan *domain.CanonicalStreamEvent, error) {
	return nil, nil
}
func (m *mockUpstreamWithToolUse) Embedding(_ context.Context, _ *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	return nil, nil
}

func TestGatewayService_Chat_ToolExecutor_Integration(t *testing.T) {
	rpm := 1000
	tpm := 50000
	exec := &fakeToolExecutor{
		results: map[string]any{
			"get_time": "2026-10-15T10:00:00Z",
		},
	}

	upstream := &mockUpstreamWithToolUse{
		chatResp: &domain.CanonicalResponse{
			Content: []domain.CanonicalContentBlock{
				{Type: domain.ContentBlockText, Text: "Let me check that"},
				{Type: domain.ContentBlockToolUse, ToolName: "get_time", ToolCallID: "tc-1",
					ToolInput: map[string]any{}},
			},
			InputTokens: 5, OutputTokens: 3, TotalTokens: 8,
		},
	}

	router := &mockRouter{
		res: &domain.RouteResolution{
			Provider:     &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: "openai"},
			Model:        &domain.LlmModel{ID: "m-1", ProviderModelName: "gpt-4"},
			Route:        &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1},
			ProtocolFace: "openai",
			Health:       &domain.ProviderHealth{},
		},
	}
	registry := newMockRegistry()
	registry.Register("openai", upstream)

	svc := buildServiceWithDeps(t, nil, nil, &fakeApiKeysWithLimits{rpm: &rpm, tpm: &tpm})
	svc.Upstreams = registry
	svc.Routers = router
	svc.ToolExecutor = exec

	// Request includes the tool declaration with MCPTarget — this is what
	// execute() looks up to pass the binding to the executor.
	req := makeValidReq()
	req.Tools = []domain.CanonicalTool{
		{
			Name:        "get_time",
			Description: "Get current server time",
			MCPTarget:   &domain.MCPBinding{ServerName: "clock-server", ToolName: "get_time", OrgSlug: "demo"},
		},
	}

	resp, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Verify tool_use was executed and converted to tool_result
	if len(resp.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(resp.Content))
	}
	if resp.Content[0].Type != domain.ContentBlockText {
		t.Errorf("block 0 should be text, got %s", resp.Content[0].Type)
	}
	if resp.Content[1].Type != domain.ContentBlockToolResult {
		t.Fatalf("block 1 should be tool_result, got type=%s", resp.Content[1].Type)
	}
	if resp.Content[1].ToolResult != "2026-10-15T10:00:00Z" {
		t.Errorf("expected timestamp, got %v", resp.Content[1].ToolResult)
	}

	// Verify executor got correct binding (ServerName)
	if len(exec.calls) != 1 || exec.calls[0] != "get_time" {
		t.Errorf("exec calls = %v", exec.calls)
	}
	if len(exec.servers) != 1 || exec.servers[0] != "clock-server" {
		t.Errorf("expected server=clock-server, got %v", exec.servers)
	}
}

func TestGatewayService_Chat_NoMCPBinding_Passthrough(t *testing.T) {
	// Request has Tools but without MCPTarget — tool_use blocks should pass through
	rpm := 1000
	tpm := 50000
	exec := &fakeToolExecutor{} // no results, but binding lookup will skip

	upstream := &mockUpstreamWithToolUse{
		chatResp: &domain.CanonicalResponse{
			Content: []domain.CanonicalContentBlock{
				{Type: domain.ContentBlockToolUse, ToolName: "get_time", ToolCallID: "tc-1"},
			},
			InputTokens: 5, OutputTokens: 3, TotalTokens: 8,
		},
	}
	router := &mockRouter{
		res: &domain.RouteResolution{
			Provider:     &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", Name: "openai"},
			Model:        &domain.LlmModel{ID: "m-1", ProviderModelName: "gpt-4"},
			Route:        &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 1},
			ProtocolFace: "openai",
			Health:       &domain.ProviderHealth{},
		},
	}
	registry := newMockRegistry()
	registry.Register("openai", upstream)

	svc := buildServiceWithDeps(t, nil, nil, &fakeApiKeysWithLimits{rpm: &rpm, tpm: &tpm})
	svc.Upstreams = registry
	svc.Routers = router
	svc.ToolExecutor = exec

	req := makeValidReq()
	// No MCPTarget on the tool declaration → lookup will fail → tool_use preserved
	req.Tools = []domain.CanonicalTool{
		{Name: "get_time", Description: "no mcp binding on purpose"},
	}

	resp, err := svc.Chat(context.Background(), "hash-xyz", "gpt-4", req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(resp.Content))
	}
	if resp.Content[0].Type != domain.ContentBlockToolUse {
		t.Errorf("without MCPTarget, tool_use should be preserved, got type=%s", resp.Content[0].Type)
	}
	if len(exec.calls) != 0 {
		t.Errorf("executor should NOT be called when MCPTarget missing: calls=%v", exec.calls)
	}
}

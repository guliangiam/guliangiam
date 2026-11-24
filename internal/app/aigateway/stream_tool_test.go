package aigateway

import (
	"context"
	"sync"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── ChatStream tool_use 自动执行测试 ─────────────────────────────────

type fakeStreamToolExecutor struct {
	mu     sync.Mutex
	calls  []toolCall
	result any
	err    error
}

type toolCall struct {
	ToolName string
	Input    map[string]any
}

func newFakeStreamToolExecutor(result any, err error) *fakeStreamToolExecutor {
	return &fakeStreamToolExecutor{result: result, err: err}
}

func (f *fakeStreamToolExecutor) ExecuteTool(_ context.Context, _ *domain.MCPBinding, toolName string, input map[string]any) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, toolCall{ToolName: toolName, Input: input})
	return f.result, f.err
}

func (f *fakeStreamToolExecutor) Calls() []toolCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]toolCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// makeValidReqWithTools 构造带 tools 的请求，每个 tool 有 MCPTarget 可被 ExecuteTools 找到
func makeValidReqWithTools() *domain.CanonicalRequest {
	return &domain.CanonicalRequest{
		ModelAlias: "gpt-4",
		Messages: []domain.CanonicalMessage{
			{Role: domain.RoleUser, Content: []domain.CanonicalContentBlock{{Type: domain.ContentBlockText, Text: "hello"}}},
		},
		Tools: []domain.CanonicalTool{
			{Name: "get_weather", MCPTarget: &domain.MCPBinding{McpServerID: "s-1", ServerName: "weather", ToolName: "get_weather"}},
		},
	}
}

// TestChatStream_ToolUse_AutoExecute 验证流式路径的 tool_use 自动执行闭环
// 1. 上游流产生 ToolCallDelta (tool_use)
// 2. tee goroutine 聚合 → 执行 → 回灌 tool_result 事件
// 3. handler 收到 tool_result 事件
func TestChatStream_ToolUse_AutoExecute(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	executor := newFakeStreamToolExecutor(map[string]any{"temp": 22}, nil)
	svc.ToolExecutor = nil // 先留 nil

	// 上游流事件：message_start → tool_use_delta → usage → message_stop
	ch := make(chan *domain.CanonicalStreamEvent, 5)
	ch <- &domain.CanonicalStreamEvent{Type: "message_start"}
	ch <- &domain.CanonicalStreamEvent{
		Type: "tool_call_delta",
		ToolCallDelta: &domain.CanonicalContentBlock{
			Type:       domain.ContentBlockToolUse,
			ToolCallID: "call-1",
			ToolName:   "get_weather",
			ToolInput:  map[string]any{"city": "beijing"},
		},
	}
	ch <- &domain.CanonicalStreamEvent{
		Type:          "message_delta",
		UsageMetadata: map[string]any{"input_tokens": 10, "output_tokens": 5},
	}
	ch <- &domain.CanonicalStreamEvent{Type: "message_stop"}
	close(ch)
	upstream.streamCh = ch
	svc.ToolExecutor = executor

	streamCh, err := svc.ChatStream(context.Background(), "h", "gpt-4", makeValidReqWithTools())
	require.NoError(t, err)

	var received []*domain.CanonicalStreamEvent
	for ev := range streamCh {
		received = append(received, ev)
	}

	// 应该有 5 个事件：message_start, tool_call_delta, message_delta, message_stop, tool_result
	require.Len(t, received, 5)
	assert.Equal(t, "message_start", received[0].Type)
	assert.Equal(t, "tool_call_delta", received[1].Type)
	assert.Equal(t, "message_delta", received[2].Type)
	assert.Equal(t, "message_stop", received[3].Type)

	// tool_result 是最后一个
	last := received[4]
	assert.Equal(t, "tool_result", last.Type)
	require.NotNil(t, last.ContentDelta)
	assert.Equal(t, domain.ContentBlockToolResult, last.ContentDelta.Type)
	assert.Equal(t, "call-1", last.ContentDelta.ToolCallID)
	assert.Equal(t, "get_weather", last.ContentDelta.ToolName)
	result, ok := last.ContentDelta.ToolResult.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 22, result["temp"])

	// ToolExecutor 被调用 1 次
	require.Len(t, executor.Calls(), 1)
	assert.Equal(t, "get_weather", executor.Calls()[0].ToolName)
	assert.Equal(t, "beijing", executor.Calls()[0].Input["city"])
}

// TestChatStream_ToolUse_ExecutorNil_FailOpen 验证 ToolExecutor nil 时 Fail-Open：
// tool_use 不自动执行，不会产生 tool_result 事件。
func TestChatStream_ToolUse_ExecutorNil_FailOpen(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	svc.ToolExecutor = nil // 显式 nil

	ch := make(chan *domain.CanonicalStreamEvent, 3)
	ch <- &domain.CanonicalStreamEvent{Type: "message_start"}
	ch <- &domain.CanonicalStreamEvent{
		Type: "tool_call_delta",
		ToolCallDelta: &domain.CanonicalContentBlock{
			Type:       domain.ContentBlockToolUse,
			ToolCallID: "call-1",
			ToolName:   "get_weather",
			ToolInput:  map[string]any{"city": "beijing"},
		},
	}
	ch <- &domain.CanonicalStreamEvent{Type: "message_stop"}
	close(ch)
	upstream.streamCh = ch

	streamCh, err := svc.ChatStream(context.Background(), "h", "gpt-4", makeValidReqWithTools())
	require.NoError(t, err)

	var received []*domain.CanonicalStreamEvent
	for ev := range streamCh {
		received = append(received, ev)
	}

	// 只有 3 个事件，没有 tool_result
	assert.Len(t, received, 3)
	assert.Equal(t, "tool_call_delta", received[1].Type)
	assert.NotEqual(t, "tool_result", received[2].Type) // 最后一个是 message_stop
}

// TestChatStream_ToolUse_NoMCPBinding 验证 req.Tools 没有对应 MCPBinding 时
// ExecuteTools 原样返回（Fail-Open 语义）——不会产生 tool_result 事件。
func TestChatStream_ToolUse_NoMCPBinding(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	executor := newFakeStreamToolExecutor(map[string]any{"temp": 22}, nil)
	svc.ToolExecutor = executor

	// 注意：makeValidReq 不带 Tools，所以 findMCPBinding 找不到 → ExecuteTools 原样返回
	ch := make(chan *domain.CanonicalStreamEvent, 3)
	ch <- &domain.CanonicalStreamEvent{Type: "message_start"}
	ch <- &domain.CanonicalStreamEvent{
		Type: "tool_call_delta",
		ToolCallDelta: &domain.CanonicalContentBlock{
			Type:       domain.ContentBlockToolUse,
			ToolCallID: "call-1",
			ToolName:   "unknown_tool",
			ToolInput:  map[string]any{"x": 1},
		},
	}
	ch <- &domain.CanonicalStreamEvent{Type: "message_stop"}
	close(ch)
	upstream.streamCh = ch

	streamCh, err := svc.ChatStream(context.Background(), "h", "gpt-4", makeValidReq())
	require.NoError(t, err)

	var received []*domain.CanonicalStreamEvent
	for ev := range streamCh {
		received = append(received, ev)
	}

	// 没有 tool_result 事件（因为 ExecuteTools 原样返回，tool_use 块没被替换）
	// 但 tool_use 块不是 ContentBlockToolResult 类型，所以我们的 for 循环 filter 掉了
	assert.Len(t, received, 3)
	assert.Equal(t, 0, len(executor.Calls())) // executor 没被调用
}

// TestChatStream_ToolUse_MultipleDeltas_AggregatesInput 验证多个增量 ToolCallDelta
// 能正确合并 ToolInput map。
func TestChatStream_ToolUse_MultipleDeltas_AggregatesInput(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	executor := newFakeStreamToolExecutor(map[string]any{"ok": true}, nil)
	svc.ToolExecutor = executor

	// 3 个增量 delta，同一个 ToolCallID
	ch := make(chan *domain.CanonicalStreamEvent, 5)
	ch <- &domain.CanonicalStreamEvent{Type: "message_start"}
	ch <- &domain.CanonicalStreamEvent{
		Type: "tool_call_delta",
		ToolCallDelta: &domain.CanonicalContentBlock{
			Type:       domain.ContentBlockToolUse,
			ToolCallID: "call-1",
			ToolName:   "get_weather",
			ToolInput:  map[string]any{"city": "beijing"},
		},
	}
	ch <- &domain.CanonicalStreamEvent{
		Type: "tool_call_delta",
		ToolCallDelta: &domain.CanonicalContentBlock{
			Type:       domain.ContentBlockToolUse,
			ToolCallID: "call-1",
			ToolInput:  map[string]any{"units": "celsius"}, // 增量：新增 field
		},
	}
	ch <- &domain.CanonicalStreamEvent{
		Type: "tool_call_delta",
		ToolCallDelta: &domain.CanonicalContentBlock{
			Type:       domain.ContentBlockToolUse,
			ToolCallID: "call-1",
			ToolInput:  map[string]any{"days": 3}, // 再加一个 field
		},
	}
	ch <- &domain.CanonicalStreamEvent{Type: "message_stop"}
	close(ch)
	upstream.streamCh = ch

	streamCh, err := svc.ChatStream(context.Background(), "h", "gpt-4", makeValidReqWithTools())
	require.NoError(t, err)

	var received []*domain.CanonicalStreamEvent
	for ev := range streamCh {
		received = append(received, ev)
	}

	// 最后一个是 tool_result
	last := received[len(received)-1]
	require.Equal(t, "tool_result", last.Type)

	// executor 收到合并后的 input
	require.Len(t, executor.Calls(), 1)
	input := executor.Calls()[0].Input
	assert.Equal(t, "beijing", input["city"])
	assert.Equal(t, "celsius", input["units"])
	assert.Equal(t, 3, input["days"])
}

// TestChatStream_ToolUse_NoToolUseBlocks 验证没有 tool_use 的流不受影响
func TestChatStream_ToolUse_NoToolUseBlocks(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	executor := newFakeStreamToolExecutor(nil, nil)
	svc.ToolExecutor = executor

	ch := make(chan *domain.CanonicalStreamEvent, 3)
	ch <- &domain.CanonicalStreamEvent{Type: "message_start"}
	ch <- &domain.CanonicalStreamEvent{
		Type:         "content_delta",
		ContentDelta: &domain.CanonicalContentBlock{Type: domain.ContentBlockText, Text: "hello"},
	}
	ch <- &domain.CanonicalStreamEvent{Type: "message_stop"}
	close(ch)
	upstream.streamCh = ch

	streamCh, err := svc.ChatStream(context.Background(), "h", "gpt-4", makeValidReq())
	require.NoError(t, err)

	var received []*domain.CanonicalStreamEvent
	for ev := range streamCh {
		received = append(received, ev)
	}

	assert.Len(t, received, 3)
	assert.Equal(t, 0, len(executor.Calls()))
}

// TestChatStream_ToolUse_ExecutorError_InjectsErrorResult 验证 ToolExecutor 返回 error 时
// 注入带 error 的 tool_result 块。
func TestChatStream_ToolUse_ExecutorError_InjectsErrorResult(t *testing.T) {
	svc, _, upstream, _ := makeService(t)
	executor := newFakeStreamToolExecutor(nil, errFakeError("mcp server timeout"))
	svc.ToolExecutor = executor

	ch := make(chan *domain.CanonicalStreamEvent, 3)
	ch <- &domain.CanonicalStreamEvent{Type: "message_start"}
	ch <- &domain.CanonicalStreamEvent{
		Type: "tool_call_delta",
		ToolCallDelta: &domain.CanonicalContentBlock{
			Type:       domain.ContentBlockToolUse,
			ToolCallID: "call-1",
			ToolName:   "get_weather",
			ToolInput:  map[string]any{"city": "beijing"},
		},
	}
	ch <- &domain.CanonicalStreamEvent{Type: "message_stop"}
	close(ch)
	upstream.streamCh = ch

	streamCh, err := svc.ChatStream(context.Background(), "h", "gpt-4", makeValidReqWithTools())
	require.NoError(t, err)

	var received []*domain.CanonicalStreamEvent
	for ev := range streamCh {
		received = append(received, ev)
	}

	last := received[len(received)-1]
	require.Equal(t, "tool_result", last.Type)
	require.NotNil(t, last.ContentDelta)
	assert.Equal(t, domain.ContentBlockToolResult, last.ContentDelta.Type)

	// ToolResult 里应该有 error 字段
	tr, ok := last.ContentDelta.ToolResult.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "mcp server timeout", tr["error"])
}

type errFakeError string

func (e errFakeError) Error() string { return string(e) }

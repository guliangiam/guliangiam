package aigateway

import (
	"encoding/json"
	"reflect"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

// ============================================================================
// ConvertChatRequestForFace — body shape autodetect
// ============================================================================

func TestConvertChatRequestForFace_OpenAI(t *testing.T) {
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"max_tokens":100,"stream":true}`
	req, err := ConvertChatRequestForFace("openai", []byte(body))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if req.Model != "gpt-4" {
		t.Errorf("model = %q, want gpt-4", req.Model)
	}
	if !req.Stream {
		t.Error("stream = false, want true")
	}
	if req.MaxTokens == nil || *req.MaxTokens != 100 {
		t.Errorf("max_tokens = %v, want 100", req.MaxTokens)
	}
	if len(req.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(req.Messages))
	}
	if req.Messages[0].Role != "user" || req.Messages[0].Content.(string) != "hi" {
		t.Errorf("message = %+v", req.Messages[0])
	}
}

func TestConvertChatRequestForFace_AnthropicNative(t *testing.T) {
	// Anthropic Messages 原生格式 — 顶层 system / max_tokens / messages[]
	body := `{
		"model": "claude-3",
		"system": "You are helpful",
		"max_tokens": 512,
		"stream": false,
		"messages": [
			{"role":"user","content":[{"type":"text","text":"Hello"}]},
			{"role":"assistant","content":[{"type":"text","text":"Hi there"}]}
		],
		"stop_sequences": ["END"],
		"tools": [{"name":"search","description":"web search","input_schema":{"type":"object"}}]
	}`
	req, err := ConvertChatRequestForFace("anthropic", []byte(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if req.Model != "claude-3" {
		t.Errorf("model = %q", req.Model)
	}
	// system 应该被 prepend 为第一条 role=system
	if len(req.Messages) != 3 {
		t.Fatalf("messages count = %d, want 3 (system + user + assistant)", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Errorf("messages[0] role = %q, want system", req.Messages[0].Role)
	}
	if req.Messages[0].Content.(string) != "You are helpful" {
		t.Errorf("messages[0] content = %v", req.Messages[0].Content)
	}
	// stop_sequences → Stop
	if len(req.Stop) != 1 || req.Stop[0] != "END" {
		t.Errorf("stop = %v", req.Stop)
	}
	// tools: Anthropic input_schema → ChatTool.Function.Parameters
	if len(req.Tools) != 1 {
		t.Fatalf("tools = %d", len(req.Tools))
	}
	if req.Tools[0].Function.Name != "search" {
		t.Errorf("tool name = %q", req.Tools[0].Function.Name)
	}
	if req.Tools[0].Function.Parameters == nil {
		t.Error("tool parameters = nil")
	}
}

func TestConvertChatRequestForFace_AnthropicNative_SystemBlocks(t *testing.T) {
	// system 可以是 [{type:"text",text:"..."}] 数组
	body := `{
		"model":"claude-3",
		"system":[{"type":"text","text":"SYS1"},{"type":"text","text":"SYS2"}],
		"max_tokens":100,
		"messages":[{"role":"user","content":"hi"}]
	}`
	req, err := ConvertChatRequestForFace("anthropic", []byte(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// 两个 system block 各转成一条 role=system
	if len(req.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content.(string) != "SYS1" {
		t.Errorf("msg[0] = %+v", req.Messages[0])
	}
	if req.Messages[1].Role != "system" || req.Messages[1].Content.(string) != "SYS2" {
		t.Errorf("msg[1] = %+v", req.Messages[1])
	}
}

func TestConvertChatRequestForFace_GeminiNative(t *testing.T) {
	body := `{
		"model":"gemini-pro",
		"systemInstruction":{"parts":[{"text":"Be concise"}]},
		"contents":[
			{"role":"user","parts":[{"text":"Hello"}]},
			{"role":"model","parts":[{"text":"Hi!"},{"functionCall":{"name":"lookup","args":{"q":"test"}}}]},
			{"role":"user","parts":[{"functionResponse":{"name":"lookup","response":"found!"}}]}
		],
		"generationConfig":{"temperature":0.7,"maxOutputTokens":256,"stopSequences":["."]},
		"stream":true
	}`
	req, err := ConvertChatRequestForFace("gemini", []byte(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if req.Model != "gemini-pro" {
		t.Errorf("model = %q", req.Model)
	}
	if !req.Stream {
		t.Error("stream = false")
	}
	// systemInstruction → system message
	if len(req.Messages) != 4 {
		t.Fatalf("messages = %d, want 4 (system + user + model + user)", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Errorf("msg[0] role = %q", req.Messages[0].Role)
	}
	if req.Messages[1].Role != "user" {
		t.Errorf("msg[1] role = %q", req.Messages[1].Role)
	}
	// Gemini "model" role → OpenAI "assistant"
	if req.Messages[2].Role != "assistant" {
		t.Errorf("msg[2] role = %q, want assistant", req.Messages[2].Role)
	}
	// generationConfig.maxOutputTokens → MaxTokens
	if req.MaxTokens == nil || *req.MaxTokens != 256 {
		t.Errorf("max_tokens = %v", req.MaxTokens)
	}
	// generationConfig.temperature → Temperature
	if req.Temperature == nil || *req.Temperature != 0.7 {
		t.Errorf("temperature = %v", req.Temperature)
	}
	// generationConfig.stopSequences → Stop
	if len(req.Stop) != 1 || req.Stop[0] != "." {
		t.Errorf("stop = %v", req.Stop)
	}
	// model message 里的 functionCall → ChatToolCall
	if req.Messages[2].ToolCalls == nil || len(req.Messages[2].ToolCalls) != 1 {
		t.Fatalf("model msg tool_calls = %v", req.Messages[2].ToolCalls)
	}
	if req.Messages[2].ToolCalls[0].Function.Name != "lookup" {
		t.Errorf("tool_call name = %q", req.Messages[2].ToolCalls[0].Function.Name)
	}
}

func TestConvertChatRequestForFace_EmptyBody(t *testing.T) {
	_, err := ConvertChatRequestForFace("openai", nil)
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestConvertChatRequestForFace_InvalidJSON(t *testing.T) {
	_, err := ConvertChatRequestForFace("openai", []byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ============================================================================
// ConvertChatResponseAnthropic / Gemini
// ============================================================================

func TestConvertChatResponseAnthropic(t *testing.T) {
	cr := &domain.CanonicalResponse{
		Content: []domain.CanonicalContentBlock{
			{Type: domain.ContentBlockText, Text: "Hello"},
			{Type: domain.ContentBlockToolUse, ToolCallID: "tc_1", ToolName: "search", ToolInput: map[string]any{"q": "hi"}},
		},
		InputTokens: 10, OutputTokens: 5, TotalTokens: 15,
		FinishReason: "tool_calls",
	}
	out := ConvertChatResponseAnthropic(cr, "claude-3")

	if out["type"] != "message" {
		t.Errorf("type = %v", out["type"])
	}
	content, _ := out["content"].([]map[string]any)
	if len(content) != 2 {
		t.Fatalf("content blocks = %d", len(content))
	}
	if content[0]["type"] != "text" {
		t.Errorf("block[0].type = %v", content[0]["type"])
	}
	if content[1]["type"] != "tool_use" {
		t.Errorf("block[1].type = %v", content[1]["type"])
	}
	if out["stop_reason"] != "tool_use" {
		t.Errorf("stop_reason = %v, want tool_use", out["stop_reason"])
	}
}

func TestConvertChatResponseGemini(t *testing.T) {
	cr := &domain.CanonicalResponse{
		Content: []domain.CanonicalContentBlock{
			{Type: domain.ContentBlockText, Text: "Done"},
		},
		InputTokens: 8, OutputTokens: 4, TotalTokens: 12,
		FinishReason: "stop",
	}
	out := ConvertChatResponseGemini(cr, "gemini-pro")

	candidates, _ := out["candidates"].([]map[string]any)
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d", len(candidates))
	}
	c := candidates[0]
	if c["finishReason"] != "STOP" {
		t.Errorf("finishReason = %v, want STOP", c["finishReason"])
	}
	if out["modelVersion"] != "gemini-pro" {
		t.Errorf("modelVersion = %v", out["modelVersion"])
	}
	um, _ := out["usageMetadata"].(map[string]any)
	if um["totalTokenCount"] != 12 {
		t.Errorf("totalTokenCount = %v", um["totalTokenCount"])
	}
}

// ============================================================================
// SSE Converters
// ============================================================================

func TestDomainEventToSSE_ContentDelta(t *testing.T) {
	ev := &domain.CanonicalStreamEvent{
		Type: "content_delta",
		ContentDelta: &domain.CanonicalContentBlock{
			Type: domain.ContentBlockText,
			Text: "Hello",
		},
	}
	out := DomainEventToSSE(ev, "cmpl-1", "gpt-4", 1234567890)
	if out == nil {
		t.Fatal("got nil")
	}
	choices, _ := out["choices"].([]map[string]any)
	delta := choices[0]["delta"].(map[string]any)
	if delta["content"] != "Hello" {
		t.Errorf("delta.content = %v", delta["content"])
	}
	if out["object"] != "chat.completion.chunk" {
		t.Errorf("object = %v", out["object"])
	}
}

func TestDomainEventToAnthropicSSE_ContentDelta(t *testing.T) {
	ev := &domain.CanonicalStreamEvent{
		Type: "content_delta",
		ContentDelta: &domain.CanonicalContentBlock{
			Type: domain.ContentBlockText,
			Text: "Hello",
		},
	}
	evt, payload := DomainEventToAnthropicSSE(ev, "msg-1", "claude-3")
	if evt != "content_block_delta" {
		t.Errorf("event = %q, want content_block_delta", evt)
	}
	delta := payload["delta"].(map[string]any)
	if delta["type"] != "text_delta" {
		t.Errorf("delta.type = %v", delta["type"])
	}
	if delta["text"] != "Hello" {
		t.Errorf("delta.text = %v", delta["text"])
	}
}

func TestDomainEventToAnthropicSSE_MessageStop(t *testing.T) {
	ev := &domain.CanonicalStreamEvent{Type: "message_stop"}
	evt, payload := DomainEventToAnthropicSSE(ev, "msg-1", "claude-3")
	if evt != "message_stop" {
		t.Errorf("event = %q, want message_stop", evt)
	}
	if payload["type"] != "message_stop" {
		t.Errorf("payload = %v", payload)
	}
}

func TestDomainEventToAnthropicSSE_Ignored(t *testing.T) {
	for _, typ := range []string{"ping", "unknown_type", ""} {
		ev := &domain.CanonicalStreamEvent{Type: typ}
		evt, payload := DomainEventToAnthropicSSE(ev, "msg-1", "claude-3")
		if evt != "" || payload != nil {
			t.Errorf("type=%q: expected empty/nil, got evt=%q payload=%v", typ, evt, payload)
		}
	}
}

func TestDomainEventToGeminiSSE_Content(t *testing.T) {
	ev := &domain.CanonicalStreamEvent{
		Type: "content_delta",
		ContentDelta: &domain.CanonicalContentBlock{
			Type: domain.ContentBlockText,
			Text: "Hello",
		},
	}
	evt, payload := DomainEventToGeminiSSE(ev, "gemini-pro")
	if evt != "content" {
		t.Errorf("event = %q, want content", evt)
	}
	candidates, _ := payload["candidates"].([]map[string]any)
	parts := candidates[0]["content"].(map[string]any)["parts"].([]map[string]any)
	if parts[0]["text"] != "Hello" {
		t.Errorf("parts[0] = %v", parts[0])
	}
}

func TestDomainEventToGeminiSSE_Finish(t *testing.T) {
	ev := &domain.CanonicalStreamEvent{
		Type: "message_delta",
		UsageMetadata: map[string]any{
			"input_tokens":  8,
			"output_tokens": 4,
			"total_tokens":  12,
		},
	}
	evt, payload := DomainEventToGeminiSSE(ev, "gemini-pro")
	if evt != "finish" {
		t.Errorf("event = %q, want finish", evt)
	}
	candidates, _ := payload["candidates"].([]map[string]any)
	if candidates[0]["finishReason"] != "STOP" {
		t.Errorf("finishReason = %v", candidates[0]["finishReason"])
	}
	um, _ := payload["usageMetadata"].(map[string]any)
	if um["totalTokenCount"] != 12 {
		t.Errorf("totalTokenCount = %v", um["totalTokenCount"])
	}
	if payload["modelVersion"] != "gemini-pro" {
		t.Errorf("modelVersion missing")
	}
}

// ============================================================================
// JSON 序列化 smoke — 确保 converter 产出可以 Marshal 且字段名正确
// ============================================================================

func TestAnthropicResponse_JSONShape(t *testing.T) {
	cr := &domain.CanonicalResponse{
		Content: []domain.CanonicalContentBlock{
			{Type: domain.ContentBlockText, Text: "Hello"},
		},
		InputTokens: 10, OutputTokens: 5, FinishReason: "stop",
	}
	out := ConvertChatResponseAnthropic(cr, "claude-3")
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// 顶层字段
	for _, k := range []string{"id", "type", "role", "content", "model", "stop_reason", "usage"} {
		if _, ok := parsed[k]; !ok {
			t.Errorf("missing field %q in %s", k, string(b))
		}
	}
}

func TestGeminiResponse_JSONShape(t *testing.T) {
	cr := &domain.CanonicalResponse{
		Content: []domain.CanonicalContentBlock{
			{Type: domain.ContentBlockText, Text: "Hello"},
		},
		InputTokens: 10, OutputTokens: 5, FinishReason: "stop",
	}
	out := ConvertChatResponseGemini(cr, "gemini-pro")
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"candidates", "modelVersion", "usageMetadata"} {
		if _, ok := parsed[k]; !ok {
			t.Errorf("missing field %q in %s", k, string(b))
		}
	}
}

// ============================================================================
// 零值 / edge case
// ============================================================================

func TestConvertChatResponseAnthropic_EmptyContent(t *testing.T) {
	cr := &domain.CanonicalResponse{FinishReason: "stop"}
	out := ConvertChatResponseAnthropic(cr, "claude-3")
	if reflect.DeepEqual(out["content"], []any{}) {
		t.Error("content should be non-nil (empty array), not nil")
	}
}

func TestConvertChatRequestForFace_OpenAIWithToolCalls(t *testing.T) {
	body := `{
		"model":"gpt-4",
		"messages":[
			{"role":"assistant","content":null,"tool_calls":[{"id":"tc1","type":"function","function":{"name":"search","arguments":"{\"q\":\"x\"}"}}]},
			{"role":"tool","content":"result","tool_call_id":"tc1"},
			{"role":"user","content":"ok"}
		]
	}`
	req, err := ConvertChatRequestForFace("openai", []byte(body))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(req.Messages) != 3 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
	if req.Messages[0].Role != "assistant" {
		t.Errorf("msg[0] role = %q", req.Messages[0].Role)
	}
	if len(req.Messages[0].ToolCalls) != 1 {
		t.Fatalf("tool_calls = %v", req.Messages[0].ToolCalls)
	}
	if req.Messages[0].ToolCalls[0].ID != "tc1" {
		t.Errorf("tool_call id = %q", req.Messages[0].ToolCalls[0].ID)
	}
	if req.Messages[1].Role != "tool" {
		t.Errorf("msg[1] role = %q", req.Messages[1].Role)
	}
	if req.Messages[1].ToolCallID != "tc1" {
		t.Errorf("msg[1] tool_call_id = %q", req.Messages[1].ToolCallID)
	}
}

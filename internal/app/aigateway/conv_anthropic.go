package aigateway

import (
	"fmt"
	"strconv"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

// ============================================================================
// Protocol-Native Streaming SSE — Anthropic
// ============================================================================
//
// Anthropic Messages Streaming (event: ...  envelope)
//   message_start     → {"type":"message_start","message":{"id","type":"message","role":"assistant","model"}}
//   content_block_start → {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
//   content_block_delta → {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"..."}}
//   content_block_stop  → {"type":"content_block_stop","index":0}
//   message_delta      → {"type":"message_delta","delta":{"stop_reason":"end_turn|max_tokens|tool_use"},"usage":{"output_tokens":n}}
//   message_stop       → {"type":"message_stop"}

// DomainEventToAnthropicSSE 把 CanonicalStreamEvent 转成 Anthropic Messages
// Streaming event envelope。返回空 event="" 表示该事件不产出 SSE（keepalive）。
//
// 第一次调用时（ev.Type == "content_delta" 且尚未 emit message_start），
// 调用方需要先发送 message_start + content_block_start。这个函数不处理
// 首帧/尾帧 —— handler 层负责帧顺序。
func DomainEventToAnthropicSSE(ev *domain.CanonicalStreamEvent, msgID, model string) (event string, meta map[string]any) {
	if ev == nil {
		return "", nil
	}

	switch ev.Type {
	case eventContentDelta:
		if ev.ContentDelta != nil && ev.ContentDelta.Type == domain.ContentBlockText {
			return eventContentBlockDelta, map[string]any{
				fieldType:  eventContentBlockDelta,
				fieldIndex: 0,
				fieldDelta: map[string]any{
					fieldType: "text_delta",
					fieldText: ev.ContentDelta.Text,
				},
			}
		}
	case eventToolCallDelta:
		if ev.ToolCallDelta != nil {
			return eventContentBlockDelta, map[string]any{
				fieldType:  eventContentBlockDelta,
				fieldIndex: 0,
				fieldDelta: map[string]any{
					fieldType:      "input_json_delta",
					"partial_json": toolInputToJSON(ev.ToolCallDelta.ToolInput),
				},
			}
		}
	case eventMessageDelta:
		stopReason := "end_turn"
		usage := map[string]any{}
		if ev.UsageMetadata != nil {
			if v, ok := ev.UsageMetadata["output_tokens"]; ok {
				usage["output_tokens"] = v
			}
		}
		payload := map[string]any{
			fieldType:  eventMessageDelta,
			fieldDelta: map[string]any{"stop_reason": stopReason},
		}
		if len(usage) > 0 {
			payload["usage"] = usage
		}
		return eventMessageDelta, payload
	case eventMessageStop:
		return eventMessageStop, map[string]any{
			fieldType: eventMessageStop,
		}
	case eventMessageStart:
		// 首帧 message_start — handler 层在收到第一条 content_delta 前调用
		return eventMessageStart, map[string]any{
			fieldType: eventMessageStart,
			fieldMessage: map[string]any{
				"id":       msgID,
				fieldType:  fieldMessage,
				fieldRole:  roleAssistant,
				fieldModel: model,
			},
		}
	}

	return "", nil
}

// ---------------------------------------------------------------------------
// Anthropic Protocol: CanonicalResponse → Anthropic Messages response
// ---------------------------------------------------------------------------

// ConvertChatResponseAnthropic 把 CanonicalResponse 转成 Anthropic Messages
// POST /v1/messages 响应格式。
//
// Anthropic schema:
//
//	{
//	  "id": "msg_xxx",
//	  "type": "message",
//	  "role": "assistant",
//	  "content": [{"type": "text", "text": "..."} | {"type": "tool_use", ...}],
//	  "stop_reason": "end_turn" | "tool_use" | "max_tokens",
//	  "model": "...",
//	  "usage": {"input_tokens": n, "output_tokens": n}
//	}
func ConvertChatResponseAnthropic(cr *domain.CanonicalResponse, modelAlias string) map[string]any {
	content := make([]map[string]any, 0, 2)

	// 1. text content
	if text := ExtractText(cr.Content); text != "" {
		content = append(content, map[string]any{
			fieldType: fieldText,
			fieldText: text,
		})
	}

	// 2. tool_use blocks (CanonicalContentBlock.ToolUse → Anthropic tool_use)
	for _, b := range cr.Content {
		if b.Type == domain.ContentBlockToolUse {
			content = append(content, map[string]any{
				fieldType: fieldToolUse,
				"id":      b.ToolCallID,
				fieldName: b.ToolName,
				"input":   b.ToolInput,
			})
		}
	}

	// 3. stop_reason 映射
	stopReason := "end_turn"
	switch cr.FinishReason {
	case fieldToolCalls, fieldToolUse:
		stopReason = fieldToolUse
	case "length":
		stopReason = "max_tokens"
	}

	return map[string]any{
		"id":            cr.UpstreamRequestID,
		fieldType:       fieldMessage,
		fieldRole:       roleAssistant,
		fieldContent:    content,
		fieldModel:      modelAlias,
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  cr.InputTokens,
			"output_tokens": cr.OutputTokens,
		},
	}
}

// ---------------------------------------------------------------------------
// Anthropic Protocol: map → ChatRequest
// ---------------------------------------------------------------------------

// convertAnthropicToChatRequest 把 Anthropic Messages JSON 转成 ChatRequest。
//
// 关键差异（vs OpenAI-compatible）：
//   - system 是顶层字段（string 或 [{type:"text"}] 数组），不是 messages[0] role="system"
//   - max_tokens 必填（ChatRequest.MaxTokens）
//   - stop_sequences → ChatRequest.Stop
//   - messages 里没有 role="system"，只有 user / assistant
//   - content blocks 有 type=text / tool_use / tool_result
func convertAnthropicToChatRequest(body map[string]any) (*ChatRequest, error) {
	req := &ChatRequest{}
	req.Model = asString(body[fieldModel])
	req.Stream = asBool(body["stream"])

	// max_tokens → MaxTokens
	if v, ok := body["max_tokens"]; ok {
		if i, err := strconv.Atoi(fmt.Sprint(v)); err == nil {
			req.MaxTokens = &i
		}
	}
	// stop_sequences → Stop
	if v, ok := body["stop_sequences"]; ok {
		if arr, ok := v.([]any); ok {
			for _, e := range arr {
				if s := asString(e); s != "" {
					req.Stop = append(req.Stop, s)
				}
			}
		}
	}

	// system (top-level) — convert to first ChatMessage role=system
	if sys := body[fieldSystem]; sys != nil && sys != "" {
		if s, ok := sys.(string); ok && s != "" {
			req.Messages = append(req.Messages, ChatMessage{
				Role:    fieldSystem,
				Content: s,
			})
		} else if arr, ok := sys.([]any); ok {
			// system 可以是 content blocks
			for _, raw := range arr {
				if b, _ := raw.(map[string]any); b != nil {
					req.Messages = append(req.Messages, ChatMessage{
						Role:    fieldSystem,
						Content: asString(b[fieldText]),
					})
				}
			}
		}
	}

	// messages[] → ChatMessage
	if msgs, ok := body["messages"].([]any); ok {
		for _, raw := range msgs {
			m, _ := raw.(map[string]any)
			if m == nil {
				continue
			}
			role := asString(m[fieldRole])
			contentVal := m[fieldContent]

			switch role {
			case "user", roleAssistant:
				msg := ChatMessage{Role: role, Content: convertAnthropicContentBlocks(contentVal)}
				// assistant 里可能有 tool_use blocks → 转 tool_calls
				// OpenAI 格式 assistant messages 有 tool_calls array
				req.Messages = append(req.Messages, msg)
			case fieldToolResult:
				// Anthropic tool_result 是 messages[] 里的元素，不是 role=tool
				// 但 tool_use blocks 也会出现在 content 里。让 convertAnthropicContentBlocks 处理。
				req.Messages = append(req.Messages, ChatMessage{
					Role:    "tool",
					Content: convertAnthropicContentBlocks(contentVal),
				})
			default:
				req.Messages = append(req.Messages, ChatMessage{Role: role, Content: contentVal})
			}
		}
	}

	// tools → ChatRequest.Tools
	if tools, ok := body["tools"].([]any); ok {
		for _, raw := range tools {
			tm, _ := raw.(map[string]any)
			if tm == nil {
				continue
			}
			req.Tools = append(req.Tools, ChatTool{
				Type: fieldFunction,
				Function: ChatToolFunc{
					Name:        asString(tm[fieldName]),
					Description: asString(tm["description"]),
					Parameters:  asMap(tm["input_schema"]),
				},
			})
		}
	}

	return req, nil
}

// convertAnthropicContentBlocks 把 Anthropic Messages content 块转成
// OpenAI-compatible ChatMessage.Content。
// - [{type:"text",text:"..."}]  → []ChatContentBlock
// - "plain string"              → "plain string"
// - [{type:"tool_use",...}]     → []ChatContentBlock + 下游 ConvertChatRequest 会处理
func convertAnthropicContentBlocks(content any) any {
	switch v := content.(type) {
	case nil:
		return nil
	case string:
		return v
	case []any:
		out := make([]ChatContentBlock, 0, len(v))
		for _, raw := range v {
			b, _ := raw.(map[string]any)
			if b == nil {
				continue
			}
			typ := asString(b[fieldType])
			switch typ {
			case fieldText:
				out = append(out, ChatContentBlock{
					Type: fieldText,
					Text: asString(b[fieldText]),
				})
			case fieldToolUse:
				// tool_use block 会在 assistant message content 里出现 ——
				// 但 OpenAI 格式 assistant message 用 tool_calls 数组表达。
				// 这里保留 block 原样，下游 Canonical 层会正确处理。
				out = append(out, ChatContentBlock{
					Type: fieldToolUse,
					Text: asString(b[fieldName]) + " " + asString(b["input"]),
				})
			case fieldToolResult:
				// tool_result 通常作为独立的 messages[] 条目出现，content 里
				// 放 text block 即可
				out = append(out, ChatContentBlock{
					Type: fieldText,
					Text: asString(b[fieldContent]),
				})
			}
		}
		return out
	default:
		return v
	}
}

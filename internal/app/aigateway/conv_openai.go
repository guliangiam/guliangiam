package aigateway

import (
	"fmt"
	"strconv"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

// ---------------------------------------------------------------------------
// OpenAI Protocol: Request → CanonicalRequest
// ---------------------------------------------------------------------------

// ConvertChatRequest converts an OpenAI ChatRequest into the domain's
// CanonicalRequest form. Handles the full message shape: plain text
// content, multi-block content arrays, tool_calls on assistant messages,
// and tool_call_id on tool-result messages.
func ConvertChatRequest(r ChatRequest) *domain.CanonicalRequest {
	cr := &domain.CanonicalRequest{
		ModelAlias: r.Model,
		Stream:     r.Stream,
	}

	if r.Temperature != nil {
		cr.Temperature = *r.Temperature
	}
	if r.MaxTokens != nil {
		cr.MaxTokens = *r.MaxTokens
	}
	cr.Stop = r.Stop

	for _, m := range r.Messages {
		role := domain.CanonicalMessageRole(strings.ToLower(m.Role))
		if role == "" {
			role = domain.RoleUser
		}
		msg := domain.CanonicalMessage{Role: role, ToolCallID: m.ToolCallID}
		msg.Content = convertContent(m.Content)

		// Assistant tool_calls → tool_use content blocks so the upstream
		// adapter sees them as a single unified content stream.
		for _, tc := range m.ToolCalls {
			msg.Content = append(msg.Content, domain.CanonicalContentBlock{
				Type:       domain.ContentBlockToolUse,
				ToolCallID: tc.ID,
				ToolName:   tc.Function.Name,
				ToolInput:  tc.Function.Parameters,
			})
		}

		cr.Messages = append(cr.Messages, msg)
	}

	for _, t := range r.Tools {
		ct := domain.CanonicalTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		}
		if ct.Name != "" {
			cr.Tools = append(cr.Tools, ct)
		}
	}

	return cr
}

// convertContent normalizes the OpenAI message `content` field into
// []CanonicalContentBlock. Accepts three wire shapes:
//
//   - string → single text block
//   - []any  → each element parsed by its "type" key
//   - nil    → empty slice (e.g. assistant message with only tool_calls)
func convertContent(content any) []domain.CanonicalContentBlock {
	switch v := content.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []domain.CanonicalContentBlock{
			{Type: domain.ContentBlockText, Text: v},
		}
	case []any:
		blocks := make([]domain.CanonicalContentBlock, 0, len(v))
		for _, item := range v {
			b, ok := parseContentBlock(item)
			if ok {
				blocks = append(blocks, b)
			}
		}
		return blocks
	default:
		// 兜底：未知类型 → 转字符串作为 text block
		return []domain.CanonicalContentBlock{
			{Type: domain.ContentBlockText, Text: fmt.Sprintf("%v", v)},
		}
	}
}

// parseContentBlock extracts type/text/image_url from a generic JSON object.
func parseContentBlock(item any) (domain.CanonicalContentBlock, bool) {
	obj, ok := item.(map[string]any)
	if !ok {
		return domain.CanonicalContentBlock{}, false
	}
	typ, _ := obj[fieldType].(string)
	switch typ {
	case fieldText:
		text, _ := obj[fieldText].(string)
		return domain.CanonicalContentBlock{Type: domain.ContentBlockText, Text: text}, true
	case "image_url":
		// image_url.url can be string or {url: "..."}; both forms supported
		var url string
		switch v := obj["image_url"].(type) {
		case string:
			url = v
		case map[string]any:
			url, _ = v["url"].(string)
		}
		return domain.CanonicalContentBlock{Type: domain.ContentBlockImageURL, ImageURL: url}, true
	default:
		// 未识别的 block type 忽略（tool_use / tool_result 不会出现在
		// OpenAI 协议面的 input content 数组里——它们用 tool_calls / tool_call_id 表示）
		return domain.CanonicalContentBlock{}, false
	}
}

// ConvertEmbeddingRequest converts an OpenAI EmbeddingRequest into a
// CanonicalRequest that Core.Execute recognises as an embedding path.
func ConvertEmbeddingRequest(r EmbeddingRequest) *domain.CanonicalRequest {
	return &domain.CanonicalRequest{
		ModelAlias: r.Model,
		Stream:     false,
		Messages: []domain.CanonicalMessage{
			{
				Role: domain.RoleUser,
				Content: []domain.CanonicalContentBlock{
					{Type: domain.ContentBlockText, Text: r.Input},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// OpenAI Protocol: CanonicalResponse → OpenAI response
// ---------------------------------------------------------------------------

// ConvertChatResponse converts CanonicalResponse to the OpenAI ChatResponse shape.
// The `created` timestamp is supplied once by the handler so every choice
// shares the same value — matching OpenAI's wire contract.
func ConvertChatResponse(cr *domain.CanonicalResponse, modelAlias string, created int64) ChatResponse {
	text := ExtractText(cr.Content)
	toolCalls := ExtractToolCalls(cr.Content)

	// OpenAI protocol: message.content is string when there is tool_calls —
	// but we also emit text when both are present (models sometimes emit both).
	var content any
	switch {
	case text == "" && len(toolCalls) > 0:
		content = nil // assistant message with only tool_calls has no content
	default:
		content = text
	}

	choice := ChatChoice{
		Index: 0,
		Message: ChatMessage{
			Role:      roleAssistant,
			Content:   content,
			ToolCalls: toolCalls,
		},
		FinishReason: cr.FinishReason,
	}

	// Canonical 层 tool_result block 的 tool_call_id 映射到 OpenAI response
	// 只有 role=tool 消息需要这个，这里 assistant 响应不带。
	_ = choice

	choices := make([]ChatChoice, 0, 1)
	// 只要有 text 或 tool_calls 就产出 choice —— 空响应不生成
	if text != "" || len(toolCalls) > 0 {
		choices = append(choices, choice)
	}

	return ChatResponse{
		ID:      cr.UpstreamRequestID,
		Object:  "chat.completion",
		Created: created,
		Model:   modelAlias,
		Choices: choices,
		Usage: Usage{
			PromptTokens:     cr.InputTokens,
			CompletionTokens: cr.OutputTokens,
			TotalTokens:      cr.TotalTokens,
		},
	}
}

// ConvertEmbeddingResponse converts CanonicalResponse to the OpenAI embedding
// response shape.
func ConvertEmbeddingResponse(cr *domain.CanonicalResponse, modelAlias string, created int64) EmbeddingResponse {
	data := ExtractEmbeddings(cr.Content)
	return EmbeddingResponse{
		Object: "list",
		Model:  modelAlias,
		Data:   data,
		Usage: EmbeddingUsage{
			PromptTokens: cr.InputTokens,
			TotalTokens:  cr.TotalTokens,
		},
	}
}

// ---------------------------------------------------------------------------
// OpenAI Protocol: CanonicalStreamEvent → SSE chunk
// ---------------------------------------------------------------------------

// DomainEventToSSE converts a CanonicalStreamEvent into an OpenAI-style
// chat.completion.chunk payload. Returns nil when the event does not map to
// a wire chunk (e.g. a keepalive ping).
func DomainEventToSSE(ev *domain.CanonicalStreamEvent, id, model string, created int64) map[string]any {
	if ev == nil {
		return nil
	}

	var delta map[string]any
	var finishReason any

	switch ev.Type {
	case eventContentDelta:
		if ev.ContentDelta != nil && ev.ContentDelta.Type == domain.ContentBlockText {
			delta = map[string]any{fieldContent: ev.ContentDelta.Text}
		}
	case eventToolCallDelta:
		if ev.ToolCallDelta != nil {
			delta = map[string]any{
				fieldToolCalls: []map[string]any{
					{
						fieldIndex: 0,
						"id":       ev.ToolCallDelta.ToolCallID,
						fieldType:  fieldFunction,
						fieldFunction: map[string]any{
							fieldName:   ev.ToolCallDelta.ToolName,
							"arguments": toolInputToJSON(ev.ToolCallDelta.ToolInput),
						},
					},
				},
			}
		}
	case eventMessageDelta:
		if ev.UsageMetadata != nil {
			usage := map[string]any{}
			if v, ok := ev.UsageMetadata["input_tokens"]; ok {
				usage["prompt_tokens"] = v
			}
			if v, ok := ev.UsageMetadata["output_tokens"]; ok {
				usage["completion_tokens"] = v
			}
			if v, ok := ev.UsageMetadata["total_tokens"]; ok {
				usage["total_tokens"] = v
			}
			if len(usage) > 0 {
				return map[string]any{
					"id":       id,
					"object":   "chat.completion.chunk",
					"created":  created,
					fieldModel: model,
					"choices": []map[string]any{
						{
							fieldIndex:      0,
							fieldDelta:      map[string]any{},
							"finish_reason": "stop",
						},
					},
					"usage": usage,
				}
			}
		}
	case eventMessageStop:
		finishReason = "stop"
	}

	if delta == nil && finishReason == nil {
		return nil
	}

	if delta == nil {
		delta = map[string]any{}
	}

	return map[string]any{
		"id":       id,
		"object":   "chat.completion.chunk",
		"created":  created,
		fieldModel: model,
		"choices": []map[string]any{
			{
				fieldIndex:      0,
				fieldDelta:      delta,
				"finish_reason": finishReason,
			},
		},
	}
}

// convertOpenAIMapToChatRequest 把 OpenAI-compatible map 转成 ChatRequest。
// 手动逐字段赋值 + 类型断言 —— 避免 json.Unmarshal 走 DisallowUnknownFields。
func convertOpenAIMapToChatRequest(body map[string]any) (*ChatRequest, error) {
	req := &ChatRequest{}

	req.Model = asString(body[fieldModel])
	req.Stream = asBool(body["stream"])

	if v, ok := body["temperature"]; ok {
		if f, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
			req.Temperature = &f
		}
	}
	if v, ok := body["max_tokens"]; ok {
		if i, err := strconv.Atoi(fmt.Sprint(v)); err == nil {
			req.MaxTokens = &i
		}
	}
	if v, ok := body["stop"]; ok {
		if arr, ok := v.([]any); ok {
			for _, e := range arr {
				if s := asString(e); s != "" {
					req.Stop = append(req.Stop, s)
				}
			}
		}
	}

	if msgs, ok := body["messages"].([]any); ok {
		for _, raw := range msgs {
			m, _ := raw.(map[string]any)
			if m == nil {
				continue
			}
			msg := ChatMessage{
				Role:    asString(m[fieldRole]),
				Content: m[fieldContent],
			}
			if tcs, ok := m[fieldToolCalls].([]any); ok {
				for _, tc := range tcs {
					tcm, _ := tc.(map[string]any)
					if tcm == nil {
						continue
					}
					fn, _ := tcm[fieldFunction].(map[string]any)
					msg.ToolCalls = append(msg.ToolCalls, ChatToolCall{
						ID:   asString(tcm["id"]),
						Type: asString(tcm[fieldType]),
						Function: ChatToolFunc{
							Name:       asString(fn[fieldName]),
							Parameters: asMap(fn["arguments"]),
						},
					})
				}
			}
			if tcid, ok := m["tool_call_id"]; ok {
				msg.ToolCallID = asString(tcid)
			}
			req.Messages = append(req.Messages, msg)
		}
	}

	if tools, ok := body["tools"].([]any); ok {
		for _, raw := range tools {
			tm, _ := raw.(map[string]any)
			if tm == nil {
				continue
			}
			fn, _ := tm[fieldFunction].(map[string]any)
			req.Tools = append(req.Tools, ChatTool{
				Type: asString(tm[fieldType]),
				Function: ChatToolFunc{
					Name:        asString(fn[fieldName]),
					Description: asString(fn["description"]),
					Parameters:  asMap(fn["parameters"]),
				},
			})
		}
	}

	return req, nil
}

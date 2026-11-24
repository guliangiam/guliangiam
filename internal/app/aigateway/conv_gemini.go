package aigateway

import (
	"fmt"
	"strconv"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

// ============================================================================
// Protocol-Native Streaming SSE — Gemini
// ============================================================================
//
// Gemini generateContent Streaming (event: ...  envelope)
//   /text              → {"candidates":[{"content":{"role":"model","parts":[{"text":"..."}]},"index":0}]}
//   /function          → {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"...","args":{}}}]}}]}
//   [DONE] marker for Gemini uses event: [DONE] style too

// DomainEventToGeminiSSE 把 CanonicalStreamEvent 转成 Gemini generateContent
// Streaming event payload。返回空 event="" 表示不产出。
//
// Gemini streaming SSE 比较松散 — 每个 chunk 是一个完整的 candidates 结构，
// event name 可以是任意值（SDK 忽略 event name，只看 data），但我们保持
// 有意义的 event name 方便调试。
func DomainEventToGeminiSSE(ev *domain.CanonicalStreamEvent, model string) (event string, meta map[string]any) {
	if ev == nil {
		return "", nil
	}

	switch ev.Type {
	case eventContentDelta:
		if ev.ContentDelta != nil && ev.ContentDelta.Type == domain.ContentBlockText {
			return fieldContent, map[string]any{
				fieldCandidates: []map[string]any{
					{
						fieldContent: map[string]any{
							fieldRole:  fieldModel,
							fieldParts: []map[string]any{{fieldText: ev.ContentDelta.Text}},
						},
						fieldIndex: 0,
					},
				},
			}
		}
	case eventToolCallDelta:
		if ev.ToolCallDelta != nil {
			return fieldContent, map[string]any{
				fieldCandidates: []map[string]any{
					{
						fieldContent: map[string]any{
							fieldRole: fieldModel,
							fieldParts: []map[string]any{
								{
									"functionCall": map[string]any{
										fieldName: ev.ToolCallDelta.ToolName,
										"args":    ev.ToolCallDelta.ToolInput,
									},
								},
							},
						},
						fieldIndex: 0,
					},
				},
			}
		}
	case eventMessageDelta:
		finishReason := "STOP"
		usageMD := map[string]any{}
		if ev.UsageMetadata != nil {
			if v, ok := ev.UsageMetadata["prompt_token_count"]; ok {
				usageMD["promptTokenCount"] = v
			} else if v, ok := ev.UsageMetadata["input_tokens"]; ok {
				usageMD["promptTokenCount"] = v
			}
			if v, ok := ev.UsageMetadata["output_tokens"]; ok {
				usageMD["candidatesTokenCount"] = v
			}
			if v, ok := ev.UsageMetadata["total_tokens"]; ok {
				usageMD["totalTokenCount"] = v
			}
		}
		candidate := map[string]any{
			"finishReason": finishReason,
			fieldIndex:     0,
		}
		payload := map[string]any{
			fieldCandidates: []map[string]any{candidate},
		}
		if model != "" {
			payload["modelVersion"] = model
		}
		if len(usageMD) > 0 {
			payload["usageMetadata"] = usageMD
		}
		return "finish", payload
	case eventMessageStop:
		return "", nil
	}

	return "", nil
}

// ---------------------------------------------------------------------------
// Gemini Protocol: CanonicalResponse → Gemini generateContent response
// ---------------------------------------------------------------------------

// ConvertChatResponseGemini 把 CanonicalResponse 转成 Gemini generateContent
// 响应格式。
//
// Gemini schema:
//
//	{
//	  "candidates": [{
//	    "content": {
//	      "role": "model",
//	      "parts": [{"text": "..."} | {"functionCall": {...}}]
//	    },
//	    "finishReason": "STOP" | "TOOL_CALL" | "MAX_TOKENS",
//	    "index": 0
//	  }],
//	  "modelVersion": "...",
//	  "usageMetadata": {"promptTokenCount": n, "candidatesTokenCount": n}
//	}
func ConvertChatResponseGemini(cr *domain.CanonicalResponse, modelAlias string) map[string]any {
	parts := make([]map[string]any, 0, 2)

	// text
	if text := ExtractText(cr.Content); text != "" {
		parts = append(parts, map[string]any{fieldText: text})
	}

	// tool_use → functionCall
	for _, b := range cr.Content {
		if b.Type == domain.ContentBlockToolUse {
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{
					fieldName:    b.ToolName,
					"args":       b.ToolInput,
					"toolCallId": b.ToolCallID,
				},
			})
		}
	}

	finishReason := "STOP"
	switch cr.FinishReason {
	case fieldToolCalls, fieldToolUse:
		finishReason = "TOOL_CALL"
	case "length":
		finishReason = "MAX_TOKENS"
	}

	return map[string]any{
		fieldCandidates: []map[string]any{{
			fieldContent: map[string]any{
				fieldRole:  fieldModel,
				fieldParts: parts,
			},
			"finishReason": finishReason,
			fieldIndex:     0,
		}},
		"modelVersion": modelAlias,
		"usageMetadata": map[string]any{
			"promptTokenCount":     cr.InputTokens,
			"candidatesTokenCount": cr.OutputTokens,
			"totalTokenCount":      cr.TotalTokens,
		},
	}
}

// ---------------------------------------------------------------------------
// Gemini Protocol: map → ChatRequest
// ---------------------------------------------------------------------------

// convertGeminiToChatRequest 把 Gemini generateContent JSON 转成 ChatRequest。
//
// 关键差异（vs OpenAI-compatible）：
//   - contents[]（不是 messages[]），role 是 user / model（不是 assistant）
//   - parts[]（不是 content[]），每个 part 有 text / functionCall
//   - systemInstruction.parts[].text → system message
//   - generationConfig.nested (temperature / maxOutputTokens / stopSequences)
func convertGeminiToChatRequest(body map[string]any) (*ChatRequest, error) {
	req := &ChatRequest{}
	req.Model = asString(body[fieldModel])
	req.Stream = asBool(body["stream"])

	// generationConfig nested
	if gc, ok := body["generationConfig"].(map[string]any); ok {
		if v, ok := gc["temperature"]; ok {
			if f, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
				req.Temperature = &f
			}
		}
		if v, ok := gc["maxOutputTokens"]; ok {
			if i, err := strconv.Atoi(fmt.Sprint(v)); err == nil {
				req.MaxTokens = &i
			}
		}
		if v, ok := gc["stopSequences"]; ok {
			if arr, ok := v.([]any); ok {
				for _, e := range arr {
					if s := asString(e); s != "" {
						req.Stop = append(req.Stop, s)
					}
				}
			}
		}
	}

	// systemInstruction
	if si, ok := body["systemInstruction"].(map[string]any); ok {
		if parts, ok := si[fieldParts].([]any); ok {
			for _, raw := range parts {
				if p, _ := raw.(map[string]any); p != nil {
					if txt, ok := p[fieldText].(string); ok && txt != "" {
						req.Messages = append(req.Messages, ChatMessage{
							Role:    fieldSystem,
							Content: txt,
						})
					}
				}
			}
		}
	}

	// contents[] → ChatMessage
	if contents, ok := body["contents"].([]any); ok {
		for _, raw := range contents {
			c, _ := raw.(map[string]any)
			if c == nil {
				continue
			}
			role := asString(c[fieldRole])
			// Gemini "model" role → OpenAI "assistant"
			if role == fieldModel {
				role = roleAssistant
			}

			parts, _ := c[fieldParts].([]any)
			textContent, toolCalls := geminiPartsToContent(parts)

			msg := ChatMessage{
				Role:      role,
				Content:   textContent,
				ToolCalls: toolCalls,
			}
			req.Messages = append(req.Messages, msg)
		}
	}

	// tools — skip complex Gemini ToolDeclaration for now (uncommon path)
	// req.Tools left empty

	return req, nil
}

// geminiPartsToContent 把 Gemini parts[] 转成 OpenAI-compatible text content + tool_calls。
func geminiPartsToContent(parts []any) (any, []ChatToolCall) {
	var sb strings.Builder
	var toolCalls []ChatToolCall

	for _, raw := range parts {
		p, _ := raw.(map[string]any)
		if p == nil {
			continue
		}
		switch {
		case hasKey(p, fieldText):
			sb.WriteString(asString(p[fieldText]))
		case hasKey(p, "functionCall"):
			fc, _ := p["functionCall"].(map[string]any)
			if fc != nil {
				toolCalls = append(toolCalls, ChatToolCall{
					ID:   asString(fc["toolCallId"]),
					Type: fieldFunction,
					Function: ChatToolFunc{
						Name:       asString(fc[fieldName]),
						Parameters: asMap(fc["args"]),
					},
				})
			}
		case hasKey(p, "functionResponse"):
			fresp, _ := p["functionResponse"].(map[string]any)
			if fresp != nil {
				sb.WriteString(asString(fresp["response"]))
			}
		}
	}

	text := sb.String()
	if text == "" {
		return nil, toolCalls
	}
	return text, toolCalls
}

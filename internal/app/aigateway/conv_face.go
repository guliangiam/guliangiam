package aigateway

import (
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ============================================================================
// Protocol-Face Aware Response Converters
// ============================================================================
//
// 设计文档 §7 — AI Gateway 面向三个协议面暴露：
//   openai     → OpenAI chat.completions shape (已有)
//   anthropic  → Anthropic Messages POST /v1/messages shape
//   gemini     → Gemini generateContent shape
//
// GatewayService 在 Canonical 层统一处理请求，Converters 负责把
// CanonicalResponse 转成 protocol-native wire shape。
// 入站请求目前仍走 OpenAI-compatible JSON，出站响应按 face 分流。
// ============================================================================

// ConvertChatResponseForFace 根据 protocol face 路由到对应的原生格式。
// - openai     → ConvertChatResponse (已有行为)
// - anthropic  → ConvertChatResponseAnthropic
// - gemini     → ConvertChatResponseGemini
// 未识别 face fallback 到 OpenAI-compatible — 保持向后兼容。
func ConvertChatResponseForFace(
	face string, cr *domain.CanonicalResponse, modelAlias string, created int64,
) any {
	switch face {
	case "anthropic":
		return ConvertChatResponseAnthropic(cr, modelAlias)
	case "gemini":
		return ConvertChatResponseGemini(cr, modelAlias)
	default:
		return ConvertChatResponse(cr, modelAlias, created)
	}
}

// ============================================================================
// Protocol-Face Aware Request Converters
// ============================================================================
//
// 设计文档 §7 — 入站请求也支持 protocol-native 格式：
//
//   openai     → OpenAI chat.completions JSON (model / messages / max_tokens / ...)
//   anthropic  → Anthropic Messages POST /v1/messages JSON (model / system / messages / max_tokens / stop_sequences / tools)
//   gemini     → Gemini generateContent JSON (model / contents / systemInstruction / generationConfig / tools)
//
// ConvertChatRequestForFace 做两件事：
//   1. 解析原始 JSON body 到 map[string]any
//   2. 按 face + body shape autodetect 选择转换器，产出 ChatRequest
//
// face 决定了 autodetect 的优先顺序。OpenAI face 先尝试 native（避免误判），
// 找不到 protocol-specific 字段再 fallback OpenAI-compatible。
// ============================================================================

// ConvertChatRequestForFace 根据 protocol face 把原始 JSON body 转成 ChatRequest。
// 三个 face 都做 body shape autodetect —— 即使客户端在 anthropic 端点发
// OpenAI-compatible JSON，转换器也能正确识别并处理。
func ConvertChatRequestForFace(face string, bodyBytes []byte) (*ChatRequest, error) {
	if len(bodyBytes) == 0 {
		return nil, apperrors.NewInvalidArgument("empty request body")
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return nil, apperrors.NewInvalidArgumentf("invalid json: %v", err)
	}

	// ---- body shape autodetection ----
	//
	// 唯一特征字段识别:
	//   - contents / systemInstruction / generationConfig → Gemini native
	//   - system (top-level, NOT in messages) + max_tokens + stop_sequences → Anthropic native
	//   - model + messages → OpenAI-compatible (canonical OpenAI shape)

	if hasKey(body, "contents") || hasKey(body, "generationConfig") || hasKey(body, "systemInstruction") {
		return convertGeminiToChatRequest(body)
	}

	// Anthropic: top-level system field OR stop_sequences OR absence of messages role="system"
	if hasKey(body, "max_tokens") || hasKey(body, "stop_sequences") {
		return convertAnthropicToChatRequest(body)
	}
	if sys, ok := body[fieldSystem]; ok && sys != nil && sys != "" {
		// has top-level system → Anthropic native
		return convertAnthropicToChatRequest(body)
	}

	// Default / OpenAI-compatible
	return convertOpenAIMapToChatRequest(body)
}

package aiupstream

// 协议面标识。AdapterFactory 以它们作注册键与 family 回退目标，RouterEngine
// 直接把 provider/路由配置里的 protocolFace 拿来比对，字符串必须逐字一致。
const (
	protocolOpenAI    = "openai"
	protocolAnthropic = "anthropic"
	protocolGemini    = "gemini"
)

// 规范化流事件类型与用量字段名。事件名是对前端的流式契约，用量键是
// CanonicalStreamEvent.UsageMetadata 的读取契约，值不能改。
const (
	eventContentDelta = "content_delta"
	eventMessageDelta = "message_delta"
	eventMessageStop  = "message_stop"

	usageInputTokens  = "input_tokens"
	usageOutputTokens = "output_tokens"
	usageTotalTokens  = "total_tokens"
)

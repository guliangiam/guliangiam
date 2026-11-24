package aigateway

// 协议字段名 / 事件名 / 元数据键在 Anthropic、Gemini、OpenAI 三个协议面之间共享，
// 集中定义为常量，避免同一字面量在多个转换器里重复出现（goconst）。
// 这些字符串是对外协议契约的一部分：修改会直接影响对接方，必须与协议文档同步。
const (
	fieldType       = "type"
	fieldIndex      = "index"
	fieldDelta      = "delta"
	fieldText       = "text"
	fieldContent    = "content"
	fieldModel      = "model"
	fieldMessage    = "message"
	fieldName       = "name"
	fieldFunction   = "function"
	fieldSystem     = "system"
	fieldCandidates = "candidates"
	fieldParts      = "parts"
	fieldToolCalls  = "tool_calls"
	fieldToolUse    = "tool_use"
	fieldToolResult = "tool_result"

	roleAssistant = "assistant"
	fieldRole     = "role"

	eventContentDelta      = "content_delta"
	eventContentBlockDelta = "content_block_delta"
	eventToolCallDelta     = "tool_call_delta"
	eventMessageDelta      = "message_delta"
	eventMessageStop       = "message_stop"
	eventMessageStart      = "message_start"

	metaOrganizationID = "organization_id"
	metaProviderID     = "provider_id"
	metaQuotaScope     = "quota_scope"
	metaEventType      = "event_type"
	metaLatencyMS      = "latency_ms"
	metaTrigger        = "trigger"
	metaProtocolFace   = "protocol_face"
	metaStatus         = "status"
	metaAlias          = "alias"
	metaOrganization   = "organization"
	metaProvider       = "provider"
	metaCallType       = "call_type"
)

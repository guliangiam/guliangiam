// Package aigateway implements the Domain layer of the AI Gateway module.
//
// Canonical is the internal representation that every protocol surface and
// upstream adapter converts to or from. Keeping it in one place means new
// surfaces or upstreams only need one adapter on each side — the Core itself
// never changes.
package aigateway

import (
	"context"
	"fmt"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AttemptState describes the outcome of a single upstream call (first try
// or any failover attempt). The closure is: the upstream picked one of our
// codes through CanonicalAdapter; we never store third-party state literals
// directly.
const (
	AttemptSucceeded = "succeeded"
	AttemptFailed    = "failed"
	AttemptTimeout   = "timeout"
	AttemptUnknown   = "unknown"
)

var ValidAttemptStates = enumset.NewSet("aigatewayAttemptState",
	AttemptSucceeded, AttemptFailed, AttemptTimeout, AttemptUnknown,
)

// CanonicalMessageRole is the closed set of message roles every protocol
// surface normalises into before entering the Core.
type CanonicalMessageRole string

const (
	RoleUser      CanonicalMessageRole = "user"
	RoleAssistant CanonicalMessageRole = "assistant"
	RoleTool      CanonicalMessageRole = "tool"
	RoleSystem    CanonicalMessageRole = "system"
)

// CanonicalContentBlockType is the closed set of content block types. Every
// protocol surface — text, image_url, tool_use, tool_result — collapses into
// one of these four kinds before entering the Core.
type CanonicalContentBlockType string

const (
	ContentBlockText       CanonicalContentBlockType = "text"
	ContentBlockImageURL   CanonicalContentBlockType = "image_url"
	ContentBlockToolUse    CanonicalContentBlockType = "tool_use"
	ContentBlockToolResult CanonicalContentBlockType = "tool_result"
)

// CanonicalContentBlock is a single part of a CanonicalMessage. A message can
// carry text, an image reference, a tool invocation or a tool result.
type CanonicalContentBlock struct {
	Type       CanonicalContentBlockType `json:"type"`
	Text       string                    `json:"text,omitempty"`
	ImageURL   string                    `json:"image_url,omitempty"`
	ToolCallID string                    `json:"tool_call_id,omitempty"`
	ToolName   string                    `json:"tool_name,omitempty"`
	ToolInput  map[string]any            `json:"tool_input,omitempty"`
	ToolResult any                       `json:"tool_result,omitempty"`
}

// CanonicalMessage is one turn in the conversation. Canonical preserves the
// original block shape — a protocol surface adapter collapses multi-block
// messages into one string only when the upstream does not support blocks.
type CanonicalMessage struct {
	Role    CanonicalMessageRole    `json:"role"`
	Content []CanonicalContentBlock `json:"content"`
	// ToolCallID ties a tool result block back to its originating tool_use.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// MCPBinding connects a CanonicalTool declaration to a concrete MCP server
// at runtime. It is NOT persisted — it is injected by the App layer's
// tool_service from the mcpservers domain and lives for one request only.
type MCPBinding struct {
	// McpServerID is the mcpservers domain entity ID.
	McpServerID string
	// ServerName is the MCP server's display name (used for lookup via the
	// mcpservers.Service proxy).
	ServerName string
	// OrgSlug is the organization the MCP server belongs to.
	OrgSlug string
	// ToolName is the actual tool name on the MCP server — may differ from
	// CanonicalTool.Name.
	ToolName string
}

// CanonicalTool is a tool declaration that the model may invoke. The Core
// treats it opaquely — protocol surfaces render it as a tool_use candidate
// and the adapter layer produces the actual MCP call.
type CanonicalTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	// MCPTarget is the runtime binding to a concrete MCP server. It is
	// omitted from JSON serialization (not persisted) because it is
	// injected per-request by the App layer.
	MCPTarget *MCPBinding `json:"-"`
}

// ToolExecutor is a Port interface (Clean Architecture) that abstracts the
// runtime execution of a tool. The App layer's tool_service calls it; the
// concrete implementation is wired at bootstrap (typically mcpservers.Service).
// nil implementations are allowed — the Core skips tool execution silently.
//
// binding is the MCP server target resolved from CanonicalTool.MCPTarget at
// request time — it carries McpServerID / ServerName / ToolName so the
// executor can route to the correct MCP server. binding may be nil when a
// tool has no runtime binding (e.g. declared without MCPTarget) — the
// executor should handle this gracefully (typically skip or return an error).
type ToolExecutor interface {
	// ExecuteTool runs the tool with the given input arguments and returns
	// whatever the tool produced. Errors are returned normally.
	ExecuteTool(ctx context.Context, binding *MCPBinding, toolName string, input map[string]any) (any, error)
}

// ToolChoiceConfig tells the model how to behave around tools: auto picks
// freely, required must call at least one, none forbids them, specific pins
// one tool.
type ToolChoiceMode string

const (
	ToolChoiceAuto     ToolChoiceMode = "auto"
	ToolChoiceRequired ToolChoiceMode = "required"
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceSpecific ToolChoiceMode = "specific"
)

type ToolChoiceConfig struct {
	Mode ToolChoiceMode `json:"mode"`
	Name string         `json:"name,omitempty"` // specific mode only
}

// ThinkingConfig describes the model's extended-thinking mode. Zero value
// means the surface decides.
type ThinkingConfig struct {
	Type         string `json:"type"`          // "enabled" / "disabled"
	BudgetTokens int    `json:"budget_tokens"` // thinking budget
}

// CanonicalRequest is the universal form that every protocol surface handler
// must produce before handing the call to the Core. Upstream adapters read
// this same struct to shape their vendor-specific request.
type CanonicalRequest struct {
	ModelAlias  string             `json:"model_alias"`
	System      string             `json:"system,omitempty"`
	Messages    []CanonicalMessage `json:"messages"`
	Tools       []CanonicalTool    `json:"tools,omitempty"`
	ToolChoice  *ToolChoiceConfig  `json:"tool_choice,omitempty"`
	Stream      bool               `json:"stream"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Stop        []string           `json:"stop,omitempty"`
	Thinking    *ThinkingConfig    `json:"thinking,omitempty"`
}

// CanonicalResponse is the Core's answer. Upstream adapters build it from
// vendor responses; protocol surfaces render it into their caller's format.
type CanonicalResponse struct {
	Content           []CanonicalContentBlock `json:"content"`
	InputTokens       int                     `json:"input_tokens"`
	OutputTokens      int                     `json:"output_tokens"`
	TotalTokens       int                     `json:"total_tokens"`
	FinishReason      string                  `json:"finish_reason,omitempty"`
	UpstreamRequestID string                  `json:"upstream_request_id,omitempty"`
}

// StreamEventError is the terminal event an upstream adapter emits when the
// stream cannot be read to completion. Protocol surfaces translate it into
// their native error frame and must not append a success terminator afterwards.
const StreamEventError = "error"

// CanonicalStreamError is the sanitized failure carried by a StreamEventError.
// It never contains a stack or a raw upstream body: the adapter logs the cause
// and exposes only the application code plus a caller-safe message.
type CanonicalStreamError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CanonicalStreamEvent is the unit of streamed communication. Protocol
// surfaces may emit it as Anthropic-style named events or as OpenAI-style
// data: lines — the stream adapter is the only place that knows.
type CanonicalStreamEvent struct {
	Type          string                 `json:"type"` // message_start / content_delta / tool_call_delta / message_delta / message_stop / ping / error
	ContentDelta  *CanonicalContentBlock `json:"content_delta,omitempty"`
	ToolCallDelta *CanonicalContentBlock `json:"tool_call_delta,omitempty"`
	UsageMetadata map[string]any         `json:"usage_metadata,omitempty"` // tokens
	Error         *CanonicalStreamError  `json:"error,omitempty"`
}

// Validate checks the request has everything the Core needs to proceed.
// Protocol surfaces are responsible for their own pre-validation before
// converting — this guard is for the Core only.
func (r *CanonicalRequest) Validate() error {
	if r.ModelAlias == "" {
		return apperrors.NewInvalidArgument("model_alias is required")
	}
	if len(r.Messages) == 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "messages must not be empty", map[string]any{"field": "messages", "min": 1})
	}
	for i, m := range r.Messages {
		if m.Role == "" {
			return apperrors.NewInvalidArgumentf("messages[%d].role is required", i)
		}
		if len(m.Content) == 0 {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("messages[%d].content must not be empty", i), map[string]any{"field": "messages[i].content", "index": i})
		}
	}
	if r.MaxTokens < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "max_tokens must be non-negative", map[string]any{"field": "max_tokens", "value": r.MaxTokens, "min": 0})
	}
	return nil
}

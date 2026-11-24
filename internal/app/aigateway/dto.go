package aigateway

import domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"

// Domain type re-exports — Transport layer may import these aliases instead
// of reaching directly into internal/domain (Clean Architecture Rule 2).
// ==============================================================

// CanonicalRequest mirrors domain.CanonicalRequest for callers outside the
// domain package — appgateway.ConvertChatRequest returns this type.
type CanonicalRequest = domain.CanonicalRequest

// StreamEvent mirrors domain.CanonicalStreamEvent — the streaming channel
// element type returned by GatewayService.ChatStream.
type StreamEvent = domain.CanonicalStreamEvent

// StreamEventError marks a terminal stream failure. Transport must translate it
// into the protocol-native error frame instead of a success terminator.
const StreamEventError = domain.StreamEventError

// ChatRequest is the OpenAI-compatible chat completion request body.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	Tools       []ChatTool    `json:"tools,omitempty"`
}

// ChatMessage mirrors OpenAI's message shape. Content can be either a plain
// string (simple text) or an array of content blocks (multi-modal / tool_use).
// See ConvertChatRequest for the normalization logic.
type ChatMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content"`
	ToolCalls  []ChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// ChatContentBlock is one content block in a multi-block message content.
// Matches OpenAI's input content block shape — a subset of what we need for
// passthrough to the Canonical layer.
type ChatContentBlock struct {
	Type     string `json:"type"`           // text | image_url | input_image
	Text     string `json:"text,omitempty"` // type=text
	ImageURL struct {
		URL    string `json:"url,omitempty"`
		Detail string `json:"detail,omitempty"`
	} `json:"image_url,omitempty"` // type=image_url
}

// ChatTool is a tool declaration the model may invoke.
type ChatTool struct {
	Type     string       `json:"type"`
	Function ChatToolFunc `json:"function"`
}

type ChatToolFunc struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ChatToolCall represents a tool invocation — either inbound (user echoes a
// tool result via tool_call_id) or outbound (model wants to call a tool).
type ChatToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ChatToolFunc `json:"function"`
}

// EmbeddingRequest mirrors OpenAI's embedding endpoint body.
type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// ChatResponse is the OpenAI-compatible non-streaming response body.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
}

type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// EmbeddingResponse mirrors OpenAI's embedding response shape.
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

type EmbeddingData struct {
	Index     int       `json:"index"`
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ModelsResponse is the OpenAI-compatible GET /models response.
type ModelsResponse struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}

// ModelObject represents one model in the /models list.
type ModelObject struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Created  int64  `json:"created,omitempty"`
	OwnedBy  string `json:"owned_by"`
	Platform string `json:"platform,omitempty"`
}

// AliasSummary is an App-level DTO representing one model alias for the
// GET /models endpoint. Handlers never see domain types.
type AliasSummary struct {
	Alias       string `json:"alias"`
	DisplayName string `json:"display_name"`
	Provider    string `json:"provider"`
	Status      string `json:"status"`
	CreatedUnix int64  `json:"created_unix,omitempty"`
}

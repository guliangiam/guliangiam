package aiupstream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// OpenAICompatAdapter implements domain.UpstreamAdapter for any provider
// that follows the OpenAI HTTP surface (OpenAI proper, DeepSeek, GLM, Kimi,
// Qwen, Seed, Ollama, vLLM, …).
type OpenAICompatAdapter struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	timeout    time.Duration
}

// NewOpenAICompat constructs an adapter pointed at baseURL (e.g.
// https://api.openai.com/v1). apiKey is sent as the Bearer token; use ""
// for local models that don't require authentication.
func NewOpenAICompat(baseURL, apiKey string, timeout time.Duration) *OpenAICompatAdapter {
	return &OpenAICompatAdapter{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: NewSSRFClient(timeout),
		timeout:    timeout,
	}
}

// Name returns the adapter-family identifier.
func (a *OpenAICompatAdapter) Name() string { return "openai_compat" }

// SupportedProtocol returns the protocol face this adapter handles.
func (a *OpenAICompatAdapter) SupportedProtocol() string { return protocolOpenAI }

// Chat sends a non-streaming chat/completions request.
func (a *OpenAICompatAdapter) Chat(ctx context.Context, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	body, err := a.buildChatBody(req, false)
	if err != nil {
		return nil, err
	}
	raw, err := a.doRequest(ctx, http.MethodPost, "/chat/completions", body)
	if err != nil {
		return nil, err
	}
	return parseChatResponse(raw)
}

// ChatStream sends a streaming chat/completions request and emits
// CanonicalStreamEvent on the returned channel.
func (a *OpenAICompatAdapter) ChatStream(ctx context.Context, req *domain.CanonicalRequest) (<-chan *domain.CanonicalStreamEvent, error) {
	body, err := a.buildChatBody(req, true)
	if err != nil {
		return nil, err
	}

	httpReq, err := a.buildHTTPRequest(ctx, http.MethodPost, "/chat/completions", body)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, sanitizeTransportError("openai_compat", "stream request failed", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, parseUpstreamError(resp)
	}

	ch := make(chan *domain.CanonicalStreamEvent, 8)
	recover.SafeGo("ai-openai-sse-consumer", func() { a.consumeSSE(ctx, resp.Body, ch) })
	return ch, nil
}

// Embedding sends an embeddings request and returns a CanonicalResponse
// whose Content slice carries the embedding blocks.
func (a *OpenAICompatAdapter) Embedding(ctx context.Context, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	body, err := a.buildEmbeddingBody(req)
	if err != nil {
		return nil, err
	}
	raw, err := a.doRequest(ctx, http.MethodPost, "/embeddings", body)
	if err != nil {
		return nil, err
	}
	return parseEmbeddingResponse(raw)
}

// ---------------------------------------------------------------------------
// request building
// ---------------------------------------------------------------------------

type openaiMessage struct {
	Role       string               `json:"role"`
	Content    any                  `json:"content"` // string for plain text, []openaiContentPart for blocks
	ToolCallID string               `json:"tool_call_id,omitempty"`
	ToolCalls  []openaiToolCallSpec `json:"tool_calls,omitempty"`
}

type openaiContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type openaiToolCallSpec struct {
	Type     string             `json:"type"`
	Function openaiToolCallFunc `json:"function"`
}

type openaiToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiToolSpec struct {
	Type     string             `json:"type"`
	Function openaiFunctionSpec `json:"function"`
}

type openaiFunctionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type openaiToolChoice struct {
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

type openaiChatBody struct {
	Model       string           `json:"model"`
	Messages    []openaiMessage  `json:"messages"`
	Tools       []openaiToolSpec `json:"tools,omitempty"`
	ToolChoice  any              `json:"tool_choice,omitempty"`
	Stream      bool             `json:"stream"`
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   *int             `json:"max_tokens,omitempty"`
	Stop        []string         `json:"stop,omitempty"`
}

type openaiEmbeddingBody struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

func (a *OpenAICompatAdapter) buildChatBody(req *domain.CanonicalRequest, stream bool) ([]byte, error) {
	if req.ModelAlias == "" {
		return nil, apperrors.NewInvalidArgument("model_alias is required")
	}

	messages := []openaiMessage{}
	if req.System != "" {
		messages = append(messages, openaiMessage{Role: string(domain.RoleSystem), Content: req.System})
	}
	for _, m := range req.Messages {
		om := openaiMessage{Role: string(m.Role), ToolCallID: m.ToolCallID}
		om.Content = blocksToOpenAIContent(m.Content)
		messages = append(messages, om)
	}

	body := openaiChatBody{
		Model:    req.ModelAlias,
		Messages: messages,
		Stream:   stream,
	}

	if len(req.Tools) > 0 {
		tools := make([]openaiToolSpec, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, openaiToolSpec{
				Type: "function",
				Function: openaiFunctionSpec{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
		body.Tools = tools
	}

	if req.ToolChoice != nil {
		body.ToolChoice = openaiToolChoiceFromCanonical(req.ToolChoice)
	}

	if req.Temperature != 0 {
		t := req.Temperature
		body.Temperature = &t
	}
	if req.MaxTokens > 0 {
		m := req.MaxTokens
		body.MaxTokens = &m
	}
	if len(req.Stop) > 0 {
		body.Stop = req.Stop
	}

	return json.Marshal(body)
}

func blocksToOpenAIContent(blocks []domain.CanonicalContentBlock) any {
	// Single text block → send as plain string (simpler, universally supported).
	if len(blocks) == 1 && blocks[0].Type == domain.ContentBlockText {
		return blocks[0].Text
	}
	// Multi-block → send as structured parts.
	parts := make([]openaiContentPart, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case domain.ContentBlockText:
			parts = append(parts, openaiContentPart{Type: string(domain.ContentBlockText), Text: b.Text})
		case domain.ContentBlockImageURL:
			parts = append(parts, openaiContentPart{Type: "image_url", ImageURL: b.ImageURL})
		case domain.ContentBlockToolUse, domain.ContentBlockToolResult:
			// 工具调用/结果由请求体的 tools/tool_calls 承载，不在 content parts 中重复下发。
		}
	}
	return parts
}

func openaiToolChoiceFromCanonical(tc *domain.ToolChoiceConfig) any {
	switch tc.Mode {
	case domain.ToolChoiceNone:
		return "none"
	case domain.ToolChoiceAuto:
		return "auto"
	case domain.ToolChoiceRequired:
		return "required"
	case domain.ToolChoiceSpecific:
		oc := openaiToolChoice{Type: "function"}
		oc.Function.Name = tc.Name
		return oc
	}
	return nil
}

func (a *OpenAICompatAdapter) buildEmbeddingBody(req *domain.CanonicalRequest) ([]byte, error) {
	if req.ModelAlias == "" {
		return nil, apperrors.NewInvalidArgument("model_alias is required")
	}
	// Collapse message content into a single input string.
	var sb strings.Builder
	for _, m := range req.Messages {
		for _, b := range m.Content {
			if b.Text != "" {
				sb.WriteString(b.Text)
				sb.WriteByte('\n')
			}
		}
	}
	input := strings.TrimSpace(sb.String())
	if input == "" {
		return nil, apperrors.NewInvalidArgument("no text to embed")
	}
	return json.Marshal(openaiEmbeddingBody{Model: req.ModelAlias, Input: input})
}

// ---------------------------------------------------------------------------
// HTTP plumbing
// ---------------------------------------------------------------------------

func (a *OpenAICompatAdapter) buildHTTPRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	u, err := url.JoinPath(a.baseURL, path)
	if err != nil {
		return nil, apperrors.NewInvalidArgumentf("openai_compat: bad url: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.NewInternalf("openai_compat: build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if a.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	return req, nil
}

func (a *OpenAICompatAdapter) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	httpReq, err := a.buildHTTPRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, sanitizeTransportError("openai_compat", "request failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, upstreamStatusError(resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.NewUnavailablef("openai_compat: read body: %v", err)
	}
	return raw, nil
}

func parseUpstreamError(resp *http.Response) error {
	return upstreamStatusError(resp.StatusCode)
}

// upstreamStatusError maps an upstream status onto a platform error.
//
// The upstream body is deliberately not reflected. An upstream may echo the
// Authorization header (or Gemini's ?key= query) in its error message, and
// these errors feed llm_usage_logs.error_code and outbox payloads. Only the
// HTTP status and its canonical text are retained, so classification by status
// is unchanged while the body never reaches persistence or API responses.
func upstreamStatusError(status int) error {
	statusText := http.StatusText(status)
	if statusText == "" {
		statusText = "upstream request failed"
	}

	switch status {
	case http.StatusUnauthorized, http.StatusPaymentRequired:
		return apperrors.NewUnauthorizedf("upstream %d: %s", status, statusText)
	case http.StatusForbidden:
		return apperrors.NewForbiddenf("upstream %d: %s", status, statusText)
	case http.StatusTooManyRequests:
		return apperrors.NewTooManyRequestsf("upstream %d: %s", status, statusText)
	case http.StatusUnprocessableEntity, http.StatusBadRequest:
		return apperrors.NewInvalidArgumentf("upstream %d: %s", status, statusText)
	default:
		return apperrors.NewUnavailablef("upstream %d: %s", status, statusText)
	}
}

// ---------------------------------------------------------------------------
// response parsing
// ---------------------------------------------------------------------------

type openaiChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func parseChatResponse(raw []byte) (*domain.CanonicalResponse, error) {
	var resp openaiChatResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, apperrors.NewUnprocessableEntityf("openai_compat: decode response: %v", err)
	}
	out := &domain.CanonicalResponse{
		UpstreamRequestID: resp.ID,
		InputTokens:       resp.Usage.PromptTokens,
		OutputTokens:      resp.Usage.CompletionTokens,
		TotalTokens:       resp.Usage.TotalTokens,
	}
	if len(resp.Choices) > 0 {
		out.FinishReason = resp.Choices[0].FinishReason
		if text := strings.TrimSpace(resp.Choices[0].Message.Content); text != "" {
			out.Content = append(out.Content, domain.CanonicalContentBlock{
				Type: domain.ContentBlockText,
				Text: text,
			})
		}
		for _, tc := range resp.Choices[0].Message.ToolCalls {
			out.Content = append(out.Content, domain.CanonicalContentBlock{
				Type:       domain.ContentBlockToolUse,
				ToolCallID: tc.ID,
				ToolName:   tc.Function.Name,
				ToolInput:  parseJSONMap(tc.Function.Arguments),
			})
		}
	}
	return out, nil
}

func parseJSONMap(s string) map[string]any {
	if s == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

type openaiEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func parseEmbeddingResponse(raw []byte) (*domain.CanonicalResponse, error) {
	var resp openaiEmbeddingResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, apperrors.NewUnprocessableEntityf("openai_compat: decode embedding: %v", err)
	}
	out := &domain.CanonicalResponse{
		InputTokens: resp.Usage.PromptTokens,
		TotalTokens: resp.Usage.TotalTokens,
	}
	if len(resp.Data) > 0 {
		enc, err := json.Marshal(resp.Data[0].Embedding)
		if err == nil {
			out.Content = append(out.Content, domain.CanonicalContentBlock{
				Type: domain.ContentBlockText,
				Text: string(enc),
			})
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// SSE streaming
// ---------------------------------------------------------------------------

func (a *OpenAICompatAdapter) consumeSSE(ctx context.Context, body io.ReadCloser, ch chan<- *domain.CanonicalStreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	// Allow long SSE payloads (some vendors emit large tool-call deltas).
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return
		}

		evt, ok := parseStreamDelta(payload)
		if ok {
			ch <- evt
			if evt.Type == domain.StreamEventError {
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		emitStreamError(ctx, ch, "openai_compat", streamReadErrorMessage, err)
	}
}

type openaiStreamChunk struct {
	Error   json.RawMessage `json:"error"`
	ID      string          `json:"id"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func parseStreamDelta(payload string) (*domain.CanonicalStreamEvent, bool) {
	var chunk openaiStreamChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return nil, false
	}
	if len(chunk.Error) > 0 {
		return streamErrorEvent(streamReportedErrorMessage), true
	}

	// usage block — final or aggregated usage event.
	if chunk.Usage != nil {
		return &domain.CanonicalStreamEvent{
			Type: eventMessageDelta,
			UsageMetadata: map[string]any{
				usageInputTokens:  chunk.Usage.PromptTokens,
				usageOutputTokens: chunk.Usage.CompletionTokens,
				usageTotalTokens:  chunk.Usage.TotalTokens,
			},
		}, true
	}

	if len(chunk.Choices) == 0 {
		return nil, false
	}
	c := chunk.Choices[0]

	// finish_reason non-nil (even if null) → stop event.
	if c.FinishReason != nil {
		return &domain.CanonicalStreamEvent{Type: eventMessageStop}, true
	}

	if c.Delta.Content != "" {
		return &domain.CanonicalStreamEvent{
			Type:         eventContentDelta,
			ContentDelta: &domain.CanonicalContentBlock{Type: domain.ContentBlockText, Text: c.Delta.Content},
		}, true
	}

	return nil, false
}

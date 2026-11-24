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

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com"
	anthropicAPIVersion     = "2023-06-01"
)

// AnthropicMessagesAdapter implements domain.UpstreamAdapter for the
// Anthropic Messages API (v1/messages).
type AnthropicMessagesAdapter struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	timeout    time.Duration
}

// NewAnthropicMessages constructs an adapter pointed at baseURL (e.g.
// https://api.anthropic.com). apiKey is sent as the x-api-key header.
func NewAnthropicMessages(baseURL, apiKey string, timeout time.Duration) *AnthropicMessagesAdapter {
	if baseURL == "" {
		baseURL = anthropicDefaultBaseURL
	}
	return &AnthropicMessagesAdapter{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: NewSSRFClient(timeout),
		timeout:    timeout,
	}
}

func (a *AnthropicMessagesAdapter) Name() string              { return protocolAnthropic }
func (a *AnthropicMessagesAdapter) SupportedProtocol() string { return protocolAnthropic }

// Chat sends a non-streaming Messages API request.
func (a *AnthropicMessagesAdapter) Chat(ctx context.Context, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	body, err := a.buildMessagesBody(req, false)
	if err != nil {
		return nil, err
	}
	raw, err := a.doRequest(ctx, http.MethodPost, "/v1/messages", body)
	if err != nil {
		return nil, err
	}
	return parseAnthropicMessageResponse(raw)
}

// ChatStream sends a streaming Messages API request and emits
// CanonicalStreamEvent on the returned channel.
func (a *AnthropicMessagesAdapter) ChatStream(ctx context.Context, req *domain.CanonicalRequest) (<-chan *domain.CanonicalStreamEvent, error) {
	body, err := a.buildMessagesBody(req, true)
	if err != nil {
		return nil, err
	}

	httpReq, err := a.buildHTTPRequest(ctx, http.MethodPost, "/v1/messages", body)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, sanitizeTransportError("anthropic", "stream request failed", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, parseUpstreamError(resp)
	}

	ch := make(chan *domain.CanonicalStreamEvent, 8)
	recover.SafeGo("ai-anthropic-sse-consumer", func() { a.consumeAnthropicSSE(ctx, resp.Body, ch) })
	return ch, nil
}

// Embedding — Anthropic does not ship a native embeddings endpoint.
// Callers should route embedding traffic through an OpenAI-compatible
// provider instead.
func (a *AnthropicMessagesAdapter) Embedding(ctx context.Context, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	return nil, apperrors.NewUnimplemented("anthropic: embeddings not supported; use an openai-compatible provider")
}

// ---------------------------------------------------------------------------
// request building
// ---------------------------------------------------------------------------

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicMessagesBody struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream"`
	// Thinking budget — 传了就开启 extended thinking。
	Thinking *struct {
		Type         string `json:"type"`
		BudgetTokens int    `json:"budget_tokens"`
	} `json:"thinking,omitempty"`
}

func (a *AnthropicMessagesAdapter) buildMessagesBody(req *domain.CanonicalRequest, stream bool) ([]byte, error) {
	if req.ModelAlias == "" {
		return nil, apperrors.NewInvalidArgument("model_alias is required")
	}

	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		am := anthropicMessage{Role: string(m.Role)}
		for _, b := range m.Content {
			if b.Type == domain.ContentBlockText {
				am.Content = append(am.Content, anthropicContentBlock{Type: string(domain.ContentBlockText), Text: b.Text})
			}
		}
		// Anthropic requires messages to alternate user/assistant.
		if len(am.Content) == 0 {
			continue
		}
		messages = append(messages, am)
	}

	body := anthropicMessagesBody{
		Model:     req.ModelAlias,
		System:    req.System,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
		Stream:    stream,
	}
	// Anthropic requires max_tokens — default to 1024 if unset.
	if body.MaxTokens <= 0 {
		body.MaxTokens = 1024
	}

	if req.Thinking != nil && req.Thinking.Type == "enabled" && req.Thinking.BudgetTokens > 0 {
		body.Thinking = &struct {
			Type         string `json:"type"`
			BudgetTokens int    `json:"budget_tokens"`
		}{Type: "enabled", BudgetTokens: req.Thinking.BudgetTokens}
	}

	return json.Marshal(body)
}

// ---------------------------------------------------------------------------
// HTTP plumbing
// ---------------------------------------------------------------------------

func (a *AnthropicMessagesAdapter) buildHTTPRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	u, err := url.JoinPath(a.baseURL, path)
	if err != nil {
		return nil, apperrors.NewInvalidArgumentf("anthropic: bad url: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.NewInternalf("anthropic: build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	if a.apiKey != "" {
		req.Header.Set("x-api-key", a.apiKey)
	}
	return req, nil
}

func (a *AnthropicMessagesAdapter) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	httpReq, err := a.buildHTTPRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, sanitizeTransportError("anthropic", "request failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, upstreamStatusError(resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.NewUnavailablef("anthropic: read body: %v", err)
	}
	return raw, nil
}

// ---------------------------------------------------------------------------
// response parsing
// ---------------------------------------------------------------------------

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicMessageResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage          `json:"usage"`
}

func parseAnthropicMessageResponse(raw []byte) (*domain.CanonicalResponse, error) {
	var resp anthropicMessageResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, apperrors.NewUnprocessableEntityf("anthropic: decode response: %v", err)
	}
	out := &domain.CanonicalResponse{
		UpstreamRequestID: resp.ID,
		InputTokens:       resp.Usage.InputTokens,
		OutputTokens:      resp.Usage.OutputTokens,
		TotalTokens:       resp.Usage.InputTokens + resp.Usage.OutputTokens,
		FinishReason:      resp.StopReason,
	}
	for _, c := range resp.Content {
		if c.Type == string(domain.ContentBlockText) && c.Text != "" {
			out.Content = append(out.Content, domain.CanonicalContentBlock{
				Type: domain.ContentBlockText,
				Text: c.Text,
			})
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// SSE streaming
// ---------------------------------------------------------------------------

func (a *AnthropicMessagesAdapter) consumeAnthropicSSE(ctx context.Context, body io.ReadCloser, ch chan<- *domain.CanonicalStreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	var currentEvent string
	var dataLines []string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()

		// Anthropic SSE format:
		//   event: <event-name>
		//   data: <json>
		//   (blank line separates events)
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			dataLines = nil
			continue
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			dataLines = append(dataLines, payload)
			continue
		}
		// Blank line → flush accumulated event + data.
		if strings.TrimSpace(line) == "" {
			if currentEvent == "" || len(dataLines) == 0 {
				continue
			}
			if currentEvent == "error" {
				emitStreamError(ctx, ch, "anthropic", streamReportedErrorMessage, nil)
				return
			}
			payload := strings.Join(dataLines, "\n")
			evt, ok := parseAnthropicStreamEvent(currentEvent, payload)
			if ok {
				ch <- evt
			}
			currentEvent = ""
			dataLines = nil
		}
	}

	if currentEvent == "error" {
		emitStreamError(ctx, ch, "anthropic", streamReportedErrorMessage, nil)
		return
	}
	if err := scanner.Err(); err != nil {
		emitStreamError(ctx, ch, "anthropic", streamReadErrorMessage, err)
	}
}

func parseAnthropicStreamEvent(event, payload string) (*domain.CanonicalStreamEvent, bool) {
	switch event {
	case "message_start":
		return &domain.CanonicalStreamEvent{Type: "message_start"}, true

	case "content_block_delta":
		var chunk struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return nil, false
		}
		if chunk.Delta.Text != "" {
			return &domain.CanonicalStreamEvent{
				Type:         eventContentDelta,
				ContentDelta: &domain.CanonicalContentBlock{Type: domain.ContentBlockText, Text: chunk.Delta.Text},
			}, true
		}
		return nil, false

	case eventMessageDelta:
		var chunk struct {
			Usage *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil || chunk.Usage == nil {
			return nil, false
		}
		return &domain.CanonicalStreamEvent{
			Type: eventMessageDelta,
			UsageMetadata: map[string]any{
				usageInputTokens:  chunk.Usage.InputTokens,
				usageOutputTokens: chunk.Usage.OutputTokens,
				usageTotalTokens:  chunk.Usage.InputTokens + chunk.Usage.OutputTokens,
			},
		}, true

	case eventMessageStop:
		return &domain.CanonicalStreamEvent{Type: eventMessageStop}, true
	}
	return nil, false
}

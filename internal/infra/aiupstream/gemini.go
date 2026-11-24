package aiupstream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	geminiDefaultBaseURL = "https://generativelanguage.googleapis.com"
	geminiAPIPrefix      = "/v1beta/models"
)

// GeminiGenerateContentAdapter implements domain.UpstreamAdapter for the
// Google Gemini generateContent / streamGenerateContent APIs.
type GeminiGenerateContentAdapter struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	timeout    time.Duration
}

// NewGeminiGenerateContent constructs an adapter pointed at baseURL (e.g.
// https://generativelanguage.googleapis.com). apiKey is passed as the
// ?key= query parameter on every request.
func NewGeminiGenerateContent(baseURL, apiKey string, timeout time.Duration) *GeminiGenerateContentAdapter {
	if baseURL == "" {
		baseURL = geminiDefaultBaseURL
	}
	return &GeminiGenerateContentAdapter{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: NewSSRFClient(timeout),
		timeout:    timeout,
	}
}

func (a *GeminiGenerateContentAdapter) Name() string              { return protocolGemini }
func (a *GeminiGenerateContentAdapter) SupportedProtocol() string { return protocolGemini }

// Chat sends a non-streaming generateContent request.
func (a *GeminiGenerateContentAdapter) Chat(ctx context.Context, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	if req.ModelAlias == "" {
		return nil, apperrors.NewInvalidArgument("model_alias is required")
	}
	body, err := a.buildGenerateContentBody(req)
	if err != nil {
		return nil, err
	}
	// Gemini model 名在路径里：POST /v1beta/models/{model}:generateContent?key=...
	path := fmt.Sprintf("%s/%s:generateContent", geminiAPIPrefix, req.ModelAlias)
	raw, err := a.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	return parseGeminiGenerateContentResponse(raw)
}

// ChatStream sends a streaming streamGenerateContent request and emits
// CanonicalStreamEvent on the returned channel.
//
// Gemini stream returns NDJSON (one JSON object per line), not SSE —
// the scanner reads line-delimited JSON chunks.
func (a *GeminiGenerateContentAdapter) ChatStream(ctx context.Context, req *domain.CanonicalRequest) (<-chan *domain.CanonicalStreamEvent, error) {
	if req.ModelAlias == "" {
		return nil, apperrors.NewInvalidArgument("model_alias is required")
	}
	body, err := a.buildGenerateContentBody(req)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/%s:streamGenerateContent", geminiAPIPrefix, req.ModelAlias)

	httpReq, err := a.buildHTTPRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		// Do not wrap err with %v: *url.Error embeds the full request URL and
		// Gemini carries the plaintext API key in ?key=.
		return nil, sanitizeTransportError("gemini", "stream request failed", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, parseUpstreamError(resp)
	}

	ch := make(chan *domain.CanonicalStreamEvent, 8)
	recover.SafeGo("ai-gemini-ndjson-consumer", func() { a.consumeGeminiNDJSON(ctx, resp.Body, ch) })
	return ch, nil
}

// Embedding — Gemini has a dedicated embeddings endpoint
// (/v1beta/models/{model}:embedContent) but Round 7 focuses on chat.
// Return unimplemented for now so callers route to an OpenAI-compatible
// provider.
func (a *GeminiGenerateContentAdapter) Embedding(ctx context.Context, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	return nil, apperrors.NewUnimplemented("gemini: embeddings not implemented in Round 7; use an openai-compatible provider")
}

// ---------------------------------------------------------------------------
// request building
// ---------------------------------------------------------------------------

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}
type geminiGenerateContentBody struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

func (a *GeminiGenerateContentAdapter) buildGenerateContentBody(req *domain.CanonicalRequest) ([]byte, error) {
	if req.ModelAlias == "" {
		return nil, apperrors.NewInvalidArgument("model_alias is required")
	}

	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		gc := geminiContent{Role: mapCanonicalRoleToGemini(string(m.Role))}
		for _, b := range m.Content {
			if b.Type == domain.ContentBlockText {
				gc.Parts = append(gc.Parts, geminiPart{Text: b.Text})
			}
		}
		if len(gc.Parts) == 0 {
			continue
		}
		contents = append(contents, gc)
	}

	body := geminiGenerateContentBody{Contents: contents}

	// System prompt → systemInstruction (Gemini accepts role "system" content block too,
	// but systemInstruction is the clearer dedicated field).
	if req.System != "" {
		body.SystemInstruction = &geminiContent{
			Role:  string(domain.RoleSystem),
			Parts: []geminiPart{{Text: req.System}},
		}
	}

	if req.MaxTokens > 0 || req.Temperature > 0 {
		body.GenerationConfig = &geminiGenerationConfig{}
		if req.MaxTokens > 0 {
			body.GenerationConfig.MaxOutputTokens = req.MaxTokens
		}
		if req.Temperature > 0 {
			body.GenerationConfig.Temperature = req.Temperature
		}
	}

	return json.Marshal(body)
}

// mapCanonicalRoleToGemini maps CanonicalMessageRole to the subset Gemini
// supports ("user", "model"). "assistant" → "model" per Gemini 1.5+ API.
func mapCanonicalRoleToGemini(role string) string {
	switch role {
	case "assistant":
		return "model"
	case string(domain.RoleSystem):
		// system 消息在 Gemini 中作为 systemInstruction 发出；这里仍可能
		// 收到混合消息列表中的 system block，保守映射回 user 避免 API 报错。
		return "user"
	default:
		return role
	}
}

// ---------------------------------------------------------------------------
// HTTP plumbing
// ---------------------------------------------------------------------------

func (a *GeminiGenerateContentAdapter) buildHTTPRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	u, err := url.JoinPath(a.baseURL, path)
	if err != nil {
		return nil, apperrors.NewInvalidArgumentf("gemini: bad url: %v", err)
	}
	// Gemini 用 query param ?key= 传递 API key。
	if a.apiKey != "" {
		parsed, _ := url.Parse(u)
		q := parsed.Query()
		q.Set("key", a.apiKey)
		parsed.RawQuery = q.Encode()
		u = parsed.String()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.NewInternalf("gemini: build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (a *GeminiGenerateContentAdapter) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	httpReq, err := a.buildHTTPRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		// Same reason as ChatStream: never render the *url.Error, whose URL
		// contains ?key=<plaintext>.
		return nil, sanitizeTransportError("gemini", "request failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, upstreamStatusError(resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.NewUnavailablef("gemini: read body: %v", err)
	}
	return raw, nil
}

// ---------------------------------------------------------------------------
// response parsing
// ---------------------------------------------------------------------------

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiGenerateContentResponse struct {
	Error         json.RawMessage     `json:"error"`
	Candidates    []geminiCandidate   `json:"candidates"`
	UsageMetadata geminiUsageMetadata `json:"usageMetadata"`
}

func parseGeminiGenerateContentResponse(raw []byte) (*domain.CanonicalResponse, error) {
	var resp geminiGenerateContentResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, apperrors.NewUnprocessableEntityf("gemini: decode response: %v", err)
	}

	out := &domain.CanonicalResponse{
		InputTokens:  resp.UsageMetadata.PromptTokenCount,
		OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:  resp.UsageMetadata.TotalTokenCount,
	}
	if len(resp.Candidates) > 0 {
		out.FinishReason = geminiFinishReasonToCanonical(resp.Candidates[0].FinishReason)
		for _, p := range resp.Candidates[0].Content.Parts {
			if p.Text != "" {
				out.Content = append(out.Content, domain.CanonicalContentBlock{
					Type: domain.ContentBlockText,
					Text: p.Text,
				})
			}
		}
	}
	return out, nil
}

// geminiFinishReasonToCanonical maps Gemini finish reason literals to
// OpenAI-ish values we store as CanonicalResponse.FinishReason.
func geminiFinishReasonToCanonical(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	case "OTHER":
		return "other"
	default:
		return reason
	}
}

// ---------------------------------------------------------------------------
// NDJSON streaming
// ---------------------------------------------------------------------------

func (a *GeminiGenerateContentAdapter) consumeGeminiNDJSON(ctx context.Context, body io.ReadCloser, ch chan<- *domain.CanonicalStreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		evt, ok := parseGeminiStreamChunk(line)
		if ok {
			ch <- evt
			if evt.Type == domain.StreamEventError {
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		emitStreamError(ctx, ch, "gemini", streamReadErrorMessage, err)
	}
}

func parseGeminiStreamChunk(line string) (*domain.CanonicalStreamEvent, bool) {
	var chunk geminiGenerateContentResponse
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return nil, false
	}
	if len(chunk.Error) > 0 {
		return streamErrorEvent(streamReportedErrorMessage), true
	}

	// usageMetadata 可能在某个 chunk 里单独出现或作为 candidate chunk 的附加字段。
	if chunk.UsageMetadata.PromptTokenCount > 0 || chunk.UsageMetadata.TotalTokenCount > 0 {
		return &domain.CanonicalStreamEvent{
			Type: eventMessageDelta,
			UsageMetadata: map[string]any{
				usageInputTokens:  chunk.UsageMetadata.PromptTokenCount,
				usageOutputTokens: chunk.UsageMetadata.CandidatesTokenCount,
				usageTotalTokens:  chunk.UsageMetadata.TotalTokenCount,
			},
		}, true
	}

	if len(chunk.Candidates) == 0 {
		return nil, false
	}
	c := chunk.Candidates[0]

	// finishReason 非空 → stop event.
	if c.FinishReason != "" {
		return &domain.CanonicalStreamEvent{Type: eventMessageStop}, true
	}

	for _, p := range c.Content.Parts {
		if p.Text != "" {
			return &domain.CanonicalStreamEvent{
				Type:         eventContentDelta,
				ContentDelta: &domain.CanonicalContentBlock{Type: domain.ContentBlockText, Text: p.Text},
			}, true
		}
	}
	return nil, false
}

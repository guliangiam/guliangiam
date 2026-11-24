package aigateway

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// Handler carries an AI Gateway Core service and exposes HTTP handlers
// that convert OpenAI-compatible surfaces to CanonicalRequest.
// Transport layer never imports the domain package — it talks only to
// App-layer DTOs and converter functions.
type Handler struct {
	Service *appgateway.GatewayService
}

// NewHandler builds a Handler.
func NewHandler(service *appgateway.GatewayService) Handler {
	return Handler{Service: service}
}

// handleChat is the POST /api/v1/ai/faces/openai/chat/completions handler.
// It dispatches to the streaming or non-streaming path based on the request's
// `stream` flag. Response always returns OpenAI-compatible chat.completions shape.
//
// Anthropic/Gemini 协议面由 chi.go 指向 handleChatAnthropic / handleChatGemini ——
// 它们共享同一个内部 handleChatForFace 逻辑，但用 protocol-native 响应格式。
func (h Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	h.handleChatForFace("openai", w, r)
}

// handleChatAnthropic 处理 POST /api/v1/ai/faces/anthropic/chat/completions。
// 入站请求仍走 OpenAI-compatible JSON（Canonical 层统一），
// 出站响应转为 Anthropic Messages POST /v1/messages 原生格式。
func (h Handler) handleChatAnthropic(w http.ResponseWriter, r *http.Request) {
	h.handleChatForFace("anthropic", w, r)
}

// handleChatGemini 处理 POST /api/v1/ai/faces/gemini/chat/completions。
// 入站请求 OpenAI-compatible，出站响应转为 Gemini generateContent 原生格式。
func (h Handler) handleChatGemini(w http.ResponseWriter, r *http.Request) {
	h.handleChatForFace("gemini", w, r)
}

// handleChatForFace 是三个协议面共用的内部实现。
// face 决定了出站响应的原生格式：openai → chat.completions,
// anthropic → Messages shape, gemini → generateContent shape。
func (h Handler) handleChatForFace(face string, w http.ResponseWriter, r *http.Request) {
	// Read body once — then let ConvertChatRequestForFace do protocol autodetect
	bodyBytes, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		// route.WriteError maps the bootstrap MaxBytesReader error to 413; wrapping
		// it as internal here would turn an oversized request body into a 500.
		route.WriteError(w, r, err)
		return
	}

	req, err := appgateway.ConvertChatRequestForFace(face, bodyBytes)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	apiKeyHash, ok := appgateway.ExtractAPIKeyHash(
		r.Header.Get("Authorization"),
		r.Header.Get("x-api-key"),
	)
	if !ok {
		route.WriteError(w, r, errAPIKeyRequired())
		return
	}

	if req.Stream {
		h.streamChatForFace(face, w, r, apiKeyHash, *req)
		return
	}

	canonical := appgateway.ConvertChatRequest(*req)
	resp, err := h.Service.Chat(r.Context(), apiKeyHash, req.Model, canonical)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	created := time.Now().Unix()
	payload := appgateway.ConvertChatResponseForFace(face, resp, req.Model, created)
	route.WriteJSON(w, http.StatusOK, payload)
}

// handleEmbeddings is the POST /api/v1/ai/faces/openai/embeddings handler.
func (h Handler) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	var req appgateway.EmbeddingRequest
	if err := route.DecodeJSON(r, &req); err != nil {
		route.WriteError(w, r, err)
		return
	}

	apiKeyHash, ok := appgateway.ExtractAPIKeyHash(
		r.Header.Get("Authorization"),
		r.Header.Get("x-api-key"),
	)
	if !ok {
		route.WriteError(w, r, errAPIKeyRequired())
		return
	}

	canonical := appgateway.ConvertEmbeddingRequest(req)

	resp, err := h.Service.Embedding(r.Context(), apiKeyHash, req.Model, canonical)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	created := time.Now().Unix()
	embResp := appgateway.ConvertEmbeddingResponse(resp, req.Model, created)
	route.WriteJSON(w, http.StatusOK, embResp)
}

// handleModels is the GET /api/v1/ai/faces/openai/models handler.
// We enumerate our aliases — each alias becomes one "model" from the
// caller's perspective. Auth flows through API Key → resolve org_id → list aliases.
func (h Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	apiKeyHash, ok := appgateway.ExtractAPIKeyHash(
		r.Header.Get("Authorization"),
		r.Header.Get("x-api-key"),
	)
	if !ok {
		route.WriteError(w, r, errAPIKeyRequired())
		return
	}

	summaries, err := h.Service.ListAliasesForAPIKey(r.Context(), apiKeyHash)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	data := make([]appgateway.ModelObject, 0, len(summaries))
	for _, s := range summaries {
		data = append(data, newModelObject(s))
	}

	route.WriteJSON(w, http.StatusOK, appgateway.ModelsResponse{
		Object: "list",
		Data:   data,
	})
}

// streamChatForFace handles the stream=true case for all three protocol faces.
// 每个 face 用各自原生 SSE envelope —— handler 内部按 face 分流：
//
//	openai     → OpenAI-style SSE  (data: {json}\n\n + data: [DONE])
//	anthropic  → Anthropic Messages Streaming (event: xxx\ndata: {json}\n\n)
//	gemini     → Gemini generateContent Streaming (event: xxx\ndata: {json}\n\n)
func (h Handler) streamChatForFace(
	face string,
	w http.ResponseWriter,
	r *http.Request,
	apiKeyHash string,
	req appgateway.ChatRequest,
) {
	canonical := appgateway.ConvertChatRequest(req)
	alias := req.Model

	evCh, err := h.Service.ChatStream(r.Context(), apiKeyHash, alias, canonical)
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httperr.WriteError(w, r, apperrors.NewInternal("streaming not supported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")

	created := time.Now().Unix()
	msgID := "chatcmpl-" + time.Now().Format("20060102150405")

	switch face {
	case "anthropic":
		streamAnthropic(w, flusher, evCh, msgID, alias)
	case "gemini":
		streamGemini(w, flusher, evCh, alias)
	default:
		// openai + unknown fallback
		streamOpenAI(w, flusher, evCh, msgID, alias, created)
	}
}

// ---- Face-specific streaming loops ----

func streamOpenAI(
	w http.ResponseWriter,
	flusher http.Flusher,
	evCh <-chan *appgateway.StreamEvent,
	firstID, alias string,
	created int64,
) {
	writeSSE(w, map[string]any{
		"id":      firstID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   alias,
		"choices": []map[string]any{
			{
				"index":         0,
				"delta":         map[string]any{"role": "assistant"},
				"finish_reason": nil,
			},
		},
	})
	flusher.Flush()

	for ev := range evCh {
		if ev.Type == appgateway.StreamEventError {
			writeOpenAIStreamError(w, ev)
			flusher.Flush()
			return
		}
		data := appgateway.DomainEventToSSE(ev, firstID, alias, created)
		if data == nil {
			continue
		}
		writeSSE(w, data)
		flusher.Flush()
	}

	writeDone(w)
	flusher.Flush()
}

func streamAnthropic(
	w http.ResponseWriter,
	flusher http.Flusher,
	evCh <-chan *appgateway.StreamEvent,
	msgID, model string,
) {
	// 首帧 message_start + content_block_start
	writeSSEEvent(w, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":    msgID,
			"type":  "message",
			"role":  "assistant",
			"model": model,
		},
	})
	flusher.Flush()

	writeSSEEvent(w, "content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         0,
		"content_block": map[string]any{"type": "text", "text": ""},
	})
	flusher.Flush()

	for ev := range evCh {
		if ev.Type == appgateway.StreamEventError {
			writeAnthropicStreamError(w, ev)
			flusher.Flush()
			return
		}
		evt, payload := appgateway.DomainEventToAnthropicSSE(ev, msgID, model)
		if evt == "" || payload == nil {
			continue
		}
		writeSSEEvent(w, evt, payload)
		flusher.Flush()
	}

	// 尾帧 content_block_stop + message_stop
	writeSSEEvent(w, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": 0,
	})
	flusher.Flush()

	writeSSEEvent(w, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": "end_turn"},
	})
	flusher.Flush()

	writeSSEEvent(w, "message_stop", map[string]any{
		"type": "message_stop",
	})
	flusher.Flush()
}

func streamGemini(
	w http.ResponseWriter,
	flusher http.Flusher,
	evCh <-chan *appgateway.StreamEvent,
	model string,
) {
	for ev := range evCh {
		if ev.Type == appgateway.StreamEventError {
			writeGeminiStreamError(w, ev)
			flusher.Flush()
			return
		}
		evt, payload := appgateway.DomainEventToGeminiSSE(ev, model)
		if evt == "" || payload == nil {
			continue
		}
		writeSSEEvent(w, evt, payload)
		flusher.Flush()
	}

	// Gemini 没有强制 terminal SSE 帧 — 写一个 [DONE] 兼容处理
	writeGeminiDone(w)
	flusher.Flush()
}

// ---- SSE writers ----

// writeSSE 写一个 OpenAI-style SSE 事件（无 event name）：`data: {json}\n\n`
func writeSSE(w http.ResponseWriter, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(body)
	_, _ = w.Write([]byte("\n\n"))
}

// writeSSEEvent 写一个显式 event name 的 SSE 事件：`event: {name}\ndata: {json}\n\n`
func writeSSEEvent(w http.ResponseWriter, eventName string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("event: "))
	_, _ = w.Write([]byte(eventName))
	_, _ = w.Write([]byte("\n"))
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(body)
	_, _ = w.Write([]byte("\n\n"))
}

// streamErrorDetails resolves the code/message carried by a terminal stream
// error. The adapter owns sanitization; the fallback keeps a malformed event
// from producing an empty error object.
func streamErrorDetails(ev *appgateway.StreamEvent) (code, message string) {
	code = string(apperrors.CodeUnavailable)
	message = "upstream stream interrupted"
	if ev != nil && ev.Error != nil {
		if ev.Error.Code != "" {
			code = ev.Error.Code
		}
		if ev.Error.Message != "" {
			message = ev.Error.Message
		}
	}
	return code, message
}

// writeOpenAIStreamError emits the OpenAI-compatible error chunk. It is not
// followed by data: [DONE]: a client must not read a failed stream as success.
func writeOpenAIStreamError(w http.ResponseWriter, ev *appgateway.StreamEvent) {
	code, message := streamErrorDetails(ev)
	writeSSE(w, map[string]any{
		"error": map[string]any{"code": code, "message": message},
	})
}

// writeAnthropicStreamError emits the native `event: error` frame. `type` is
// Anthropic's discriminator; `code`/`message` are the shared structure used by
// the other faces.
func writeAnthropicStreamError(w http.ResponseWriter, ev *appgateway.StreamEvent) {
	code, message := streamErrorDetails(ev)
	writeSSEEvent(w, "error", map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "api_error",
			"code":    code,
			"message": message,
		},
	})
}

// writeGeminiStreamError emits a named error frame. It replaces the usual
// [DONE] marker so truncated generation is never reported as a completed one.
func writeGeminiStreamError(w http.ResponseWriter, ev *appgateway.StreamEvent) {
	code, message := streamErrorDetails(ev)
	writeSSEEvent(w, "error", map[string]any{
		"error": map[string]any{"code": code, "message": message},
	})
}

// writeDone sends the OpenAI-style terminal SSE marker.
func writeDone(w http.ResponseWriter) {
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
}

// writeGeminiDone sends a terminal marker for Gemini streaming — SDK 不强制，
// 但留一个明确标记方便调试。
func writeGeminiDone(w http.ResponseWriter) {
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
}

// newModelObject builds a ModelObject from an App-level AliasSummary.
func newModelObject(s appgateway.AliasSummary) appgateway.ModelObject {
	ownedBy := "guliangiam"
	if s.DisplayName != "" {
		ownedBy = s.DisplayName
	}
	return appgateway.ModelObject{
		ID:       s.Alias,
		Object:   "model",
		Created:  s.CreatedUnix,
		OwnedBy:  ownedBy,
		Platform: "openai-compatible",
	}
}

// errAPIKeyRequired 统一走 apperrors：本地 error 类型缺少 HTTPStatus()/ProtocolCode()，
// route.WriteError 会把它当未知错误渲染成 500 而不是 401。
func errAPIKeyRequired() error {
	return apperrors.NewUnauthorized("api key required")
}

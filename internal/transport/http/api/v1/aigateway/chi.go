package aigateway

import (
	"github.com/go-chi/chi/v5"
)

// RegisterChiRoutes wires the AI Gateway data plane routes onto the given
// chi.Router. 调用方负责把这些路由挂到合适的父路由组上——AI Gateway 的
// 协议面（/api/v1/ai/faces/{openai,anthropic,gemini}/*）是面向外部客户端的
// API Key 鉴权面，不走 internal protected session chain。
//
// 三个协议面共用同一组 inbound handler（CanonicalRequest 统一），
// 但 outbound 响应按 protocol face 用不同格式：
//
//	openai     → chat.completions JSON (+ OpenAI SSE streaming)
//	anthropic  → Messages POST /v1/messages JSON (+ SSE 保留 OpenAI-style)
//	gemini     → generateContent JSON (+ SSE 保留 OpenAI-style)
func RegisterChiRoutes(router chi.Router, handler Handler) {
	// 协议面 — OpenAI-compatible
	router.Route("/api/v1/ai/faces/openai", func(r chi.Router) {
		r.Post("/chat/completions", handler.handleChat)
		r.Post("/embeddings", handler.handleEmbeddings)
		r.Get("/models", handler.handleModels)
	})

	// 协议面 — Anthropic Messages 原生响应
	router.Route("/api/v1/ai/faces/anthropic", func(r chi.Router) {
		r.Post("/chat/completions", handler.handleChatAnthropic)
		// 原生 Anthropic Messages 路径（Claude SDK 默认 POST /v1/messages）：同一
		// handler，入站按 Messages body 自动识别，出站为 Messages 原生响应。
		r.Post("/messages", handler.handleChatAnthropic)
		r.Post("/embeddings", handler.handleEmbeddings)
		r.Get("/models", handler.handleModels)
	})

	// 协议面 — Gemini generateContent 原生响应
	router.Route("/api/v1/ai/faces/gemini", func(r chi.Router) {
		r.Post("/chat/completions", handler.handleChatGemini)
		r.Post("/embeddings", handler.handleEmbeddings)
		r.Get("/models", handler.handleModels)
	})
}

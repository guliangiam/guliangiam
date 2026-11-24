package aigateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

// ---------------------------------------------------------------------------
// Helpers — Content Block Extraction
// ---------------------------------------------------------------------------

// ExtractText flattens a list of CanonicalContentBlock into a single text
// string. Image / tool_use / tool_result blocks are ignored because OpenAI's
// non-streaming shape only carries a single assistant message string.
func ExtractText(blocks []domain.CanonicalContentBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		if b.Type == domain.ContentBlockText {
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

// ExtractToolCalls pulls all tool_use content blocks and renders them as
// OpenAI-style tool_calls entries on the assistant message. Returns an empty
// slice when the model did not request any tools.
func ExtractToolCalls(blocks []domain.CanonicalContentBlock) []ChatToolCall {
	out := make([]ChatToolCall, 0)
	for _, b := range blocks {
		if b.Type != domain.ContentBlockToolUse {
			continue
		}
		out = append(out, ChatToolCall{
			ID:   b.ToolCallID,
			Type: fieldFunction,
			Function: ChatToolFunc{
				Name:       b.ToolName,
				Parameters: b.ToolInput,
			},
		})
	}
	return out
}

// toolInputToJSON renders a tool input map as a compact JSON string — exactly
// what OpenAI's stream contract expects for `function.arguments`. Returns "{}"
// when the input is nil or marshalable fails.
func toolInputToJSON(input map[string]any) string {
	if len(input) == 0 {
		return "{}"
	}
	b, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ExtractEmbeddings extracts the first embedding vector from content blocks.
// The AI Gateway embedding path is simple: one model, one input, one vector.
func ExtractEmbeddings(blocks []domain.CanonicalContentBlock) []EmbeddingData {
	_ = blocks
	return []EmbeddingData{
		{
			Index:     0,
			Object:    "embedding",
			Embedding: []float64{},
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers — Auth / Request Body
// ---------------------------------------------------------------------------

// ExtractAPIKeyHash pulls the raw API key string out of request headers
// and returns its SHA-256 hex digest — the canonical lookup key for
// ApiKeysRepo.GetByHash. The digest is computed over the complete submitted
// key (including the "sk-" prefix), the same input that
// domain.LlmApiKey.GenerateKey hashes and VerifyKey compares.
// Supports both OpenAI-style Authorization: Bearer and x-api-key header.
func ExtractAPIKeyHash(authHeader, apiKeyHeader string) (string, bool) {
	var raw string
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		raw = strings.TrimSpace(authHeader[7:])
	} else if authHeader != "" {
		raw = strings.TrimSpace(authHeader)
	}
	if raw == "" {
		raw = strings.TrimSpace(apiKeyHeader)
	}
	if raw == "" {
		return "", false
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:]), true
}

// ---------------------------------------------------------------------------
// Helpers — Type Coercion (map[string]any accessors)
// ---------------------------------------------------------------------------

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case nil:
		return false
	case string:
		return strings.EqualFold(x, "true") || x == "1"
	default:
		return false
	}
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func hasKey(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}

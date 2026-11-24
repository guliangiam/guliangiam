package aiupstream

import (
	"sort"
	"sync"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// openaiCompatibleProviders lists provider types that default to the
// OpenAI HTTP surface when no explicit protocol is configured.
var openaiCompatibleProviders = map[string]struct{}{
	protocolOpenAI: {},
	"deepseek":     {},
	"glm":          {},
	"kimi":         {},
	"qwen":         {},
	"seed":         {},
	"ollama":       {},
	"vllm":         {},
}

// anthropicFamilyProviders lists provider types that use the Anthropic
// Messages API surface — they must NOT fall back to openai.
var anthropicFamilyProviders = map[string]struct{}{
	protocolAnthropic: {},
	"claude":          {},
}

// geminiFamilyProviders lists provider types that use the Google Gemini
// generateContent API surface — they must NOT fall back to openai.
var geminiFamilyProviders = map[string]struct{}{
	protocolGemini:         {},
	"google_generative_ai": {},
	"flash":                {},
}

// KeyDecryptor opens provider credentials stored in
// llm_providers.api_key_sealed. Reads use the column API with the provider row
// AAD so a v2 token is authenticated against (table, column, id), while v1/v0
// values stay readable during the F2-01c upgrade. Writes remain EncryptField
// (v1) until Batch 2. Implementations must not include plaintext or ciphertext
// material in returned errors.
type KeyDecryptor interface {
	DecryptColumn(aad crypto.AADContext, ciphertext string) (string, error)
}

// AdapterFactory is the process-wide registry that wires protocol-face keys
// to UpstreamAdapter instances. It implements domain.ProviderRegistry so
// the Gateway Core can read it at runtime without knowing which adapters
// exist.
type AdapterFactory struct {
	mu       sync.RWMutex
	registry map[string]domain.UpstreamAdapter

	// Decryptor opens api_key_sealed before an adapter is constructed. When
	// nil it resolves to crypto.GlobalFieldEncryptor() on first use, matching
	// the default encryptor policy in AdminService. Bootstrap may inject an
	// alternative (KMS) implementation; tests inject fakes.
	Decryptor KeyDecryptor
}

// NewAdapterFactory returns an empty registry. Bootstrap code is expected
// to call Register for each concrete adapter, or RegisterDefaults to
// receive the built-in OpenAI-compatible adapter.
func NewAdapterFactory() *AdapterFactory {
	return &AdapterFactory{registry: make(map[string]domain.UpstreamAdapter)}
}

// Register wires a protocol string ("openai", "anthropic", …) to an
// adapter. Later registrations for the same protocol overwrite earlier
// ones — this lets tests swap adapters easily.
func (f *AdapterFactory) Register(protocol string, adapter domain.UpstreamAdapter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registry[protocol] = adapter
}

// Get looks up an adapter by protocol and returns it along with whether
// it was found.
func (f *AdapterFactory) Get(protocol string) (domain.UpstreamAdapter, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	a, ok := f.registry[protocol]
	return a, ok
}

// List returns a sorted snapshot of every registered protocol key.
func (f *AdapterFactory) List() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	keys := make([]string, 0, len(f.registry))
	for k := range f.registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// GetForProvider resolves an adapter for a concrete provider instance.
//
// Resolution order:
//  1. preferredProtocol — 显式指定的协议面直接命中。
//  2. providerType family 识别 — anthropicFamily → "anthropic",
//     geminiFamily → "gemini", openaiCompatible → "openai"。
//  3. preferredProtocol 再兜一次（显式指定但 adapter 未注册）。
func (f *AdapterFactory) GetForProvider(providerType, preferredProtocol string) (domain.UpstreamAdapter, bool) {
	if preferredProtocol != "" {
		if a, ok := f.Get(preferredProtocol); ok {
			return a, true
		}
	}
	// Anthropic family — 绝不回落到 openai。
	if _, ok := anthropicFamilyProviders[providerType]; ok {
		if a, ok := f.Get(protocolAnthropic); ok {
			return a, true
		}
	}
	// Gemini family — 绝不回落到 openai。
	if _, ok := geminiFamilyProviders[providerType]; ok {
		if a, ok := f.Get(protocolGemini); ok {
			return a, true
		}
	}
	// OpenAI-compatible family.
	if _, ok := openaiCompatibleProviders[providerType]; ok {
		if a, ok := f.Get(protocolOpenAI); ok {
			return a, true
		}
	}
	if preferredProtocol != "" {
		if a, ok := f.Get(preferredProtocol); ok {
			return a, true
		}
	}
	return nil, false
}

// RegisterDefaults installs the built-in adapters as handlers for
// protocols "openai", "anthropic" and "gemini". baseURL/apiKey/timeout
// 均为零值 —— 这些仅作 stub / 占位，生产环境必须通过 BuildAdapter
// 配合真实 provider 记录来构造带参 adapter。
func RegisterDefaults(factory *AdapterFactory) {
	factory.Register(protocolOpenAI, NewOpenAICompat("", "", 0))
	factory.Register(protocolAnthropic, NewAnthropicMessages("", "", 0))
	factory.Register(protocolGemini, NewGeminiGenerateContent("", "", 0))
}

// BuildAdapter constructs a fresh UpstreamAdapter for the given provider.
//
// Unlike Get / GetForProvider (which return process-wide singletons),
// BuildAdapter wires a **new adapter instance per call** with the provider's
// own BaseURL, decrypted credential and RequestTimeoutMs. This is what the
// data-plane Chat / ChatStream / Embedding paths MUST use — each provider in
// the DB has its own endpoint and credentials, and the adapter needs those at
// runtime.
//
// Protocol resolution:
//  1. protocolFace 优先（由 RouterEngine 从 Provider.PreferredProtocol 或路由配置确定）。
//  2. 未注册的 protocolFace → 尝试 providerType family 回退。
//  3. 仍失败 → 返回 domain.ErrUnsupportedProtocol。
//
// api_key_sealed 在构造任何适配器之前解密一次，protocolFace 直接匹配与
// providerType family 回退的所有分支都只使用解密后的凭据。解密失败即
// fail-close：不构造适配器、不透传密文、不回退明文。
// 空 api_key_sealed 表示未配置凭据（种子 disabled 行），保持以空 key 构造。
func (f *AdapterFactory) BuildAdapter(provider *domain.LlmProvider, protocolFace string) (domain.UpstreamAdapter, error) {
	if provider == nil {
		return nil, apperrors.NewInvalidArgument("provider is nil")
	}

	apiKey, err := f.decryptAPIKey(provider.ID, provider.APIKeySealed)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(provider.RequestTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// 1. 直接匹配 protocolFace
	switch protocolFace {
	case protocolOpenAI:
		return NewOpenAICompat(provider.BaseURL, apiKey, timeout), nil
	case protocolAnthropic:
		return NewAnthropicMessages(provider.BaseURL, apiKey, timeout), nil
	case protocolGemini:
		return NewGeminiGenerateContent(provider.BaseURL, apiKey, timeout), nil
	}

	// 2. family 回退
	pt := provider.ProviderType
	if _, ok := anthropicFamilyProviders[pt]; ok {
		return NewAnthropicMessages(provider.BaseURL, apiKey, timeout), nil
	}
	if _, ok := geminiFamilyProviders[pt]; ok {
		return NewGeminiGenerateContent(provider.BaseURL, apiKey, timeout), nil
	}
	if _, ok := openaiCompatibleProviders[pt]; ok {
		return NewOpenAICompat(provider.BaseURL, apiKey, timeout), nil
	}

	return nil, domain.ErrUnsupportedProtocol
}

// decryptAPIKey opens the credential sealed at rest. An empty sealed value
// means "no credential configured" and is returned as an empty key without
// touching the encryptor.
//
// The row AAD is built from the provider's own primary key, so a v2 token moved
// to another provider row fails with ErrAADMismatch and an empty provider id is
// rejected instead of falling back to an unbound read. Existing v1/v0 values
// keep their previous plaintext (v1 ignores the supplied AAD).
//
// The default decryptor is crypto.GlobalFieldEncryptor(); bootstrap publishes
// it at startup. Decryption errors are wrapped as typed platform errors and
// never carry the sealed value or the plaintext.
func (f *AdapterFactory) decryptAPIKey(providerID, sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	decryptor := f.Decryptor
	if decryptor == nil {
		decryptor = crypto.GlobalFieldEncryptor()
	}
	plaintext, err := decryptor.DecryptColumn(crypto.LLMProviderAPIKeySealedColumn.AAD(providerID), sealed)
	if err != nil {
		return "", apperrors.WrapInternal(err, "aigateway: provider credential decryption failed")
	}
	return plaintext, nil
}

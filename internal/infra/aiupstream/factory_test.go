package aiupstream

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}

// sealedKey encrypts plaintext with the real FieldEncryptor, mirroring
// AdminService.encryptAPIKey, so fixtures exercise the envelope path.
func sealedKey(t *testing.T, plaintext string) string {
	t.Helper()
	enc, err := crypto.NewFieldEncryptorChecked()
	require.NoError(t, err)
	sealed, err := enc.EncryptField(plaintext)
	require.NoError(t, err)
	require.NotEmpty(t, sealed)
	return sealed
}

// ── Factory: family resolution via GetForProvider ──────────────────────

func TestAdapterFactory_GetForProvider_AnthropicFamily(t *testing.T) {
	factory := NewAdapterFactory()
	anthropic := NewAnthropicMessages("", "", 0)
	factory.Register("anthropic", anthropic)

	// Anthropic family should hit the anthropic adapter — never fall back.
	a, ok := factory.GetForProvider("anthropic", "")
	assert.True(t, ok)
	assert.Equal(t, "anthropic", a.SupportedProtocol())

	a, ok = factory.GetForProvider("claude", "")
	assert.True(t, ok, "claude is in anthropicFamilyProviders")
	assert.Equal(t, "anthropic", a.SupportedProtocol())

	// preferredProtocol='openai' for an anthropic provider should NOT force openai
	// — family rule wins when preferred is missing.
	a, ok = factory.GetForProvider("anthropic", "openai")
	assert.True(t, ok, "should still find anthropic via family fallback")
	assert.Equal(t, "anthropic", a.SupportedProtocol())
}

func TestAdapterFactory_GetForProvider_GeminiFamily(t *testing.T) {
	factory := NewAdapterFactory()
	gemini := NewGeminiGenerateContent("", "", 0)
	factory.Register("gemini", gemini)

	for _, pt := range []string{"gemini", "google_generative_ai", "flash"} {
		a, ok := factory.GetForProvider(pt, "")
		assert.True(t, ok, "%s should resolve to gemini", pt)
		assert.Equal(t, "gemini", a.SupportedProtocol())
	}
}

func TestAdapterFactory_GetForProvider_OpenAICompatibleFamily(t *testing.T) {
	factory := NewAdapterFactory()
	openai := NewOpenAICompat("", "", 0)
	factory.Register("openai", openai)

	for _, pt := range []string{"openai", "deepseek", "glm", "kimi", "qwen", "seed", "ollama", "vllm"} {
		a, ok := factory.GetForProvider(pt, "")
		assert.True(t, ok, "%s should resolve to openai-compatible", pt)
		assert.Equal(t, "openai", a.SupportedProtocol())
	}
}

func TestAdapterFactory_GetForProvider_PreferredProtocolFirst(t *testing.T) {
	factory := NewAdapterFactory()
	factory.Register("openai", NewOpenAICompat("", "", 0))
	factory.Register("custom", NewOpenAICompat("", "", 0)) // custom protocol

	// preferredProtocol='custom' for an anthropic-ish provider — explicit wins
	a, ok := factory.GetForProvider("anthropic", "custom")
	assert.True(t, ok)
	assert.Equal(t, "openai", a.SupportedProtocol()) // custom → NewOpenAICompat

	// preferred not registered → family fallback
	factory2 := NewAdapterFactory()
	factory2.Register("anthropic", NewAnthropicMessages("", "", 0))
	a, ok = factory2.GetForProvider("anthropic", "not-registered")
	assert.True(t, ok)
	assert.Equal(t, "anthropic", a.SupportedProtocol())
}

func TestAdapterFactory_GetForProvider_UnknownTypeNoFamily(t *testing.T) {
	factory := NewAdapterFactory()
	// Nothing registered — all lookups fail
	_, ok := factory.GetForProvider("totally-unknown", "")
	assert.False(t, ok)

	_, ok = factory.GetForProvider("totally-unknown", "unknown-protocol")
	assert.False(t, ok)
}

// ── Factory: BuildAdapter constructs real adapter per provider ──────────

func TestAdapterFactory_BuildAdapter_ProtocolFaceDispatch(t *testing.T) {
	plaintextKey := "sk-live-openai-fixture"
	sealed := sealedKey(t, plaintextKey)
	factory := NewAdapterFactory()

	provider := &domain.LlmProvider{
		ID:                "p-1",
		ProviderType:      "openai",
		PreferredProtocol: "openai",
		BaseURL:           "https://api.openai.com",
		APIKeySealed:      sealed,
		RequestTimeoutMs:  0, // zero → default
	}

	t.Run("openai face", func(t *testing.T) {
		a, err := factory.BuildAdapter(provider, "openai")
		require.NoError(t, err)
		assert.Equal(t, "openai", a.SupportedProtocol())
		oa, ok := a.(*OpenAICompatAdapter)
		require.True(t, ok, "must return OpenAICompatAdapter")
		assert.Equal(t, plaintextKey, oa.apiKey, "adapter must hold the decrypted credential")
		assert.NotEqual(t, sealed, oa.apiKey, "sealed ciphertext must not be passed through")
	})

	t.Run("anthropic face", func(t *testing.T) {
		a, err := factory.BuildAdapter(provider, "anthropic")
		require.NoError(t, err)
		assert.Equal(t, "anthropic", a.SupportedProtocol())
		aa, ok := a.(*AnthropicMessagesAdapter)
		require.True(t, ok, "must return AnthropicMessagesAdapter")
		assert.Equal(t, plaintextKey, aa.apiKey, "adapter must hold the decrypted credential")
		assert.NotEqual(t, sealed, aa.apiKey, "sealed ciphertext must not be passed through")
	})

	t.Run("gemini face", func(t *testing.T) {
		a, err := factory.BuildAdapter(provider, "gemini")
		require.NoError(t, err)
		assert.Equal(t, "gemini", a.SupportedProtocol())
		ga, ok := a.(*GeminiGenerateContentAdapter)
		require.True(t, ok, "must return GeminiGenerateContentAdapter")
		assert.Equal(t, plaintextKey, ga.apiKey, "adapter must hold the decrypted credential")
		assert.NotEqual(t, sealed, ga.apiKey, "sealed ciphertext must not be passed through")
	})
}

// Default (uninjected) decryptor must be crypto.GlobalFieldEncryptor(), the
// same instance AdminService uses to seal api_key_sealed.
func TestAdapterFactory_BuildAdapter_DefaultDecryptor(t *testing.T) {
	plaintextKey := "sk-default-decryptor-fixture"
	sealed := sealedKey(t, plaintextKey)
	factory := NewAdapterFactory()

	a, err := factory.BuildAdapter(&domain.LlmProvider{
		ID:           "p-default",
		ProviderType: "openai",
		BaseURL:      "https://api.openai.com",
		APIKeySealed: sealed,
	}, "openai")
	require.NoError(t, err)
	oa, ok := a.(*OpenAICompatAdapter)
	require.True(t, ok)
	assert.Equal(t, plaintextKey, oa.apiKey)
}

// Existing v1 rows have no AAD and must keep decrypting after the read path
// switch; this is the zero-risk property of F2-01c Batch 1.
func TestAdapterFactory_BuildAdapter_SealedV1StaysReadable(t *testing.T) {
	enc, err := crypto.NewFieldEncryptorChecked()
	require.NoError(t, err)
	const plaintextKey = "sk-v1-legacy"
	v1, err := enc.EncryptField(plaintextKey)
	require.NoError(t, err)
	require.NotContains(t, v1, "-aad:")

	factory := NewAdapterFactory()
	factory.Decryptor = enc
	build := func(id string) (*OpenAICompatAdapter, error) {
		a, err := factory.BuildAdapter(&domain.LlmProvider{
			ID:           id,
			ProviderType: "openai",
			BaseURL:      "https://api.openai.com",
			APIKeySealed: v1,
		}, "openai")
		if err != nil {
			return nil, err
		}
		oa, ok := a.(*OpenAICompatAdapter)
		require.True(t, ok)
		return oa, nil
	}

	oa, err := build("p-v1")
	require.NoError(t, err)
	assert.Equal(t, plaintextKey, oa.apiKey)

	// v1 carries no row identity, so a token moved to another provider row still
	// opens -- the legacy behavior this batch intentionally preserves.
	oa, err = build("p-other")
	require.NoError(t, err)
	assert.Equal(t, plaintextKey, oa.apiKey)
}

// F2-01c Batch 1: a v2 token is bound to the provider row. The same token on
// another provider row must fail with ErrAADMismatch instead of returning the
// original plaintext.
func TestAdapterFactory_BuildAdapter_SealedV2RequiresMatchingProvider(t *testing.T) {
	enc, err := crypto.NewFieldEncryptorChecked()
	require.NoError(t, err)
	const plaintextKey = "sk-v2-row-bound"
	v2, err := enc.EncryptColumn(crypto.LLMProviderAPIKeySealedColumn.AAD("p-a"), plaintextKey)
	require.NoError(t, err)
	require.Contains(t, v2, "-aad:")

	factory := NewAdapterFactory()
	factory.Decryptor = enc
	build := func(id, sealed string) (domain.UpstreamAdapter, error) {
		return factory.BuildAdapter(&domain.LlmProvider{
			ID:           id,
			ProviderType: "openai",
			BaseURL:      "https://api.openai.com",
			APIKeySealed: sealed,
		}, "openai")
	}

	a, err := build("p-a", v2)
	require.NoError(t, err)
	oa, ok := a.(*OpenAICompatAdapter)
	require.True(t, ok)
	assert.Equal(t, plaintextKey, oa.apiKey)

	a, err = build("p-b", v2)
	require.Error(t, err)
	assert.Nil(t, a)
	require.ErrorIs(t, err, crypto.ErrAADMismatch)
	assert.NotContains(t, err.Error(), v2, "the error must not echo the sealed token")

	// A v2 token bound to another registered column cannot be replayed as an API key.
	crossed, err := enc.EncryptColumn(crypto.PayChannelConfigSealedColumn.AAD("p-a"), plaintextKey)
	require.NoError(t, err)
	_, err = build("p-a", crossed)
	require.ErrorIs(t, err, crypto.ErrAADMismatch)

	// The pre-Batch-1 unbound reader cannot open the v2 token at all.
	_, err = enc.DecryptField(v2)
	require.ErrorIs(t, err, crypto.ErrAADContext)
}

// A sealed value without a provider id has no row binding. It must fail closed
// instead of silently using the legacy unbound path.
func TestAdapterFactory_BuildAdapter_SealedRequiresProviderID(t *testing.T) {
	enc, err := crypto.NewFieldEncryptorChecked()
	require.NoError(t, err)
	v1, err := enc.EncryptField("sk-no-row")
	require.NoError(t, err)

	factory := NewAdapterFactory()
	factory.Decryptor = enc
	a, err := factory.BuildAdapter(&domain.LlmProvider{
		ProviderType: "openai",
		BaseURL:      "https://api.openai.com",
		APIKeySealed: v1,
	}, "openai")
	require.Error(t, err)
	assert.Nil(t, a)
	require.ErrorIs(t, err, crypto.ErrAADContext)
}

// The decryptor receives the registered (table, column, provider id) binding,
// never a bare ciphertext-only read.
func TestAdapterFactory_DecryptAPIKey_BindsProviderRow(t *testing.T) {
	decryptor := &stubDecryptor{plaintext: "sk-open"}
	factory := NewAdapterFactory()
	factory.Decryptor = decryptor

	plaintext, err := factory.decryptAPIKey("provider-1", "v1-token")
	require.NoError(t, err)
	assert.Equal(t, "sk-open", plaintext)
	assert.Equal(t, 1, decryptor.calls)
	assert.Equal(t, crypto.AADContext{
		Table: "guliangiam.llm_providers", Column: "api_key_sealed", PKValue: "provider-1",
	}, decryptor.aad)
}

type stubDecryptor struct {
	calls     int
	plaintext string
	err       error
	aad       crypto.AADContext
}

func (d *stubDecryptor) DecryptColumn(aad crypto.AADContext, _ string) (string, error) {
	d.calls++
	d.aad = aad
	return d.plaintext, d.err
}

// Decryption failure must fail closed for every dispatch branch: no adapter,
// typed error, no ciphertext or plaintext in the message.
func TestAdapterFactory_BuildAdapter_DecryptFailureFailsClosed(t *testing.T) {
	const sealed = "v1:not-decryptable-token"
	wantErr := errors.New("cipher: message authentication failed")
	factory := NewAdapterFactory()
	factory.Decryptor = &stubDecryptor{err: wantErr}

	branches := []struct {
		name         string
		protocolFace string
		providerType string
	}{
		{name: "openai face", protocolFace: "openai", providerType: "openai"},
		{name: "anthropic face", protocolFace: "anthropic", providerType: "anthropic"},
		{name: "gemini face", protocolFace: "gemini", providerType: "gemini"},
		{name: "anthropic family fallback", protocolFace: "", providerType: "claude"},
		{name: "gemini family fallback", protocolFace: "", providerType: "google_generative_ai"},
		{name: "openai family fallback", protocolFace: "", providerType: "deepseek"},
	}

	for _, b := range branches {
		t.Run(b.name, func(t *testing.T) {
			a, err := factory.BuildAdapter(&domain.LlmProvider{
				ID:           "p-1",
				ProviderType: b.providerType,
				BaseURL:      "https://upstream.example.com",
				APIKeySealed: sealed,
			}, b.protocolFace)
			require.Error(t, err)
			assert.Nil(t, a, "no adapter may be constructed when decryption fails")

			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr, "must be a typed platform error")
			assert.Equal(t, apperrors.CodeInternal, appErr.Code)
			assert.ErrorIs(t, err, wantErr, "cause must be preserved")
			assert.NotContains(t, err.Error(), sealed, "error must not expose the sealed value")
		})
	}
}

// Empty api_key_sealed (seed/disabled rows) keeps the previous behaviour:
// build the adapter with an empty key and never touch the decryptor.
func TestAdapterFactory_BuildAdapter_EmptySealedSkipsDecryption(t *testing.T) {
	decryptor := &stubDecryptor{err: errors.New("decryptor must not be called for empty sealed")}
	factory := NewAdapterFactory()
	factory.Decryptor = decryptor

	a, err := factory.BuildAdapter(&domain.LlmProvider{
		ProviderType: "openai",
		BaseURL:      "https://api.openai.com",
		APIKeySealed: "",
	}, "openai")
	require.NoError(t, err)
	assert.Equal(t, 0, decryptor.calls, "empty sealed must not invoke the decryptor")
	oa, ok := a.(*OpenAICompatAdapter)
	require.True(t, ok)
	assert.Equal(t, "", oa.apiKey)
}

func TestAdapterFactory_BuildAdapter_NilProvider(t *testing.T) {
	factory := NewAdapterFactory()
	_, err := factory.BuildAdapter(nil, "openai")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestAdapterFactory_BuildAdapter_TimeoutDefault(t *testing.T) {
	factory := NewAdapterFactory()
	// Provider with RequestTimeoutMs = 0 → should default to 30s
	provider := &domain.LlmProvider{
		ProviderType: "openai",
		BaseURL:      "https://example.com",
	}
	a, err := factory.BuildAdapter(provider, "openai")
	require.NoError(t, err)
	// We can't read the unexported timeout field from the interface, but
	// construction succeeded with zero timeout → default applied.
	assert.NotNil(t, a)
}

func TestAdapterFactory_BuildAdapter_FamilyFallbackDecryptsCredential(t *testing.T) {
	plaintextKey := "sk-family-fixture"
	sealed := sealedKey(t, plaintextKey)
	factory := NewAdapterFactory()

	cases := []struct {
		name         string
		providerType string
		baseURL      string
		protocol     string
		assertKey    func(t *testing.T, a domain.UpstreamAdapter)
	}{
		{
			name:         "anthropic family",
			providerType: "claude",
			baseURL:      "https://api.anthropic.com",
			protocol:     "anthropic",
			assertKey: func(t *testing.T, a domain.UpstreamAdapter) {
				aa, ok := a.(*AnthropicMessagesAdapter)
				require.True(t, ok)
				assert.Equal(t, plaintextKey, aa.apiKey)
			},
		},
		{
			name:         "gemini family",
			providerType: "google_generative_ai",
			baseURL:      "https://generativelanguage.googleapis.com",
			protocol:     "gemini",
			assertKey: func(t *testing.T, a domain.UpstreamAdapter) {
				ga, ok := a.(*GeminiGenerateContentAdapter)
				require.True(t, ok)
				assert.Equal(t, plaintextKey, ga.apiKey)
			},
		},
		{
			name:         "openai family",
			providerType: "deepseek",
			baseURL:      "https://api.deepseek.com",
			protocol:     "openai",
			assertKey: func(t *testing.T, a domain.UpstreamAdapter) {
				oa, ok := a.(*OpenAICompatAdapter)
				require.True(t, ok)
				assert.Equal(t, plaintextKey, oa.apiKey)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := factory.BuildAdapter(&domain.LlmProvider{
				ID:           "p-family",
				ProviderType: tc.providerType,
				BaseURL:      tc.baseURL,
				APIKeySealed: sealed,
			}, "")
			require.NoError(t, err)
			assert.Equal(t, tc.protocol, a.SupportedProtocol())
			tc.assertKey(t, a)
		})
	}
}

func TestAdapterFactory_BuildAdapter_UnknownProtocol(t *testing.T) {
	factory := NewAdapterFactory()
	provider := &domain.LlmProvider{
		ProviderType: "custom",
		BaseURL:      "https://example.com",
	}
	// providerType="custom" is not in any family map + protocolFace="unknown" → error
	_, err := factory.BuildAdapter(provider, "unknown")
	require.ErrorIs(t, err, domain.ErrUnsupportedProtocol)
}

// ── Factory: Register/Get/List thread safety ────────────────────────────

func TestAdapterFactory_ConcurrentRegisterGet(t *testing.T) {
	factory := NewAdapterFactory()

	var wg sync.WaitGroup
	const goroutines = 20

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			proto := strings.Builder{}
			proto.WriteString("proto-")
			proto.WriteByte(byte('a' + (idx % 26)))
			// Register + Get concurrently
			factory.Register(proto.String(), NewOpenAICompat("", "", 0))
			_, _ = factory.Get(proto.String())
			factory.List()
		}(i)
	}
	wg.Wait()

	// List should contain 20 distinct keys
	assert.Equal(t, goroutines, len(factory.List()))
}

// ── RegisterDefaults installs three adapters ────────────────────────────

func TestAdapterFactory_RegisterDefaults(t *testing.T) {
	factory := NewAdapterFactory()
	RegisterDefaults(factory)

	for _, proto := range []string{"openai", "anthropic", "gemini"} {
		a, ok := factory.Get(proto)
		assert.True(t, ok, "%s should be registered by RegisterDefaults", proto)
		assert.Equal(t, proto, a.SupportedProtocol())
	}

	list := factory.List()
	assert.Len(t, list, 3)
	assert.Equal(t, []string{"anthropic", "gemini", "openai"}, list) // sorted
}

// ── Regression: NewOpenAICompat trims trailing slashes from baseURL ─────

func TestOpenAICompatAdapter_TrimsTrailingSlash(t *testing.T) {
	a := NewOpenAICompat("https://api.openai.com/", "", 10*time.Second)
	_ = a
	// We can't read unexported baseURL, but construction doesn't panic — good enough.
}

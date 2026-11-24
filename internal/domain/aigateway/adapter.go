package aigateway

import (
	"context"
	"errors"
)

// UpstreamAdapter is the Port that every vendor-specific adapter implements.
// The Core only ever talks to this interface — the transport surface and the
// vendor adapter are the only two places that know about concrete protocols.
type UpstreamAdapter interface {
	// Name returns the provider-family identifier the adapter handles
	// ("openai_compat", "anthropic", "gemini", "local_model", …).
	Name() string
	// SupportedProtocol tells callers which protocol face the adapter consumes
	// ("openai", "anthropic", "gemini"). Used to match the adapter at build
	// time and during failover.
	SupportedProtocol() string
	// Chat sends a non-streaming request.
	Chat(ctx context.Context, req *CanonicalRequest) (*CanonicalResponse, error)
	// ChatStream sends a streaming request and returns a channel of events.
	// The channel closes when the stream ends. Implementations must honour
	// ctx.Done() for client-side cancellation.
	ChatStream(ctx context.Context, req *CanonicalRequest) (<-chan *CanonicalStreamEvent, error)
	// Embedding sends an embedding request. The response body still goes
	// through CanonicalResponse — the Core only differs by whether it expects
	// completion text or an embedding array in that same struct.
	Embedding(ctx context.Context, req *CanonicalRequest) (*CanonicalResponse, error)
}

// ProviderRegistry is a process-wide registry that wires protocol-face keys
// to adapter constructors. Bootstrap populates it once; the Core reads it at
// runtime via BuildAdapter — which builds a **fresh adapter instance** per
// provider (baseURL / apiKey / timeout come from the provider record, NOT
// from a pre-built singleton).
//
// Register / Get / List remain for backward compatibility and for tests that
// want to stub a pre-built adapter.
type ProviderRegistry interface {
	Register(protocol string, adapter UpstreamAdapter)
	Get(protocol string) (UpstreamAdapter, bool)
	List() []string

	// BuildAdapter constructs a fresh UpstreamAdapter for the given provider.
	// It uses the provider's own BaseURL / APIKeySealed / RequestTimeoutMs to
	// wire a concrete adapter instance for protocolFace ("openai", "anthropic",
	// "gemini"). Returns ErrUnsupportedProtocol when the registry has no
	// adapter registered for that protocol face — callers should then surface
	// a 503 to the upstream-facing client.
	BuildAdapter(provider *LlmProvider, protocolFace string) (UpstreamAdapter, error)
}

// ConnectionTester is an optional capability implemented by adapters that can
// issue a credential-bearing, read-only probe (e.g. GET /models). The
// management plane uses it so a provider connection test exercises the same
// credential path as real traffic instead of a bare base-URL HEAD.
type ConnectionTester interface {
	// Probe returns the upstream HTTP status code and, for auth/upstream
	// failures, a typed platform error. Transport errors must not expose the
	// provider credential.
	Probe(ctx context.Context) (statusCode int, err error)
}

// ErrUnsupportedProtocol is returned by ProviderRegistry.BuildAdapter when
// no concrete adapter implementation is available for the requested protocol face.
var ErrUnsupportedProtocol = errors.New("aigateway: no adapter for protocol face")

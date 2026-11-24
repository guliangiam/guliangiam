package providers

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

// Provider couples the family-neutral registration with its native detail.
// Detail always remains inside the application layer; responses are built by
// the family strategy so sealed credentials can never be serialized directly.
type Provider struct {
	Registration domain.Registration
	Detail       any
	plaintext    string
	secretType   string
	fingerprint  string
	keyVersion   int
	metadata     registryMetadata
	row          *domain.Provider
	config       map[string]any
	legacy       bool
}

type AIProviderConfig struct {
	ProviderType       string   `json:"providerType"`
	BaseURL            string   `json:"baseUrl"`
	APIKey             string   `json:"apiKey,omitempty"`
	SupportedProtocols []string `json:"supportedProtocols,omitempty"`
	PreferredProtocol  string   `json:"preferredProtocol,omitempty"`
	RequestTimeoutMs   int      `json:"requestTimeoutMs,omitempty"`
	MaxRetries         int      `json:"maxRetries,omitempty"`
	Priority           int      `json:"priority,omitempty"`
	Weight             int      `json:"weight,omitempty"`
	RPMLimit           *int     `json:"rpmLimit,omitempty"`
	TPMLimit           *int     `json:"tpmLimit,omitempty"`
}

type AIProviderPatch struct {
	ProviderType       *string  `json:"providerType,omitempty"`
	BaseURL            *string  `json:"baseUrl,omitempty"`
	SupportedProtocols []string `json:"supportedProtocols,omitempty"`
	PreferredProtocol  *string  `json:"preferredProtocol,omitempty"`
	RequestTimeoutMs   *int     `json:"requestTimeoutMs,omitempty"`
	MaxRetries         *int     `json:"maxRetries,omitempty"`
	Priority           *int     `json:"priority,omitempty"`
	Weight             *int     `json:"weight,omitempty"`
	RPMLimit           *int     `json:"rpmLimit,omitempty"`
	TPMLimit           *int     `json:"tpmLimit,omitempty"`
}

type PaymentProviderConfig struct {
	Channel      string   `json:"channel"`
	Environment  string   `json:"environment"`
	Config       string   `json:"config,omitempty"`
	ConfigSealed string   `json:"configSealed,omitempty"`
	ConfigSet    []string `json:"configSet,omitempty"`
}

type NotificationProviderConfig struct {
	ProviderType string         `json:"providerType"`
	WebhookURL   string         `json:"webhookUrl,omitempty"`
	AccessToken  string         `json:"accessToken,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
}

type NotificationProviderPatch struct {
	ProviderType *string        `json:"providerType,omitempty"`
	WebhookURL   *string        `json:"webhookUrl,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
}

type ChangeProviderStatusCommand struct {
	ID     string
	Family string
	Status string
	// OrganizationID scopes the read/write to the caller's tenant. Callers that
	// pass an empty value retain the unscoped behaviour used by internal system
	// transitions.
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID  string
	Revision int64
}

type RotateProviderSecretCommand struct {
	ID     string
	Family string
	// ActorID is the operator user id (empty for system actions).
	ActorID     string
	SecretType  string
	Plaintext   string
	Revision    int64
	Reason      string
	GracePeriod time.Duration
}

type RotateProviderSecretResult struct {
	ProviderID   string `json:"providerId"`
	SecretType   string `json:"secretType"`
	SecretSet    bool   `json:"secretSet"`
	SecretMasked string `json:"secretMasked,omitempty"`
	Fingerprint  string `json:"fingerprint"`
	Version      int64  `json:"version"`
}

// ProviderStrategy contains only family semantics. Persistence, transaction,
// registration metadata, audit and outbox orchestration stay in UnifiedService.
type ProviderStrategy interface {
	Family() domain.Family
	Validate(provider *Provider) error
	TestConnection(ctx context.Context, provider *Provider) (*TestConnectionResult, error)
	Sanitize(provider *Provider) map[string]any
	BeforeActivate(ctx context.Context, tx appuow.UnitOfWork, provider *Provider) error
}

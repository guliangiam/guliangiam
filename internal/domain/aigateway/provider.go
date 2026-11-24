package aigateway

import (
	"fmt"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	ProviderStatusActive   = "active"
	ProviderStatusDisabled = "disabled"
	ProviderStatusTesting  = "testing"
)

var ValidProviderStatuses = enumset.NewSet("aigatewayProviderStatus",
	ProviderStatusActive, ProviderStatusDisabled, ProviderStatusTesting,
)

var validProviderTransitions = map[string][]string{
	ProviderStatusActive:   {ProviderStatusDisabled, ProviderStatusTesting},
	ProviderStatusDisabled: {ProviderStatusActive, ProviderStatusTesting},
	ProviderStatusTesting:  {ProviderStatusActive, ProviderStatusDisabled},
}

// ValidProviderTypes is the closed set of upstream vendor families the system
// talks to. It is intentionally kept separate from IAM's providers table —
// the two model completely different things (AI vendor vs identity provider).
var ValidProviderTypes = enumset.NewSet("aigatewayProviderType",
	"openai", "anthropic", "gemini", "deepseek", "glm", "kimi",
	"qwen", "seed", "ollama", "vllm", "custom",
)

type LlmProvider struct {
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"organizationId"`
	Name               string     `json:"name"`
	DisplayName        string     `json:"displayName,omitempty"`
	ProviderType       string     `json:"providerType"`
	BaseURL            string     `json:"baseUrl"`
	APIKeySealed       string     `json:"-"` // encrypted, never exposed
	APIKeyFingerprint  string     `json:"apiKeyFingerprint"`
	Status             string     `json:"status"`
	RequestTimeoutMs   int        `json:"requestTimeoutMs"`
	MaxRetries         int        `json:"maxRetries"`
	Priority           int        `json:"priority"`
	Weight             int        `json:"weight"`
	RPMLimit           *int       `json:"rpmLimit,omitempty"`
	TPMLimit           *int       `json:"tpmLimit,omitempty"`
	SupportedProtocols []string   `json:"supportedProtocols,omitempty"`
	PreferredProtocol  string     `json:"preferredProtocol"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty"`
	Revision           int64      `json:"revision"`
}

func (p *LlmProvider) Validate() error {
	if p.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if p.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if p.Name == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if !ValidProviderTypes.Contains(p.ProviderType) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid providerType: %s", p.ProviderType),
			map[string]any{"field": "providerType", "value": string(p.ProviderType), "allowed": ValidProviderTypes.SortedValues()},
		)
	}
	if p.BaseURL == "" {
		return apperrors.NewInvalidArgument("baseUrl is required")
	}
	if p.Status == "" {
		p.Status = ProviderStatusActive
	}
	if !ValidProviderStatuses.Contains(p.Status) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid status: %s", p.Status),
			map[string]any{"field": "status", "value": p.Status, "allowed": ValidProviderStatuses.SortedValues()},
		)
	}
	if p.RequestTimeoutMs <= 0 {
		p.RequestTimeoutMs = 30000
	}
	if p.MaxRetries < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "maxRetries must be non-negative", map[string]any{"field": "maxRetries", "value": p.MaxRetries, "min": 0})
	}
	if p.PreferredProtocol == "" {
		p.PreferredProtocol = "openai"
	}
	return nil
}

func (p *LlmProvider) CanTransitionTo(newStatus string) bool {
	for _, s := range validProviderTransitions[p.Status] {
		if s == newStatus {
			return true
		}
	}
	return false
}

func (p *LlmProvider) TransitionTo(newStatus string) error {
	if newStatus == p.Status {
		return nil
	}
	if !ValidProviderStatuses.Contains(newStatus) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid provider status: %s", newStatus),
			map[string]any{"field": "status", "value": newStatus, "allowed": ValidProviderStatuses.SortedValues()},
		)
	}
	if !p.CanTransitionTo(newStatus) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move provider from %s to %s", p.Status, newStatus),
			map[string]any{"field": "status", "value": p.Status, "allowed": append([]string{}, validProviderTransitions[p.Status]...)},
		)
	}
	p.Status = newStatus
	return nil
}

// IsAvailable reports whether the provider is in a state that accepts live
// traffic. testing is excluded because test providers may still have stale
// credentials or unconfirmed base URLs.
func (p *LlmProvider) IsAvailable() bool {
	return p.Status == ProviderStatusActive
}

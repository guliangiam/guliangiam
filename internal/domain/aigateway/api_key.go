package aigateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	llmapi "github.com/guliangiam/guliangiam/internal/domain/llm_api_keys"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 生成状态机枚举是唯一事实源（contracts/registries/state-machines.yaml）；
// 这里只保留字符串常量以兼容既有调用点（OPEN-API-41）。
const (
	APIKeyStatusActive  = string(llmapi.LlmApiKeysStateActive_State)
	APIKeyStatusRevoked = string(llmapi.LlmApiKeysStateRevoked_State)
	APIKeyStatusExpired = string(llmapi.LlmApiKeysStateExpired_State)
)

// apiKeyStatuses is the closed set the check and the 422 `allowed` metadata share.
var apiKeyStatuses = []string{APIKeyStatusActive, APIKeyStatusRevoked, APIKeyStatusExpired}

const (
	keyPrefixLen    = 20 // displayed prefix: "sk-abc***xyz" uses first 3 + last 3
	keyFullLenBytes = 32 // 256-bit entropy before hex encoding
	keyHashLenHex   = 64 // SHA-256 → 64 hex chars
)

type LlmApiKey struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	KeyPrefix      string     `json:"keyPrefix"`
	KeyHash        string     `json:"-"` // sha256 hash only, never exposed
	AllowedAliases []string   `json:"allowedAliases,omitempty"`
	RPMLimit       *int       `json:"rpmLimit,omitempty"`
	TPMLimit       *int       `json:"tpmLimit,omitempty"`
	MonthlyBudget  *float64   `json:"monthlyBudget,omitempty"`
	Currency       string     `json:"currency,omitempty"`
	Status         string     `json:"status"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
	Revision       int64      `json:"revision"`
}

// GenerateKey creates a new random API key, returns the plaintext (to be
// shown ONCE at creation) and populates KeyPrefix + KeyHash on the receiver.
// The plaintext must be discarded after the transport layer sends it.
func (k *LlmApiKey) GenerateKey() (plaintext string, err error) {
	raw := make([]byte, keyFullLenBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", apperrors.NewInternalf("failed to generate api key entropy: %v", err)
	}
	plaintext = "sk-" + hex.EncodeToString(raw)
	k.KeyHash = hashKey(plaintext)
	// Prefix for display: "sk-" + first 5 hex + "***" + last 4 hex
	if len(plaintext) > keyPrefixLen {
		k.KeyPrefix = plaintext[:8] + "***" + plaintext[len(plaintext)-4:]
	} else {
		k.KeyPrefix = plaintext
	}
	return plaintext, nil
}

// VerifyKey compares the given plaintext against the stored hash.
func (k *LlmApiKey) VerifyKey(plaintext string) bool {
	if k.KeyHash == "" {
		return false
	}
	return hashKey(plaintext) == k.KeyHash
}

// Revoke marks the key as no longer usable. A revoked key can never be
// revived — create a new one instead.
func (k *LlmApiKey) Revoke() error {
	if k.Status == APIKeyStatusRevoked {
		return nil
	}
	if k.Status == APIKeyStatusExpired {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "cannot revoke an already-expired key, recreate instead", map[string]any{"field": "status", "value": string(k.Status), "expected": "active"})
	}
	k.Status = APIKeyStatusRevoked
	return nil
}

// IsActive reports whether the key can be used right now.
func (k *LlmApiKey) IsActive(now time.Time) bool {
	if k.Status != APIKeyStatusActive {
		return false
	}
	if k.DeletedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && now.After(*k.ExpiresAt) {
		return false
	}
	return true
}

func (k *LlmApiKey) Validate() error {
	if k.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if k.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if k.Name == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if k.Status == "" {
		k.Status = APIKeyStatusActive
	}
	switch k.Status {
	case APIKeyStatusActive, APIKeyStatusRevoked, APIKeyStatusExpired:
		// valid
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid api key status: %s", k.Status),
			map[string]any{"field": "status", "value": k.Status, "allowed": append([]string{}, apiKeyStatuses...)},
		)
	}
	if k.Currency == "" {
		k.Currency = currencyUSD
	}
	// 限额/预算不允许为负：负 RPM/TPM 会让 TryIncr 永远拒绝，
	// 负预算会让预算比对语义颠倒（BE-C-04）。
	if k.RPMLimit != nil && *k.RPMLimit < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "rpmLimit must be non-negative", map[string]any{"field": "rpmLimit", "value": *k.RPMLimit, "min": 0})
	}
	if k.TPMLimit != nil && *k.TPMLimit < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "tpmLimit must be non-negative", map[string]any{"field": "tpmLimit", "value": *k.TPMLimit, "min": 0})
	}
	if k.MonthlyBudget != nil && *k.MonthlyBudget < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "monthlyBudget must be non-negative", map[string]any{"field": "monthlyBudget", "value": *k.MonthlyBudget, "min": 0})
	}
	return nil
}

func hashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

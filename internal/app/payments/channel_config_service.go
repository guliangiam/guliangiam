package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// maxChannelConfigBytes bounds the plaintext JSON accepted by the standalone
// channel-config endpoint. config_sealed is TEXT, so this is a policy bound
// rather than a column bound; it also keeps the v1 envelope comfortably below
// the 65,535-byte limit of MySQL TEXT mirrors (OPEN-API-37).
const maxChannelConfigBytes = 64 << 10

// ChannelConfigCipher seals a plaintext channel config with the existing field
// encryption envelope. It is an app-layer port: the composition root injects
// the platform FieldEncryptor, so app/payments never imports infra or crypto.
type ChannelConfigCipher interface {
	EncryptField(plaintext string) (string, error)
}

// ChannelConfigService 是渠道配置的业务用例接口 (app 层)。
// 职责：版本化管理 + 状态机 (draft → testing → active/disabled) +
// 与渠道网关的 VerifyConfig 沙箱自测。
type ChannelConfigService interface {
	Create(ctx context.Context, input CreateChannelConfigInput) (*domain.PayChannelConfig, error)
	GetByID(ctx context.Context, organizationID, id string) (*domain.PayChannelConfig, error)
	List(ctx context.Context, organizationID string) ([]*domain.PayChannelConfig, error)
	Transition(ctx context.Context, organizationID, id, newStatus string) (*domain.PayChannelConfig, error)
	Disable(ctx context.Context, organizationID, id string) (*domain.PayChannelConfig, error)
}

type CreateChannelConfigInput struct {
	ID             string
	OrganizationID string
	Channel        string
	Environment    string
	// Config is the plaintext JSON object supplied by the client. The service
	// canonicalizes it, derives ConfigSet from its top-level keys, seals it with
	// ChannelConfigCipher and only then persists the envelope.
	Config string
	Now    time.Time
}

type channelConfigService struct {
	Repo   domain.ChannelConfigsRepo
	Clock  clock.Clock
	Cipher ChannelConfigCipher
	UUID   uuid.Generator
}

func NewChannelConfigService(repo domain.ChannelConfigsRepo, ck clock.Clock, cipher ChannelConfigCipher, uuidGen uuid.Generator) ChannelConfigService {
	return &channelConfigService{Repo: repo, Clock: ck, Cipher: cipher, UUID: uuidGen}
}

func (s *channelConfigService) Create(ctx context.Context, input CreateChannelConfigInput) (*domain.PayChannelConfig, error) {
	now := input.Now
	if now.IsZero() {
		now = s.Clock.Now()
	}
	if input.Environment == "" {
		input.Environment = "production"
	}
	if input.ID == "" {
		if s.UUID == nil {
			return nil, apperrors.NewInternal("channel config id generator is not configured")
		}
		id, err := s.UUID.NewString()
		if err != nil {
			return nil, apperrors.WrapInternal(err, "generate channel config id")
		}
		input.ID = id
	}
	canonical, configSet, err := canonicalizeChannelConfig(input.Config)
	if err != nil {
		return nil, err
	}
	if s.Cipher == nil {
		return nil, apperrors.NewInternal("channel config cipher is not configured")
	}
	sealed, err := s.Cipher.EncryptField(canonical)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "seal channel config")
	}
	if sealed == "" {
		return nil, apperrors.NewInternal("channel config cipher returned an empty sealed value")
	}
	fingerprint := configFingerprint(canonical)

	// 自动递增 version_no：同一 org+channel+env 的最大现有 version + 1
	existing, _ := s.Repo.List(ctx, input.OrganizationID)
	maxVersion := 0
	for _, e := range existing {
		if e.Channel == input.Channel && e.Environment == input.Environment && e.OrganizationID == input.OrganizationID {
			if e.VersionNo > maxVersion {
				maxVersion = e.VersionNo
			}
		}
	}

	cfg := &domain.PayChannelConfig{
		ID:                input.ID,
		OrganizationID:    input.OrganizationID,
		Channel:           input.Channel,
		Environment:       input.Environment,
		ConfigSealed:      sealed,
		ConfigFingerprint: fingerprint,
		ConfigSet:         configSet,
		VersionNo:         maxVersion + 1,
		Status:            domain.ChannelConfigStatusDraft,
		CreatedAt:         now,
		UpdatedAt:         now,
		Revision:          1,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := s.Repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *channelConfigService) GetByID(ctx context.Context, organizationID, id string) (*domain.PayChannelConfig, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	return s.Repo.GetByIDForOrganization(ctx, id, organizationID)
}

func (s *channelConfigService) List(ctx context.Context, organizationID string) ([]*domain.PayChannelConfig, error) {
	return s.Repo.List(ctx, organizationID)
}

// Transition 手动推进状态机。允许路径：见 PayChannelConfig.CanTransitionTo。
// 成功时会根据目标状态设置时间戳 (ActivatedAt / DisabledAt) 并持久化。
func (s *channelConfigService) Transition(ctx context.Context, organizationID, id, newStatus string) (*domain.PayChannelConfig, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	cfg, err := s.Repo.GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if !cfg.CanTransitionTo(newStatus) {
		return nil, errInvalidTransition(cfg.Status, newStatus)
	}
	if err := cfg.TransitionTo(newStatus); err != nil {
		return nil, err
	}
	cfg.UpdatedAt = s.Clock.Now()
	if err := s.Repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Disable 快捷方法：任何状态都可以转到 disabled（状态机允许）。
func (s *channelConfigService) Disable(ctx context.Context, organizationID, id string) (*domain.PayChannelConfig, error) {
	return s.Transition(ctx, organizationID, id, domain.ChannelConfigStatusDisabled)
}

// configFingerprint hashes the canonical plaintext JSON config (SHA-256, first
// 8 bytes hex). Hashing the envelope would be unstable because every encryption
// uses a fresh DEK/nonce, so new rows fingerprint the same canonical plaintext
// the server seals.
//
// Rows written before A-6.2B through this standalone endpoint hashed the client
// supplied ciphertext. They are immutable versioned rows and are not backfilled
// in this batch, so their fingerprints remain ciphertext-based until a version
// is re-created. The unified provider path already fingerprinted plaintext.
func configFingerprint(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:8])
}

// canonicalizeChannelConfig validates the request payload and returns
// deterministic JSON plus the sorted top-level keys used for ConfigSet.
// Object key order and insignificant whitespace are normalized, so the same
// logical config yields one fingerprint; numbers are kept verbatim via
// json.Number instead of being re-encoded through float64.
//
// The returned errors never echo the config value: payment configs contain
// credentials and must not reach logs or API responses.
func canonicalizeChannelConfig(raw string) (string, []string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil, invalidChannelConfig("config is required")
	}
	if len(trimmed) > maxChannelConfigBytes {
		return "", nil, invalidChannelConfig("config exceeds the maximum size")
	}

	var object map[string]any
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil {
		return "", nil, invalidChannelConfig("config must be a valid JSON object")
	}
	if len(object) == 0 {
		return "", nil, invalidChannelConfig("config must be a non-empty JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", nil, invalidChannelConfig("config must contain exactly one JSON object")
	}

	canonical, err := json.Marshal(object)
	if err != nil {
		return "", nil, invalidChannelConfig("config must be a valid JSON object")
	}
	if len(canonical) > maxChannelConfigBytes {
		return "", nil, invalidChannelConfig("config exceeds the maximum size")
	}

	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return string(canonical), keys, nil
}

func invalidChannelConfig(message string) error {
	return apperrors.NewWithMeta(apperrors.CodePaymentChannelConfigInvalid, message, map[string]any{"field": "config"})
}

// errInvalidTransition 本地错误构造，避免 import apperrors。
func errInvalidTransition(from, to string) error {
	return &transitionError{from: from, to: to}
}

type transitionError struct {
	from, to string
}

func (e *transitionError) Error() string {
	return "cannot transition channel config from " + e.from + " to " + e.to
}

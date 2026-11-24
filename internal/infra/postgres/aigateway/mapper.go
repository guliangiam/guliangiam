package aigateway

import (
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// ===== LlmProvider =====

func scanLlmProvider(row pgx.Row) (*domain.LlmProvider, error) {
	var p domain.LlmProvider
	var supportedPro protocolsJSON
	err := row.Scan(
		&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.ProviderType,
		&supportedPro, &p.PreferredProtocol, &p.BaseURL,
		&p.APIKeySealed, &p.APIKeyFingerprint, &p.Status,
		&p.RequestTimeoutMs, &p.MaxRetries, &p.Priority, &p.Weight,
		&p.RPMLimit, &p.TPMLimit,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.Revision,
	)
	if err != nil {
		return nil, err
	}
	p.SupportedProtocols = []string(supportedPro)
	return &p, nil
}

// ===== LlmModel =====

func scanLlmModel(row pgx.Row) (*domain.LlmModel, error) {
	var m domain.LlmModel
	var caps capabilitiesJSON
	err := row.Scan(
		&m.ID, &m.OrganizationID, &m.ProviderID, &m.ProviderModelName, &m.DisplayName,
		&caps, &m.MaxContextTokens, &m.MaxOutputTokens,
		&m.Status, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt, &m.Revision,
	)
	if err != nil {
		return nil, err
	}
	m.Capabilities = map[string]any(caps)
	return &m, nil
}

// ===== LlmModelAlias =====

func scanLlmModelAlias(row pgx.Row) (*domain.LlmModelAlias, error) {
	var a domain.LlmModelAlias
	err := row.Scan(
		&a.ID, &a.OrganizationID, &a.Alias, &a.DisplayName,
		&a.Status, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt, &a.Revision,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ===== LlmRoute =====

func scanLlmRoute(row pgx.Row) (*domain.LlmRoute, error) {
	var r domain.LlmRoute
	err := row.Scan(
		&r.ID, &r.OrganizationID, &r.AliasID, &r.ModelID,
		&r.Weight, &r.Priority, &r.Status,
		&r.CreatedAt, &r.UpdatedAt, &r.DeletedAt, &r.Revision,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ===== LlmApiKey =====

func scanLlmApiKey(row pgx.Row) (*domain.LlmApiKey, error) {
	var k domain.LlmApiKey
	var allowedAliases apiKeyAliasesJSON
	err := row.Scan(
		&k.ID, &k.OrganizationID, &k.Name, &k.KeyPrefix, &k.KeyHash,
		&allowedAliases, &k.RPMLimit, &k.TPMLimit, &k.MonthlyBudget,
		&k.Currency, &k.Status, &k.ExpiresAt,
		&k.CreatedAt, &k.UpdatedAt, &k.DeletedAt, &k.Revision,
	)
	if err != nil {
		return nil, err
	}
	k.AllowedAliases = []string(allowedAliases)
	return &k, nil
}

// ===== LlmQuotaPolicy =====

func scanLlmQuotaPolicy(row pgx.Row) (*domain.LlmQuotaPolicy, error) {
	var q domain.LlmQuotaPolicy
	err := row.Scan(
		&q.ID, &q.OrganizationID, &q.Scope, &q.ScopeRef,
		&q.LimitType, &q.LimitValue, &q.ResetCycle, &q.Status,
		&q.CreatedAt, &q.UpdatedAt, &q.DeletedAt, &q.Revision,
	)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// ===== LlmPriceVersion =====

func scanLlmPriceVersion(row pgx.Row) (*domain.LlmPriceVersion, error) {
	var v domain.LlmPriceVersion
	err := row.Scan(
		&v.ID, &v.OrganizationID, &v.ModelID, &v.VersionNo,
		&v.InputPricePer1M, &v.OutputPricePer1M, &v.Currency, &v.EffectiveFrom,
		&v.CreatedAt, &v.UpdatedAt, &v.DeletedAt, &v.Revision,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ===== ProviderHealth =====

func scanProviderHealth(row pgx.Row) (*domain.ProviderHealth, error) {
	var h domain.ProviderHealth
	err := row.Scan(
		&h.ID, &h.ProviderID, &h.State, &h.ConsecutiveFailures,
		&h.OpenUntil, &h.LastCheckedAt, &h.LastLatencyMs, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// ===== UsageLog =====

func scanUsageLog(row pgx.Row) (*domain.UsageLog, error) {
	var u domain.UsageLog
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.APIKeyID, &u.AliasID, &u.ProviderID, &u.ModelID,
		&u.PriceVersionID, &u.ProtocolFace, &u.AttemptNo,
		&u.InputTokens, &u.OutputTokens, &u.TotalTokens,
		&u.CostAmount, &u.Currency, &u.Status,
		&u.ErrorCode, &u.TTFTMs, &u.TotalLatencyMs,
		&u.UpstreamRequestID, &u.FailoverFrom, &u.RequestID, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ===== JSON helpers =====

// protocolsJSON is []string serialized as JSONB.
type protocolsJSON []string

func (j *protocolsJSON) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case nil:
		*j = nil
		return nil
	default:
		return apperrors.NewInternalf("unsupported supported_protocols scan type: %T", src)
	}
	return shared.UnmarshalJSON(data, j)
}

// Value implements driver.Valuer — but pgx uses pgtype instead, so we rely
// on shared.MarshalJSON at call sites and pass []byte directly.

type capabilitiesJSON map[string]any

func (j *capabilitiesJSON) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case nil:
		*j = nil
		return nil
	default:
		return apperrors.NewInternalf("unsupported capabilities scan type: %T", src)
	}
	return shared.UnmarshalJSON(data, j)
}

type apiKeyAliasesJSON []string

func (j *apiKeyAliasesJSON) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case nil:
		*j = nil
		return nil
	default:
		return apperrors.NewInternalf("unsupported allowed_aliases scan type: %T", src)
	}
	return shared.UnmarshalJSON(data, j)
}

// compile-time sanity that we don't accidentally import time without using it.
var _ = time.Time{}

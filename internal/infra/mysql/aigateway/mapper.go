package aigateway

import (
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// ============================================================================
// LlmProvider
// ============================================================================

func scanLlmProvider(sc scannable) (*domain.LlmProvider, error) {
	var p domain.LlmProvider
	var supportedProtocolsStr string
	var rpmLimit sql.NullInt64
	var tpmLimit sql.NullInt64
	err := sc.Scan(
		&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.ProviderType,
		&supportedProtocolsStr, &p.PreferredProtocol, &p.BaseURL,
		&p.APIKeySealed, &p.APIKeyFingerprint, &p.Status,
		&p.RequestTimeoutMs, &p.MaxRetries, &p.Priority, &p.Weight,
		&rpmLimit, &tpmLimit,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.Revision,
	)
	if err != nil {
		return nil, err
	}
	if rpmLimit.Valid {
		v := int(rpmLimit.Int64)
		p.RPMLimit = &v
	}
	if tpmLimit.Valid {
		v := int(tpmLimit.Int64)
		p.TPMLimit = &v
	}
	if supportedProtocolsStr != "" {
		if err := json.Unmarshal([]byte(supportedProtocolsStr), &p.SupportedProtocols); err != nil {
			return nil, apperrors.NewInternal("unmarshal supported_protocols")
		}
	}
	if p.SupportedProtocols == nil {
		p.SupportedProtocols = []string{}
	}
	return &p, nil
}

// ============================================================================
// LlmModel
// ============================================================================

func scanLlmModel(sc scannable) (*domain.LlmModel, error) {
	var m domain.LlmModel
	var capabilitiesStr string
	var maxContextTokens sql.NullInt64
	var maxOutputTokens sql.NullInt64
	err := sc.Scan(
		&m.ID, &m.OrganizationID, &m.ProviderID, &m.ProviderModelName, &m.DisplayName,
		&capabilitiesStr, &maxContextTokens, &maxOutputTokens,
		&m.Status, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt, &m.Revision,
	)
	if err != nil {
		return nil, err
	}
	if maxContextTokens.Valid {
		v := int(maxContextTokens.Int64)
		m.MaxContextTokens = &v
	}
	if maxOutputTokens.Valid {
		v := int(maxOutputTokens.Int64)
		m.MaxOutputTokens = &v
	}
	if capabilitiesStr != "" {
		if err := json.Unmarshal([]byte(capabilitiesStr), &m.Capabilities); err != nil {
			return nil, apperrors.NewInternal("unmarshal capabilities")
		}
	}
	if m.Capabilities == nil {
		m.Capabilities = map[string]any{}
	}
	return &m, nil
}

// ============================================================================
// LlmModelAlias
// ============================================================================

func scanLlmModelAlias(sc scannable) (*domain.LlmModelAlias, error) {
	var a domain.LlmModelAlias
	err := sc.Scan(
		&a.ID, &a.OrganizationID, &a.Alias, &a.DisplayName,
		&a.Status, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt, &a.Revision,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ============================================================================
// LlmRoute
// ============================================================================

func scanLlmRoute(sc scannable) (*domain.LlmRoute, error) {
	var r domain.LlmRoute
	err := sc.Scan(
		&r.ID, &r.OrganizationID, &r.AliasID, &r.ModelID,
		&r.Weight, &r.Priority, &r.Status,
		&r.CreatedAt, &r.UpdatedAt, &r.DeletedAt, &r.Revision,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ============================================================================
// LlmApiKey
// ============================================================================

func scanLlmApiKey(sc scannable) (*domain.LlmApiKey, error) {
	var k domain.LlmApiKey
	var allowedAliasesStr string
	var rpmLimit sql.NullInt64
	var tpmLimit sql.NullInt64
	var monthlyBudget sql.NullFloat64
	err := sc.Scan(
		&k.ID, &k.OrganizationID, &k.Name, &k.KeyPrefix, &k.KeyHash,
		&allowedAliasesStr, &rpmLimit, &tpmLimit, &monthlyBudget,
		&k.Currency, &k.Status, &k.ExpiresAt,
		&k.CreatedAt, &k.UpdatedAt, &k.DeletedAt, &k.Revision,
	)
	if err != nil {
		return nil, err
	}
	if rpmLimit.Valid {
		v := int(rpmLimit.Int64)
		k.RPMLimit = &v
	}
	if tpmLimit.Valid {
		v := int(tpmLimit.Int64)
		k.TPMLimit = &v
	}
	if monthlyBudget.Valid {
		v := monthlyBudget.Float64
		k.MonthlyBudget = &v
	}
	if allowedAliasesStr != "" {
		if err := json.Unmarshal([]byte(allowedAliasesStr), &k.AllowedAliases); err != nil {
			return nil, apperrors.NewInternal("unmarshal allowed_aliases")
		}
	}
	if k.AllowedAliases == nil {
		k.AllowedAliases = []string{}
	}
	return &k, nil
}

// ============================================================================
// LlmQuotaPolicy
// ============================================================================

func scanLlmQuotaPolicy(sc scannable) (*domain.LlmQuotaPolicy, error) {
	var q domain.LlmQuotaPolicy
	var scopeRef sql.NullString
	err := sc.Scan(
		&q.ID, &q.OrganizationID, &q.Scope, &scopeRef,
		&q.LimitType, &q.LimitValue, &q.ResetCycle, &q.Status,
		&q.CreatedAt, &q.UpdatedAt, &q.DeletedAt, &q.Revision,
	)
	if err != nil {
		return nil, err
	}
	if scopeRef.Valid {
		v := scopeRef.String
		q.ScopeRef = &v
	}
	return &q, nil
}

// ============================================================================
// LlmPriceVersion
// ============================================================================

func scanLlmPriceVersion(sc scannable) (*domain.LlmPriceVersion, error) {
	var v domain.LlmPriceVersion
	err := sc.Scan(
		&v.ID, &v.OrganizationID, &v.ModelID, &v.VersionNo,
		&v.InputPricePer1M, &v.OutputPricePer1M, &v.Currency, &v.EffectiveFrom,
		&v.CreatedAt, &v.UpdatedAt, &v.DeletedAt, &v.Revision,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ============================================================================
// ProviderHealth
// ============================================================================

func scanProviderHealth(sc scannable) (*domain.ProviderHealth, error) {
	var h domain.ProviderHealth
	var openUntil sql.NullTime
	var lastCheckedAt sql.NullTime
	var lastLatencyMs sql.NullInt64
	err := sc.Scan(
		&h.ID, &h.ProviderID, &h.State, &h.ConsecutiveFailures,
		&openUntil, &lastCheckedAt, &lastLatencyMs, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if openUntil.Valid {
		t := openUntil.Time
		h.OpenUntil = &t
	}
	if lastCheckedAt.Valid {
		t := lastCheckedAt.Time
		h.LastCheckedAt = &t
	}
	if lastLatencyMs.Valid {
		v := int(lastLatencyMs.Int64)
		h.LastLatencyMs = &v
	}
	return &h, nil
}

// ============================================================================
// UsageLog
// ============================================================================

func scanUsageLog(sc scannable) (*domain.UsageLog, error) {
	var u domain.UsageLog
	var apiKeyID sql.NullString
	var priceVersionID sql.NullString
	var errorCode sql.NullString
	var ttftMs sql.NullInt64
	var totalLatencyMs sql.NullInt64
	var upstreamRequestID sql.NullString
	var failoverFrom sql.NullString
	err := sc.Scan(
		&u.ID, &u.OrganizationID, &apiKeyID, &u.AliasID, &u.ProviderID, &u.ModelID,
		&priceVersionID, &u.ProtocolFace, &u.AttemptNo,
		&u.InputTokens, &u.OutputTokens, &u.TotalTokens,
		&u.CostAmount, &u.Currency, &u.Status,
		&errorCode, &ttftMs, &totalLatencyMs,
		&upstreamRequestID, &failoverFrom, &u.RequestID, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if apiKeyID.Valid {
		v := apiKeyID.String
		u.APIKeyID = &v
	}
	if priceVersionID.Valid {
		v := priceVersionID.String
		u.PriceVersionID = &v
	}
	if errorCode.Valid {
		v := errorCode.String
		u.ErrorCode = &v
	}
	if ttftMs.Valid {
		v := int(ttftMs.Int64)
		u.TTFTMs = &v
	}
	if totalLatencyMs.Valid {
		v := int(totalLatencyMs.Int64)
		u.TotalLatencyMs = &v
	}
	if upstreamRequestID.Valid {
		v := upstreamRequestID.String
		u.UpstreamRequestID = &v
	}
	if failoverFrom.Valid {
		v := failoverFrom.String
		u.FailoverFrom = &v
	}
	return &u, nil
}

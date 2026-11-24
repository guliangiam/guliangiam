package aigateway

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockScanner handles sql.Null* types + plain primitives + **time.Time dest
// for MySQL scan unit tests. Pattern copied from payments/mapper_ext_test.go.
type mockScanner struct {
	values []interface{}
	err    error
}

func (m *mockScanner) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	if len(dest) > len(m.values) {
		return errors.New("not enough values")
	}
	for i, d := range dest {
		switch v := m.values[i].(type) {
		case string:
			if p, ok := d.(*string); ok {
				*p = v
			}
		case *string:
			if p, ok := d.(*string); ok && v != nil {
				*p = *v
			}
		case sql.NullString:
			switch p := d.(type) {
			case *sql.NullString:
				*p = v
			case *string:
				if v.Valid {
					*p = v.String
				}
			}
		case sql.NullFloat64:
			switch p := d.(type) {
			case *sql.NullFloat64:
				*p = v
			case *float64:
				if v.Valid {
					*p = v.Float64
				}
			}
		case sql.NullTime:
			switch p := d.(type) {
			case *sql.NullTime:
				*p = v
			case *time.Time:
				if v.Valid {
					*p = v.Time
				}
			}
		case sql.NullInt64:
			if p, ok := d.(*sql.NullInt64); ok {
				*p = v
			}
		case int:
			if p, ok := d.(*int); ok {
				*p = v
			}
		case int64:
			if p, ok := d.(*int64); ok {
				*p = v
			}
		case float64:
			if p, ok := d.(*float64); ok {
				*p = v
			}
		case bool:
			if p, ok := d.(*bool); ok {
				*p = v
			}
		case time.Time:
			switch p := d.(type) {
			case *time.Time:
				*p = v
			case **time.Time:
				t := v
				*p = &t
			}
		case []byte:
			switch p := d.(type) {
			case *[]byte:
				*p = v
			case *string:
				*p = string(v)
			}
		case nil:
			if pp, ok := d.(**time.Time); ok {
				*pp = nil
			}
			// skip everything else
		}
	}
	return nil
}

// compile-time guard
var _ scannable = (*mockScanner)(nil)

// ── scanLlmProvider ───────────────────────────────────────────────

func TestMySQLScanLlmProvider_AllFields(t *testing.T) {
	now := time.Now().UTC()
	supported := []string{"openai", "anthropic"}
	supportedJSON, _ := json.Marshal(supported)

	ms := &mockScanner{values: []interface{}{
		"prov-1", "org-1", "openai-primary", "OpenAI Primary", "openai",
		string(supportedJSON), "openai", "https://api.openai.com/v1",
		"sealed-key-abc", "fp-123456", "active",
		30000, 3, 100, 2,
		sql.NullInt64{Int64: 1000, Valid: true},
		sql.NullInt64{Int64: 200000, Valid: true},
		now.Add(-2 * time.Hour), now,
		nil, // deleted_at NULL
		int64(5),
	}}
	p, err := scanLlmProvider(ms)
	require.NoError(t, err)
	assert.Equal(t, "prov-1", p.ID)
	assert.Equal(t, "OpenAI Primary", p.DisplayName)
	assert.Equal(t, "openai", p.PreferredProtocol)
	assert.Equal(t, "https://api.openai.com/v1", p.BaseURL)
	assert.Equal(t, 30000, p.RequestTimeoutMs)
	assert.Equal(t, 3, p.MaxRetries)
	assert.Equal(t, 100, p.Priority)
	assert.Equal(t, 2, p.Weight)
	require.NotNil(t, p.RPMLimit)
	assert.Equal(t, 1000, *p.RPMLimit)
	require.NotNil(t, p.TPMLimit)
	assert.Equal(t, 200000, *p.TPMLimit)
	assert.Nil(t, p.DeletedAt)
	assert.Equal(t, int64(5), p.Revision)
	assert.Equal(t, []string{"openai", "anthropic"}, p.SupportedProtocols)
}

func TestMySQLScanLlmProvider_Minimal(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"prov-2", "org-1", "deepseek-1", "DeepSeek", "deepseek",
		"[]",     // supported_protocols empty JSON array
		"openai", // preferred_protocol
		"https://api.deepseek.com",
		"sealed-key-x", "fp-xyz", "active",
		15000, 2, 50, 1,
		sql.NullInt64{Valid: false}, // rpm_limit NULL
		sql.NullInt64{Valid: false}, // tpm_limit NULL
		now, now,
		nil, // deleted_at NULL
		int64(1),
	}}
	p, err := scanLlmProvider(ms)
	require.NoError(t, err)
	assert.Equal(t, "deepseek", p.ProviderType)
	assert.Nil(t, p.RPMLimit)
	assert.Nil(t, p.TPMLimit)
	assert.Empty(t, p.SupportedProtocols) // [] → 空切片
	assert.Nil(t, p.DeletedAt)
}

// ── scanLlmModel ──────────────────────────────────────────────────

func TestMySQLScanLlmModel_AllFields(t *testing.T) {
	now := time.Now().UTC()
	caps := map[string]any{"vision": true, "tools": true, "context_window": 128000}
	capsJSON, _ := json.Marshal(caps)

	ms := &mockScanner{values: []interface{}{
		"model-1", "org-1", "prov-1", "gpt-4o", "GPT-4o Turbo",
		string(capsJSON),
		sql.NullInt64{Int64: 128000, Valid: true}, // max_context_tokens
		sql.NullInt64{Int64: 4096, Valid: true},   // max_output_tokens
		"active",
		now.Add(-time.Hour), now,
		nil, // deleted_at NULL
		int64(3),
	}}
	m, err := scanLlmModel(ms)
	require.NoError(t, err)
	assert.Equal(t, "model-1", m.ID)
	assert.Equal(t, "gpt-4o", m.ProviderModelName)
	assert.Equal(t, "GPT-4o Turbo", m.DisplayName)
	assert.Equal(t, "active", m.Status)
	require.NotNil(t, m.MaxContextTokens)
	assert.Equal(t, 128000, *m.MaxContextTokens)
	require.NotNil(t, m.MaxOutputTokens)
	assert.Equal(t, 4096, *m.MaxOutputTokens)
	assert.Equal(t, int64(3), m.Revision)
	assert.Equal(t, true, m.Capabilities["vision"])
}

func TestMySQLScanLlmModel_Minimal(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"model-2", "org-1", "prov-2", "deepseek-chat", "DeepSeek Chat",
		"{}", // capabilities empty JSON object
		sql.NullInt64{Valid: false},
		sql.NullInt64{Valid: false},
		"active",
		now, now,
		nil,
		int64(1),
	}}
	m, err := scanLlmModel(ms)
	require.NoError(t, err)
	assert.Equal(t, "deepseek-chat", m.ProviderModelName)
	assert.Nil(t, m.MaxContextTokens)
	assert.Nil(t, m.MaxOutputTokens)
	assert.NotNil(t, m.Capabilities)
	assert.Nil(t, m.DeletedAt)
}

// ── scanLlmModelAlias ─────────────────────────────────────────────

func TestMySQLScanLlmModelAlias_AllFields(t *testing.T) {
	now := time.Now().UTC()
	deletedAt := now.Add(-time.Hour)

	ms := &mockScanner{values: []interface{}{
		"alias-1", "org-1", "gpt4", "GPT-4 Family",
		"disabled",
		now.Add(-3 * time.Hour), now.Add(-time.Hour),
		deletedAt, // deleted_at NOT NULL
		int64(2),
	}}
	a, err := scanLlmModelAlias(ms)
	require.NoError(t, err)
	assert.Equal(t, "alias-1", a.ID)
	assert.Equal(t, "gpt4", a.Alias)
	assert.Equal(t, "GPT-4 Family", a.DisplayName)
	assert.Equal(t, "disabled", a.Status)
	assert.Equal(t, int64(2), a.Revision)
	require.NotNil(t, a.DeletedAt)
	assert.Equal(t, deletedAt, *a.DeletedAt)
}

func TestMySQLScanLlmModelAlias_Minimal(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"alias-2", "org-1", "claude", "",
		"active",
		now, now,
		nil,
		int64(1),
	}}
	a, err := scanLlmModelAlias(ms)
	require.NoError(t, err)
	assert.Equal(t, "claude", a.Alias)
	assert.Empty(t, a.DisplayName)
	assert.Equal(t, "active", a.Status)
	assert.Nil(t, a.DeletedAt)
}

// ── scanLlmRoute ──────────────────────────────────────────────────

func TestMySQLScanLlmRoute_AllFields(t *testing.T) {
	now := time.Now().UTC()

	ms := &mockScanner{values: []interface{}{
		"route-1", "org-1", "alias-1", "model-1",
		70, 10, "active",
		now.Add(-2 * time.Hour), now,
		nil,
		int64(4),
	}}
	r, err := scanLlmRoute(ms)
	require.NoError(t, err)
	assert.Equal(t, "route-1", r.ID)
	assert.Equal(t, "alias-1", r.AliasID)
	assert.Equal(t, "model-1", r.ModelID)
	assert.Equal(t, 70, r.Weight)
	assert.Equal(t, 10, r.Priority)
	assert.Equal(t, "active", r.Status)
	assert.Equal(t, int64(4), r.Revision)
	assert.Nil(t, r.DeletedAt)
}

func TestMySQLScanLlmRoute_Minimal(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"route-2", "org-1", "alias-2", "model-2",
		30, 20, "active",
		now, now,
		nil,
		int64(1),
	}}
	r, err := scanLlmRoute(ms)
	require.NoError(t, err)
	assert.Equal(t, 30, r.Weight)
	assert.Equal(t, 20, r.Priority)
	assert.Equal(t, "active", r.Status)
}

// ── scanLlmApiKey ─────────────────────────────────────────────────

func TestMySQLScanLlmApiKey_AllFields(t *testing.T) {
	now := time.Now().UTC()
	expires := now.Add(30 * 24 * time.Hour)
	allowedAliases := []string{"gpt4", "claude"}
	allowedJSON, _ := json.Marshal(allowedAliases)

	ms := &mockScanner{values: []interface{}{
		"key-1", "org-1", "Production Key", "sk-abc***xyz", "hash-def123",
		string(allowedJSON),
		sql.NullInt64{Int64: 5000, Valid: true},        // rpm_limit
		sql.NullInt64{Int64: 500000, Valid: true},      // tpm_limit
		sql.NullFloat64{Float64: 1000.00, Valid: true}, // monthly_budget
		"USD", "active",
		expires, // expires_at NOT NULL (**time.Time)
		now.Add(-7 * 24 * time.Hour), now,
		nil, // deleted_at NULL
		int64(3),
	}}
	k, err := scanLlmApiKey(ms)
	require.NoError(t, err)
	assert.Equal(t, "key-1", k.ID)
	assert.Equal(t, "Production Key", k.Name)
	assert.Equal(t, "sk-abc***xyz", k.KeyPrefix)
	assert.Equal(t, "hash-def123", k.KeyHash)
	require.NotNil(t, k.RPMLimit)
	assert.Equal(t, 5000, *k.RPMLimit)
	require.NotNil(t, k.TPMLimit)
	assert.Equal(t, 500000, *k.TPMLimit)
	require.NotNil(t, k.MonthlyBudget)
	assert.Equal(t, 1000.00, *k.MonthlyBudget)
	assert.Equal(t, "USD", k.Currency)
	assert.Equal(t, "active", k.Status)
	require.NotNil(t, k.ExpiresAt)
	assert.Equal(t, expires, *k.ExpiresAt)
	assert.Equal(t, []string{"gpt4", "claude"}, k.AllowedAliases)
	assert.Nil(t, k.DeletedAt)
	assert.Equal(t, int64(3), k.Revision)
}

func TestMySQLScanLlmApiKey_Minimal(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"key-2", "org-1", "Dev Key", "sk-dev***aaa", "hash-dev",
		"[]", // allowed_aliases empty
		sql.NullInt64{Valid: false},
		sql.NullInt64{Valid: false},
		sql.NullFloat64{Valid: false},
		"USD", "active",
		nil, // expires_at NULL
		now, now,
		nil,
		int64(1),
	}}
	k, err := scanLlmApiKey(ms)
	require.NoError(t, err)
	assert.Nil(t, k.RPMLimit)
	assert.Nil(t, k.TPMLimit)
	assert.Nil(t, k.MonthlyBudget)
	assert.Nil(t, k.ExpiresAt)
	assert.Empty(t, k.AllowedAliases) // [] → 空切片
}

// ── scanLlmQuotaPolicy ────────────────────────────────────────────

func TestMySQLScanLlmQuotaPolicy_AllFields(t *testing.T) {
	now := time.Now().UTC()

	ms := &mockScanner{values: []interface{}{
		"qp-1", "org-1", "api_key",
		sql.NullString{String: "key-1", Valid: true}, // scope_ref NOT NULL
		"rpm", 600.00, "monthly", "active",
		now.Add(-2 * 24 * time.Hour), now,
		nil,
		int64(2),
	}}
	q, err := scanLlmQuotaPolicy(ms)
	require.NoError(t, err)
	assert.Equal(t, "qp-1", q.ID)
	assert.Equal(t, "api_key", q.Scope)
	require.NotNil(t, q.ScopeRef)
	assert.Equal(t, "key-1", *q.ScopeRef)
	assert.Equal(t, "rpm", q.LimitType)
	assert.Equal(t, 600.00, q.LimitValue)
	assert.Equal(t, "monthly", q.ResetCycle)
	assert.Equal(t, "active", q.Status)
	assert.Equal(t, int64(2), q.Revision)
	assert.Nil(t, q.DeletedAt)
}

func TestMySQLScanLlmQuotaPolicy_Minimal(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"qp-2", "org-1", "organization",
		sql.NullString{Valid: false}, // scope_ref NULL
		"cost", 10000.50, "monthly", "active",
		now, now,
		nil,
		int64(1),
	}}
	q, err := scanLlmQuotaPolicy(ms)
	require.NoError(t, err)
	assert.Equal(t, "organization", q.Scope)
	assert.Nil(t, q.ScopeRef)
	assert.Equal(t, "cost", q.LimitType)
	assert.Equal(t, 10000.50, q.LimitValue)
}

// ── scanLlmPriceVersion ────────────────────────────────────────────

func TestMySQLScanLlmPriceVersion_AllFields(t *testing.T) {
	now := time.Now().UTC()
	effective := now.Add(-24 * time.Hour)

	ms := &mockScanner{values: []interface{}{
		"pv-1", "org-1", "model-1", 2,
		2.50, 10.00, "USD",
		effective,
		now.Add(-48 * time.Hour), now,
		nil,
		int64(1),
	}}
	v, err := scanLlmPriceVersion(ms)
	require.NoError(t, err)
	assert.Equal(t, "pv-1", v.ID)
	assert.Equal(t, "model-1", v.ModelID)
	assert.Equal(t, 2, v.VersionNo)
	assert.Equal(t, 2.50, v.InputPricePer1M)
	assert.Equal(t, 10.00, v.OutputPricePer1M)
	assert.Equal(t, "USD", v.Currency)
	assert.Equal(t, effective, v.EffectiveFrom)
	assert.Equal(t, int64(1), v.Revision)
	assert.Nil(t, v.DeletedAt)
}

func TestMySQLScanLlmPriceVersion_Minimal(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"pv-2", "org-1", "model-2", 1,
		1.00, 5.00, "CNY",
		now,
		now, now,
		nil,
		int64(1),
	}}
	v, err := scanLlmPriceVersion(ms)
	require.NoError(t, err)
	assert.Equal(t, 1, v.VersionNo)
	assert.Equal(t, 1.00, v.InputPricePer1M)
	assert.Equal(t, 5.00, v.OutputPricePer1M)
	assert.Equal(t, "CNY", v.Currency)
}

// ── scanProviderHealth ─────────────────────────────────────────────
// Note: tenant-less — no organization_id. 8 dests total.

func TestMySQLScanProviderHealth_AllFields(t *testing.T) {
	now := time.Now().UTC()
	openUntil := now.Add(30 * time.Second)
	lastChecked := now.Add(-10 * time.Second)

	ms := &mockScanner{values: []interface{}{
		"health-1", "prov-1", "open", 5,
		sql.NullTime{Time: openUntil, Valid: true},   // open_until
		sql.NullTime{Time: lastChecked, Valid: true}, // last_checked_at
		sql.NullInt64{Int64: 123, Valid: true},       // last_latency_ms
		now,                                          // updated_at
	}}
	h, err := scanProviderHealth(ms)
	require.NoError(t, err)
	assert.Equal(t, "health-1", h.ID)
	assert.Equal(t, "prov-1", h.ProviderID)
	assert.Equal(t, "open", h.State)
	assert.Equal(t, 5, h.ConsecutiveFailures)
	require.NotNil(t, h.OpenUntil)
	assert.Equal(t, openUntil, *h.OpenUntil)
	require.NotNil(t, h.LastCheckedAt)
	assert.Equal(t, lastChecked, *h.LastCheckedAt)
	require.NotNil(t, h.LastLatencyMs)
	assert.Equal(t, 123, *h.LastLatencyMs)
	assert.Equal(t, now, h.UpdatedAt)
}

func TestMySQLScanProviderHealth_Minimal(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"health-2", "prov-2", "healthy", 0,
		sql.NullTime{Valid: false},  // open_until NULL
		sql.NullTime{Valid: false},  // last_checked_at NULL
		sql.NullInt64{Valid: false}, // last_latency_ms NULL
		now,
	}}
	h, err := scanProviderHealth(ms)
	require.NoError(t, err)
	assert.Equal(t, "healthy", h.State)
	assert.Equal(t, 0, h.ConsecutiveFailures)
	assert.Nil(t, h.OpenUntil)
	assert.Nil(t, h.LastCheckedAt)
	assert.Nil(t, h.LastLatencyMs)
}

// ── scanUsageLog ──────────────────────────────────────────────────
// 22 dests. Multiple nullable FK/sensor fields.

func TestMySQLScanUsageLog_AllFields(t *testing.T) {
	now := time.Now().UTC()

	ms := &mockScanner{values: []interface{}{
		"log-1", "org-1",
		sql.NullString{String: "key-1", Valid: true}, // api_key_id NOT NULL
		"alias-1", "prov-1", "model-1",
		sql.NullString{String: "pv-1", Valid: true}, // price_version_id NOT NULL
		"openai", 1,
		1000, 500, 1500,
		0.0025, "USD", "succeeded",
		sql.NullString{Valid: false},           // error_code NULL
		sql.NullInt64{Int64: 120, Valid: true}, // ttft_ms NOT NULL
		sql.NullInt64{Int64: 450, Valid: true}, // total_latency_ms NOT NULL
		sql.NullString{String: "upstream-req-abc", Valid: true},
		sql.NullString{Valid: false}, // failover_from NULL
		"req-12345",
		now,
	}}
	u, err := scanUsageLog(ms)
	require.NoError(t, err)
	assert.Equal(t, "log-1", u.ID)
	assert.Equal(t, "org-1", u.OrganizationID)
	require.NotNil(t, u.APIKeyID)
	assert.Equal(t, "key-1", *u.APIKeyID)
	assert.Equal(t, "alias-1", u.AliasID)
	assert.Equal(t, "prov-1", u.ProviderID)
	assert.Equal(t, "model-1", u.ModelID)
	require.NotNil(t, u.PriceVersionID)
	assert.Equal(t, "pv-1", *u.PriceVersionID)
	assert.Equal(t, "openai", u.ProtocolFace)
	assert.Equal(t, 1, u.AttemptNo)
	assert.Equal(t, 1000, u.InputTokens)
	assert.Equal(t, 500, u.OutputTokens)
	assert.Equal(t, 1500, u.TotalTokens)
	assert.Equal(t, 0.0025, u.CostAmount)
	assert.Equal(t, "USD", u.Currency)
	assert.Equal(t, "succeeded", u.Status)
	assert.Nil(t, u.ErrorCode)
	require.NotNil(t, u.TTFTMs)
	assert.Equal(t, 120, *u.TTFTMs)
	require.NotNil(t, u.TotalLatencyMs)
	assert.Equal(t, 450, *u.TotalLatencyMs)
	require.NotNil(t, u.UpstreamRequestID)
	assert.Equal(t, "upstream-req-abc", *u.UpstreamRequestID)
	assert.Nil(t, u.FailoverFrom)
	assert.Equal(t, "req-12345", u.RequestID)
	assert.Equal(t, now, u.CreatedAt)
}

func TestMySQLScanUsageLog_FailedAllNil(t *testing.T) {
	now := time.Now().UTC()

	ms := &mockScanner{values: []interface{}{
		"log-2", "org-1",
		sql.NullString{Valid: false}, // api_key_id NULL
		"alias-2", "prov-2", "model-2",
		sql.NullString{Valid: false}, // price_version_id NULL
		"anthropic", 2,
		2000, 1000, 3000,
		0.0000, "USD", "failed",
		sql.NullString{String: "E_TIMEOUT", Valid: true},
		sql.NullInt64{Valid: false}, // ttft_ms NULL
		sql.NullInt64{Valid: false}, // total_latency_ms NULL
		sql.NullString{Valid: false},
		sql.NullString{String: "prov-1", Valid: true}, // failover_from NOT NULL
		"req-99999",
		now,
	}}
	u, err := scanUsageLog(ms)
	require.NoError(t, err)
	assert.Equal(t, "log-2", u.ID)
	assert.Nil(t, u.APIKeyID)
	assert.Nil(t, u.PriceVersionID)
	assert.Equal(t, "anthropic", u.ProtocolFace)
	assert.Equal(t, 2, u.AttemptNo)
	assert.Equal(t, "failed", u.Status)
	require.NotNil(t, u.ErrorCode)
	assert.Equal(t, "E_TIMEOUT", *u.ErrorCode)
	assert.Nil(t, u.TTFTMs)
	assert.Nil(t, u.TotalLatencyMs)
	assert.Nil(t, u.UpstreamRequestID)
	require.NotNil(t, u.FailoverFrom)
	assert.Equal(t, "prov-1", *u.FailoverFrom)
	assert.Equal(t, "req-99999", u.RequestID)
}

package aigateway

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── fake pgx.Row ───────────────────────────────────────────────────────

// fakeRow implements pgx.Row by setting dest pointers from values in order.
// This is sufficient for testing scanLlm* / scanProviderHealth / scanUsageLog
// because each one calls row.Scan with dests in a fixed column order.
type fakeRow struct {
	values []any
	idx    int
	err    error // optional pre-existing error returned immediately
}

func newFakeRow(values ...any) *fakeRow { return &fakeRow{values: values} }

func (f *fakeRow) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	if len(dest) > len(f.values)-f.idx {
		// short read — just copy what's there; tests assert specific values
		for i, d := range dest {
			if f.idx+i >= len(f.values) {
				continue
			}
			setDest(d, f.values[f.idx+i])
		}
		f.idx += len(dest)
		return nil
	}
	for i, d := range dest {
		setDest(d, f.values[f.idx+i])
	}
	f.idx += len(dest)
	return nil
}

// setDest assigns src to *dest.
// Priority order:
//  1. If dest implements Scan(any) error → call that (our custom JSON types).
//  2. If src is nil → set dest to zero value.
//  3. If dest is a *T (dv.Elem().Kind() == Ptr):
//     a. If sv is also *U and assignable to *T → Set directly (handles *int nil → *int nil).
//     b. If sv is assignable to T → create new *T, set, assign.
//  4. Otherwise AssignableTo/Convert + Set for value fields.
func setDest(dest, src any) {
	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Pointer || dv.IsNil() {
		return
	}
	// Step 1: custom JSON Scan types like protocolsJSON
	if scanner, ok := dest.(interface{ Scan(any) error }); ok {
		_ = scanner.Scan(src)
		return
	}
	// Step 2: nil src → zero value
	if src == nil {
		dv.Elem().Set(reflect.Zero(dv.Elem().Type()))
		return
	}
	sv := reflect.ValueOf(src)

	// Step 3: dest is *T (optional/pointer field in domain struct)
	if dv.Elem().Kind() == reflect.Pointer {
		// 3a: sv is also *U and assignable to *T → direct set (handles nil pointer case)
		if sv.Type().AssignableTo(dv.Elem().Type()) {
			dv.Elem().Set(sv)
			return
		}
		// 3b: sv is assignable to T → wrap in pointer
		if dv.Elem().Elem().IsValid() && sv.Type().AssignableTo(dv.Elem().Elem().Type()) {
			ptr := reflect.New(dv.Elem().Elem().Type())
			ptr.Elem().Set(sv)
			dv.Elem().Set(ptr)
			return
		}
	}

	// Step 4: plain type assignment with optional Convert
	if sv.Type().AssignableTo(dv.Elem().Type()) {
		dv.Elem().Set(sv)
		return
	}
	if sv.Type().ConvertibleTo(dv.Elem().Type()) {
		dv.Elem().Set(sv.Convert(dv.Elem().Type()))
		return
	}
	// Give up — don't panic
}

// ── Helpers for test data ─────────────────────────────────────────────

var sampleTime = time.Date(2024, 10, 1, 12, 0, 0, 0, time.UTC)
var sampleTime2 = time.Date(2024, 10, 1, 13, 0, 0, 0, time.UTC)

// ── LlmProvider scan ───────────────────────────────────────────────────

func TestScanLlmProvider(t *testing.T) {
	maxCtx := 128000
	maxOut := 4096
	rpm := 1000
	supportedProtocols := []string{"openai", "anthropic"}

	row := newFakeRow(
		"p-1",                           // id
		"org-1",                         // organization_id
		"my-provider",                   // name
		"My Provider",                   // display_name
		"openai",                        // provider_type
		mustJSON(t, supportedProtocols), // supported_protocols (JSONB bytes)
		"openai",                        // preferred_protocol
		"https://api.openai.com",        // base_url
		"secret-key",                    // api_key_sealed
		"fp-abc",                        // api_key_fingerprint
		"active",                        // status
		30000,                           // request_timeout_ms
		2,                               // max_retries
		100,                             // priority
		10,                              // weight
		&rpm,                            // rpm_limit (*int)
		(*int)(nil),                     // tpm_limit (*int, NULL)
		sampleTime,                      // created_at
		sampleTime2,                     // updated_at
		(*time.Time)(nil),               // deleted_at (NULL)
		int64(1),                        // revision
	)

	p, err := scanLlmProvider(row)
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "p-1", p.ID)
	assert.Equal(t, "org-1", p.OrganizationID)
	assert.Equal(t, "my-provider", p.Name)
	assert.Equal(t, "My Provider", p.DisplayName)
	assert.Equal(t, "openai", p.ProviderType)
	assert.Equal(t, supportedProtocols, p.SupportedProtocols)
	assert.Equal(t, "openai", p.PreferredProtocol)
	assert.Equal(t, "https://api.openai.com", p.BaseURL)
	assert.Equal(t, "secret-key", p.APIKeySealed)
	assert.Equal(t, "fp-abc", p.APIKeyFingerprint)
	assert.Equal(t, "active", p.Status)
	assert.Equal(t, 30000, p.RequestTimeoutMs)
	assert.Equal(t, 2, p.MaxRetries)
	assert.Equal(t, 100, p.Priority)
	assert.Equal(t, 10, p.Weight)
	require.NotNil(t, p.RPMLimit)
	assert.Equal(t, 1000, *p.RPMLimit)
	assert.Nil(t, p.TPMLimit)
	assert.Equal(t, sampleTime, p.CreatedAt)
	assert.Equal(t, sampleTime2, p.UpdatedAt)
	assert.Nil(t, p.DeletedAt)
	assert.Equal(t, int64(1), p.Revision)

	_ = maxCtx
	_ = maxOut
}

// ── LlmModel scan ──────────────────────────────────────────────────────

func TestScanLlmModel(t *testing.T) {
	maxCtx := 128000
	maxOut := 4096
	caps := map[string]any{"function_calling": true, "vision": false}

	row := newFakeRow(
		"m-1",             // id
		"org-1",           // organization_id
		"p-1",             // provider_id
		"gpt-4o-2024",     // provider_model_name
		"GPT-4o",          // display_name
		mustJSON(t, caps), // capabilities
		&maxCtx,           // max_context_tokens (nullable pointer)
		&maxOut,           // max_output_tokens
		"active",          // status
		sampleTime,        // created_at
		sampleTime2,       // updated_at
		(*time.Time)(nil), // deleted_at
		int64(1),          // revision
	)

	m, err := scanLlmModel(row)
	require.NoError(t, err)
	require.NotNil(t, m)

	assert.Equal(t, "m-1", m.ID)
	assert.Equal(t, "p-1", m.ProviderID)
	assert.Equal(t, "gpt-4o-2024", m.ProviderModelName)
	assert.Equal(t, caps, m.Capabilities)
	assert.NotNil(t, m.MaxContextTokens)
	assert.Equal(t, 128000, *m.MaxContextTokens)
	assert.NotNil(t, m.MaxOutputTokens)
	assert.Equal(t, 4096, *m.MaxOutputTokens)
	assert.Equal(t, "active", m.Status)
}

// ── LlmModelAlias scan ─────────────────────────────────────────────────

func TestScanLlmModelAlias(t *testing.T) {
	row := newFakeRow(
		"a-1", "org-1", "gpt-4o", "Production GPT-4o", "active",
		sampleTime, sampleTime2, (*time.Time)(nil), int64(3),
	)

	a, err := scanLlmModelAlias(row)
	require.NoError(t, err)
	require.NotNil(t, a)

	assert.Equal(t, "a-1", a.ID)
	assert.Equal(t, "org-1", a.OrganizationID)
	assert.Equal(t, "gpt-4o", a.Alias)
	assert.Equal(t, "Production GPT-4o", a.DisplayName)
	assert.Equal(t, "active", a.Status)
	assert.Equal(t, int64(3), a.Revision)
}

// ── LlmRoute scan ──────────────────────────────────────────────────────

func TestScanLlmRoute(t *testing.T) {
	row := newFakeRow(
		"r-1", "org-1", "a-1", "m-1",
		10, 100, "active",
		sampleTime, sampleTime2, (*time.Time)(nil), int64(1),
	)

	r, err := scanLlmRoute(row)
	require.NoError(t, err)
	require.NotNil(t, r)

	assert.Equal(t, "r-1", r.ID)
	assert.Equal(t, "a-1", r.AliasID)
	assert.Equal(t, "m-1", r.ModelID)
	assert.Equal(t, 10, r.Weight)
	assert.Equal(t, 100, r.Priority)
}

// ── LlmApiKey scan ─────────────────────────────────────────────────────

func TestScanLlmApiKey(t *testing.T) {
	allowed := []string{"gpt-4o", "gpt-4o-mini"}
	expires := sampleTime2
	budget := 9999.99
	rpm := 1000

	row := newFakeRow(
		"k-1", "org-1", "my-api-key", "sk-test-abc", "hash-xyz",
		mustJSON(t, allowed),
		&rpm,        // rpm_limit (*int)
		(*int)(nil), // tpm_limit (*int, NULL)
		&budget,     // monthly_budget (*float64)
		"USD", "active", &expires,
		sampleTime, sampleTime2, (*time.Time)(nil), int64(1),
	)

	k, err := scanLlmApiKey(row)
	require.NoError(t, err)
	require.NotNil(t, k)

	assert.Equal(t, "k-1", k.ID)
	assert.Equal(t, "my-api-key", k.Name)
	assert.Equal(t, "sk-test-abc", k.KeyPrefix)
	assert.Equal(t, "hash-xyz", k.KeyHash)
	assert.Equal(t, allowed, k.AllowedAliases)
	require.NotNil(t, k.MonthlyBudget)
	assert.InDelta(t, 9999.99, *k.MonthlyBudget, 0.001)
	assert.Equal(t, "USD", k.Currency)
	assert.Equal(t, "active", k.Status)
	require.NotNil(t, k.ExpiresAt)
	assert.Equal(t, expires, *k.ExpiresAt)
}

// ── LlmQuotaPolicy scan ────────────────────────────────────────────────

func TestScanLlmQuotaPolicy(t *testing.T) {
	scopeRef := "k-1"
	row := newFakeRow(
		"q-1", "org-1", "api_key", &scopeRef,
		"rpm", float64(60), "minute", "active",
		sampleTime, sampleTime2, (*time.Time)(nil), int64(1),
	)

	q, err := scanLlmQuotaPolicy(row)
	require.NoError(t, err)
	require.NotNil(t, q)

	assert.Equal(t, "q-1", q.ID)
	assert.Equal(t, "api_key", q.Scope)
	require.NotNil(t, q.ScopeRef)
	assert.Equal(t, "k-1", *q.ScopeRef)
	assert.Equal(t, float64(60), q.LimitValue)
	assert.Equal(t, "rpm", q.LimitType)
	assert.Equal(t, "minute", q.ResetCycle)
	assert.Equal(t, "active", q.Status)
}

// ── ProviderHealth scan ────────────────────────────────────────────────
func TestScanLlmPriceVersion(t *testing.T) {
	eff := sampleTime
	row := newFakeRow(
		"pv-1", "org-1", "m-1", int(1),
		5.0, 15.0, "USD", eff,
		sampleTime, sampleTime2, (*time.Time)(nil), int64(1),
	)

	v, err := scanLlmPriceVersion(row)
	require.NoError(t, err)
	require.NotNil(t, v)

	assert.Equal(t, "pv-1", v.ID)
	assert.Equal(t, "m-1", v.ModelID)
	assert.Equal(t, 1, v.VersionNo)
	assert.InDelta(t, 5.0, v.InputPricePer1M, 0.001)
	assert.InDelta(t, 15.0, v.OutputPricePer1M, 0.001)
	assert.Equal(t, "USD", v.Currency)
	assert.Equal(t, eff, v.EffectiveFrom)
}

// ── ProviderHealth scan (no organization_id / revision) ───────────────

func TestScanProviderHealth(t *testing.T) {
	openUntil := sampleTime2
	latency := 150

	row := newFakeRow(
		"h-1", "p-1", "healthy", int(3),
		&openUntil, sampleTime, &latency, sampleTime2,
	)

	h, err := scanProviderHealth(row)
	require.NoError(t, err)
	require.NotNil(t, h)

	assert.Equal(t, "h-1", h.ID)
	assert.Equal(t, "p-1", h.ProviderID)
	assert.Equal(t, "healthy", h.State)
	assert.Equal(t, 3, h.ConsecutiveFailures)
	require.NotNil(t, h.OpenUntil)
	assert.Equal(t, openUntil, *h.OpenUntil)
	require.NotNil(t, h.LastLatencyMs)
	assert.Equal(t, 150, *h.LastLatencyMs)
	assert.Equal(t, sampleTime2, h.UpdatedAt)
}

// ── UsageLog scan ──────────────────────────────────────────────────────

func TestScanUsageLog(t *testing.T) {
	priceVersionID := "pv-1"
	errCode := "UPSTREAM_500"
	upstreamID := "upstream-req-xyz"
	failoverFrom := "p-2"
	apiKeyID := "k-1"
	ttft := 100
	totalLat := 500

	row := newFakeRow(
		"u-1", "org-1", &apiKeyID, "a-1", "p-1", "m-1",
		&priceVersionID,        // price_version_id (*string)
		"openai",               // protocol_face
		int(1),                 // attempt_no
		int(5), int(3), int(8), // input/output/total tokens
		0.00002, "USD", "ok", // cost
		&errCode,      // error_code (*string)
		&ttft,         // ttft_ms (*int)
		&totalLat,     // total_latency_ms (*int)
		&upstreamID,   // upstream_request_id (*string)
		&failoverFrom, // failover_from (*string)
		"req-id-123",
		sampleTime,
	)

	u, err := scanUsageLog(row)
	require.NoError(t, err)
	require.NotNil(t, u)

	assert.Equal(t, "u-1", u.ID)
	assert.Equal(t, "org-1", u.OrganizationID)
	require.NotNil(t, u.APIKeyID)
	assert.Equal(t, "k-1", *u.APIKeyID)
	assert.Equal(t, "a-1", u.AliasID)
	assert.Equal(t, "openai", u.ProtocolFace)
	assert.Equal(t, 1, u.AttemptNo)
	assert.Equal(t, 5, u.InputTokens)
	assert.Equal(t, 3, u.OutputTokens)
	assert.Equal(t, 8, u.TotalTokens)
	assert.InDelta(t, 0.00002, u.CostAmount, 0.000000001)
	assert.Equal(t, "ok", u.Status)
	require.NotNil(t, u.UpstreamRequestID)
	assert.Equal(t, "upstream-req-xyz", *u.UpstreamRequestID)
	require.NotNil(t, u.FailoverFrom)
	assert.Equal(t, "p-2", *u.FailoverFrom)
	assert.Equal(t, "req-id-123", u.RequestID)
}

// ── JSON custom Scan types ─────────────────────────────────────────────

func TestProtocolsJSON_Scan_FromBytes(t *testing.T) {
	var j protocolsJSON
	require.NoError(t, j.Scan([]byte(`["openai","anthropic"]`)))
	assert.Equal(t, protocolsJSON{"openai", "anthropic"}, j)
}

func TestProtocolsJSON_Scan_FromString(t *testing.T) {
	var j protocolsJSON
	require.NoError(t, j.Scan(`["openai"]`))
	assert.Equal(t, protocolsJSON{"openai"}, j)
}

func TestProtocolsJSON_Scan_Nil(t *testing.T) {
	var j protocolsJSON
	require.NoError(t, j.Scan(nil))
	assert.Nil(t, j)
}

func TestProtocolsJSON_Scan_UnsupportedType(t *testing.T) {
	var j protocolsJSON
	err := j.Scan(12345)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestCapabilitiesJSON_Scan_FromBytes(t *testing.T) {
	var j capabilitiesJSON
	require.NoError(t, j.Scan([]byte(`{"vision":true,"function_calling":false}`)))
	assert.Equal(t, capabilitiesJSON{"vision": true, "function_calling": false}, j)
}

func TestCapabilitiesJSON_Scan_Nil(t *testing.T) {
	var j capabilitiesJSON
	require.NoError(t, j.Scan(nil))
	assert.Nil(t, j)
}

func TestCapabilitiesJSON_Scan_UnsupportedType(t *testing.T) {
	var j capabilitiesJSON
	err := j.Scan(float64(3.14))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestApiKeyAliasesJSON_Scan_FromBytes(t *testing.T) {
	var j apiKeyAliasesJSON
	require.NoError(t, j.Scan([]byte(`["gpt-4o","claude"]`)))
	assert.Equal(t, apiKeyAliasesJSON{"gpt-4o", "claude"}, j)
}

func TestApiKeyAliasesJSON_Scan_Empty(t *testing.T) {
	var j apiKeyAliasesJSON
	require.NoError(t, j.Scan([]byte(`[]`)))
	assert.Equal(t, apiKeyAliasesJSON{}, j) // empty slice, not nil
}

func TestApiKeyAliasesJSON_Scan_Nil(t *testing.T) {
	var j apiKeyAliasesJSON
	require.NoError(t, j.Scan(nil))
	assert.Nil(t, j)
}

// ── Helpers ────────────────────────────────────────────────────────────

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

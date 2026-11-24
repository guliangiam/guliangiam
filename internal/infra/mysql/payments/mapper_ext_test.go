package payments

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockScanner handles sql.Null* types + plain primitives for MySQL scan tests.
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
			if p, ok := d.(*time.Time); ok {
				*p = v
			}
		case []byte:
			switch p := d.(type) {
			case *[]byte:
				*p = v
			case *string:
				*p = string(v)
			}
		case nil:
			// skip
		}
	}
	return nil
}

// compile-time guard
var _ scannable = (*mockScanner)(nil)

// ── scanCallbackReceipt ────────────────────────────────────────────

func TestMySQLScanCallbackReceipt_AllFields(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"receipt-1", "org-1", "alipay", "sha256", []byte("payload"),
		sql.NullString{String: `{"sign":"abc"}`, Valid: true}, now,
		sql.NullString{String: "127.0.0.1", Valid: true}, "accepted",
		sql.NullString{String: "event-1", Valid: true},
		sql.NullString{String: "out-1", Valid: true},
		sql.NullString{String: `{"state":"succeeded"}`, Valid: true},
		sql.NullString{Valid: false}, now, now.Add(time.Second), int64(2),
	}}
	rec, err := scanCallbackReceipt(ms)
	require.NoError(t, err)
	assert.Equal(t, "receipt-1", rec.ID)
	assert.Equal(t, "abc", rec.Headers["sign"])
	assert.Equal(t, "succeeded", rec.RawEvent["state"])
	assert.Equal(t, now, rec.CreatedAt)
	assert.Equal(t, now.Add(time.Second), rec.UpdatedAt)
	assert.Equal(t, int64(2), rec.Revision)
}

// ── scanPayAttempt ─────────────────────────────────────────────────

func TestMySQLScanPayAttempt_AllFields(t *testing.T) {
	now := time.Now().UTC()
	refundID := "refund-1"
	orderNo := "ORD-001"
	reqID := "REQ-001"
	amount := 99.99
	currency := "CNY"
	errCode := "E1000"
	errMsg := "timeout"
	latency := int64(150)
	rawMap := map[string]any{"code": "SUCCESS", "trade_no": "T-001"}
	rawBytes, _ := json.Marshal(rawMap)

	ms := &mockScanner{values: []interface{}{
		"att-1", "org-1", "pay-1",
		sql.NullString{String: refundID, Valid: true},
		"alipay", "charge", 2,
		sql.NullString{String: orderNo, Valid: true},
		sql.NullString{String: reqID, Valid: true},
		"succeeded",
		sql.NullFloat64{Float64: amount, Valid: true},
		sql.NullString{String: currency, Valid: true},
		sql.NullString{String: errCode, Valid: true},
		sql.NullString{String: errMsg, Valid: true},
		sql.NullInt64{Int64: latency, Valid: true},
		now,
		sql.NullString{String: string(rawBytes), Valid: true}, // raw_response as TEXT
	}}
	a, err := scanPayAttempt(ms)
	require.NoError(t, err)
	assert.Equal(t, "att-1", a.ID)
	require.NotNil(t, a.RefundID)
	assert.Equal(t, "refund-1", *a.RefundID)
	assert.Equal(t, domain.AttemptStateSucceeded, a.State)
	require.NotNil(t, a.Amount)
	assert.Equal(t, 99.99, *a.Amount)
	assert.Equal(t, "SUCCESS", a.RawResponse["code"])
}

func TestMySQLScanPayAttempt_NilRefund(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"att-2", "org-1", "pay-1",
		sql.NullString{Valid: false}, // refund_id NULL
		"wxpay", "charge", 1,
		sql.NullString{Valid: false},
		sql.NullString{Valid: false},
		"pending",
		sql.NullFloat64{Valid: false},
		sql.NullString{Valid: false},
		sql.NullString{Valid: false},
		sql.NullString{Valid: false},
		sql.NullInt64{Valid: false},
		now,
		sql.NullString{Valid: false}, // raw_response NULL → no JSON
	}}
	a, err := scanPayAttempt(ms)
	require.NoError(t, err)
	assert.Nil(t, a.RefundID)
	assert.Equal(t, domain.AttemptStatePending, a.State)
	assert.Empty(t, a.RawResponse)
}

// ── scanReconBatch ────────────────────────────────────────────────

func TestMySQLScanReconBatch(t *testing.T) {
	now := time.Now().UTC()
	batchDate := now.AddDate(0, 0, -1)
	deletedAt := now

	t.Run("all fields", func(t *testing.T) {
		ms := &mockScanner{values: []interface{}{
			"rb-ali-20251001", "org-1", "alipay", batchDate, "completed",
			3, 100, 98,
			now, now,
			sql.NullTime{Time: deletedAt, Valid: true},
			int64(2),
		}}
		b, err := scanReconBatch(ms)
		require.NoError(t, err)
		assert.Equal(t, "rb-ali-20251001", b.ID)
		assert.Equal(t, 3, b.DiffCount)
		assert.Equal(t, int64(2), b.Revision)
		require.NotNil(t, b.DeletedAt)
	})
	t.Run("no deletedAt", func(t *testing.T) {
		ms := &mockScanner{values: []interface{}{
			"rb-wx-20251001", "org-1", "wechat", time.Now(), "pending",
			0, 50, 50,
			time.Now(), time.Now(),
			sql.NullTime{Valid: false},
			int64(1),
		}}
		b, err := scanReconBatch(ms)
		require.NoError(t, err)
		assert.Nil(t, b.DeletedAt)
	})
}

// ── scanReconItem ─────────────────────────────────────────────────

func TestMySQLScanReconItem(t *testing.T) {
	now := time.Now().UTC()
	orderNo := "CH-001"
	payID := "pay-1"
	chAmt := 100.00
	la := 101.00
	chSt := "succeeded"
	ls := "failed"
	approvedBy := "reviewer"
	approvedAt := now.Add(2 * time.Minute)
	resolvedBy := "admin"
	resolvedAt := now.Add(time.Minute)
	note := "amount mismatch reviewed"

	ms := &mockScanner{values: []interface{}{
		"ri-1", "rb-test-20251001", "amount_mismatch",
		sql.NullString{String: orderNo, Valid: true},
		sql.NullString{String: payID, Valid: true},
		sql.NullFloat64{Float64: chAmt, Valid: true},
		sql.NullFloat64{Float64: la, Valid: true},
		sql.NullString{String: chSt, Valid: true},
		sql.NullString{String: ls, Valid: true},
		true,
		"approved",
		sql.NullString{String: approvedBy, Valid: true},
		sql.NullTime{Time: approvedAt, Valid: true},
		sql.NullString{String: resolvedBy, Valid: true},
		sql.NullTime{Time: resolvedAt, Valid: true},
		sql.NullString{String: note, Valid: true},
		now,
	}}
	i, err := scanReconItem(ms)
	require.NoError(t, err)
	assert.Equal(t, "ri-1", i.ID)
	assert.True(t, i.Resolved)
	require.NotNil(t, i.ResolvedBy)
	assert.Equal(t, "admin", *i.ResolvedBy)
	require.NotNil(t, i.Note)
	assert.Equal(t, "amount mismatch reviewed", *i.Note)
}

// ── scanChannelConfig (17 cols) ──────────────────────────────────

// SELECT: id, organization_id, provider_id, channel, environment, config_sealed,
//         config_fingerprint, config_set, version_no, status, test_passed_at,
//         activated_at, disabled_at, created_at, updated_at, deleted_at, revision

func TestMySQLScanChannelConfig_AllFields(t *testing.T) {
	now := time.Now().UTC()
	testPassed := now.Add(-time.Hour)
	activated := now.Add(-30 * time.Minute)
	disabled := now
	configSetBytes, _ := json.Marshal([]string{"AppID", "PrivateKey"})

	ms := &mockScanner{values: []interface{}{
		"cc-1", "org-1", sql.NullString{String: "provider-1", Valid: true}, "alipay", "production",
		"sealed-k1", "fp-abc12345",
		sql.NullString{String: string(configSetBytes), Valid: true}, // config_set TEXT(JSON)
		3, "active",
		sql.NullTime{Time: testPassed, Valid: true}, // test_passed_at
		sql.NullTime{Time: activated, Valid: true},  // activated_at
		sql.NullTime{Time: disabled, Valid: true},   // disabled_at
		now.Add(-2 * time.Hour),                     // created_at
		now,                                         // updated_at
		sql.NullTime{Valid: false},                  // deleted_at
		int64(3),                                    // revision
	}}
	cfg, err := scanChannelConfig(ms)
	require.NoError(t, err)
	assert.Equal(t, "cc-1", cfg.ID)
	assert.Equal(t, "provider-1", cfg.ProviderID)
	assert.Equal(t, domain.ChannelConfigStatusActive, cfg.Status)
	assert.Equal(t, 3, cfg.VersionNo)
	assert.NotEmpty(t, cfg.ConfigFingerprint)
	assert.Contains(t, cfg.ConfigSet, "AppID")
	require.NotNil(t, cfg.TestPassedAt)
	require.NotNil(t, cfg.ActivatedAt)
	require.NotNil(t, cfg.DisabledAt)
	assert.Nil(t, cfg.DeletedAt)
	assert.Equal(t, int64(3), cfg.Revision)
}

func TestMySQLScanChannelConfig_Minimal(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockScanner{values: []interface{}{
		"cc-2", "org-1", sql.NullString{Valid: false}, "wxpay", "sandbox",
		"sealed-x", "",
		sql.NullString{Valid: false}, // config_set NULL
		1, "draft",
		sql.NullTime{Valid: false}, // test_passed_at
		sql.NullTime{Valid: false}, // activated_at
		sql.NullTime{Valid: false}, // disabled_at
		now,
		sql.NullTime{Valid: false}, // updated_at
		sql.NullTime{Valid: false}, // deleted_at
		int64(1),
	}}
	cfg, err := scanChannelConfig(ms)
	require.NoError(t, err)
	assert.Empty(t, cfg.ProviderID)
	assert.Equal(t, domain.ChannelConfigStatusDraft, cfg.Status)
	assert.Equal(t, 1, cfg.VersionNo)
	assert.Empty(t, cfg.ConfigFingerprint) // 空 string 不影响
	assert.Empty(t, cfg.ConfigSet)         // NULL config_set
	assert.Nil(t, cfg.ActivatedAt)
}

package payments

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockScannerEx handles the richer type set our scan functions expect
// (int, *int, *float64, *time.Time, *string, etc.).
type mockScannerEx struct {
	values []interface{}
	err    error
}

func (m *mockScannerEx) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	if len(dest) > len(m.values) {
		return errors.New("not enough values")
	}
	for i, d := range dest {
		switch v := m.values[i].(type) {
		case int:
			if ptr, ok := d.(*int); ok {
				*ptr = v
			}
		case int64:
			if ptr, ok := d.(*int64); ok {
				*ptr = v
			}
		case float64:
			if ptr, ok := d.(*float64); ok {
				*ptr = v
			}
		case *float64:
			if ptr, ok := d.(**float64); ok {
				*ptr = v
			}
		case string:
			if ptr, ok := d.(*string); ok {
				*ptr = v
			}
		case []byte:
			if ptr, ok := d.(*[]byte); ok {
				*ptr = v
			}
		case *[]byte:
			if ptr, ok := d.(**[]byte); ok {
				*ptr = v
			}
		case bool:
			if ptr, ok := d.(*bool); ok {
				*ptr = v
			}
		case time.Time:
			if ptr, ok := d.(*time.Time); ok {
				*ptr = v
			}
		case *string:
			if ptr, ok := d.(**string); ok {
				*ptr = v
			}
		case *time.Time:
			if ptr, ok := d.(**time.Time); ok {
				*ptr = v
			}
		case nil:
			// skip nil values
		}
	}
	return nil
}

// ── scanCallbackReceipt tests ───────────────────────────────────────

func TestScanCallbackReceipt_AllFields(t *testing.T) {
	now := time.Now().UTC()
	headers := []byte(`{"sign":"abc"}`)
	rawEvent := []byte(`{"state":"succeeded"}`)
	clientIP := "127.0.0.1"
	eventKey := "event-1"
	outTradeNo := "out-1"
	ex := &mockScannerEx{values: []interface{}{
		"receipt-1", "org-1", "alipay", "sha256", []byte("payload"),
		&headers, now, &clientIP, "accepted", &eventKey, &outTradeNo,
		&rawEvent, (*string)(nil), now, now.Add(time.Second), int64(2),
	}}
	rec, err := (CallbackReceiptsRepository{}).scan(ex)
	require.NoError(t, err)
	assert.Equal(t, "receipt-1", rec.ID)
	assert.Equal(t, "abc", rec.Headers["sign"])
	assert.Equal(t, "succeeded", rec.RawEvent["state"])
	assert.Equal(t, now, rec.CreatedAt)
	assert.Equal(t, now.Add(time.Second), rec.UpdatedAt)
	assert.Equal(t, int64(2), rec.Revision)
}

// ── scanPayAttempt tests ────────────────────────────────────────────

func TestScanPayAttempt_AllFields(t *testing.T) {
	now := time.Now().UTC()
	refundID := "refund-1"
	orderNo := "ORD-001"
	reqID := "REQ-001"
	amount := 99.99
	currency := "CNY"
	errCode := "E1000"
	errMsg := "timeout"
	latency := 150
	rawBytes, _ := json.Marshal(map[string]any{"code": "SUCCESS"})

	ex := &mockScannerEx{values: []interface{}{
		"att-1", "org-1", "pay-1", &refundID, "alipay",
		"charge", 2, &orderNo, &reqID,
		"succeeded", &amount, &currency, &errCode, &errMsg, &latency,
		now, rawBytes,
	}}
	a, err := scanPayAttempt(ex)
	require.NoError(t, err)
	assert.Equal(t, "att-1", a.ID)
	assert.Equal(t, "org-1", a.OrganizationID)
	assert.Equal(t, "pay-1", a.PaymentID)
	require.NotNil(t, a.RefundID)
	assert.Equal(t, "refund-1", *a.RefundID)
	assert.Equal(t, "alipay", a.Channel)
	assert.Equal(t, "charge", a.AttemptType)
	assert.Equal(t, 2, a.AttemptNo)
	require.NotNil(t, a.ChannelOrderNo)
	assert.Equal(t, "ORD-001", *a.ChannelOrderNo)
	assert.Equal(t, domain.AttemptState("succeeded"), a.State)
	require.NotNil(t, a.Amount)
	assert.Equal(t, 99.99, *a.Amount)
	require.NotNil(t, a.Currency)
	assert.Equal(t, "CNY", *a.Currency)
	assert.Equal(t, now, a.OccurredAt)
	assert.Equal(t, "SUCCESS", a.RawResponse["code"])
}

func TestScanPayAttempt_NilRefundID(t *testing.T) {
	now := time.Now().UTC()
	ex := &mockScannerEx{values: []interface{}{
		"att-2", "org-1", "pay-1", nil, "wxpay",
		"charge", 1, nil, nil,
		"pending", nil, nil, nil, nil, nil,
		now, nil,
	}}
	a, err := scanPayAttempt(ex)
	require.NoError(t, err)
	assert.Nil(t, a.RefundID)
	assert.Equal(t, domain.AttemptStatePending, a.State)
	assert.Empty(t, a.RawResponse) // nil JSONB → empty map
}

// ── scanPayChannelConfig tests ───────────────────────────────────────

func TestScanPayChannelConfig_ProviderID(t *testing.T) {
	now := time.Now().UTC()
	providerID := "provider-1"
	ex := &mockScannerEx{values: []interface{}{
		"detail-1", "org-1", &providerID, "alipay", "production",
		"sealed", "fingerprint", []byte(`["AppID"]`), 2, "active",
		nil, nil, nil, now, now, nil, int64(1),
	}}
	config, err := scanPayChannelConfig(ex)
	require.NoError(t, err)
	assert.Equal(t, "provider-1", config.ProviderID)
	assert.Equal(t, []string{"AppID"}, config.ConfigSet)
}

func TestScanPayChannelConfig_NullProviderID(t *testing.T) {
	now := time.Now().UTC()
	ex := &mockScannerEx{values: []interface{}{
		"detail-1", "org-1", nil, "alipay", "production",
		"sealed", "fingerprint", nil, 1, "draft",
		nil, nil, nil, now, now, nil, int64(1),
	}}
	config, err := scanPayChannelConfig(ex)
	require.NoError(t, err)
	assert.Empty(t, config.ProviderID)
}

// ── scanReconBatch tests ─────────────────────────────────────────────

func TestScanReconBatch_AllFields(t *testing.T) {
	now := time.Now().UTC()
	batchDate := now.AddDate(0, 0, -1)
	deletedAt := now
	ex := &mockScannerEx{values: []interface{}{
		"rb-ali-20251001", "org-1", "alipay", batchDate, "completed",
		3, 100, 98,
		now, now, &deletedAt, int64(2),
	}}
	b, err := scanReconBatch(ex)
	require.NoError(t, err)
	assert.Equal(t, "rb-ali-20251001", b.ID)
	assert.Equal(t, "alipay", b.Channel)
	assert.Equal(t, batchDate, b.BatchDate)
	assert.Equal(t, "completed", b.Status)
	assert.Equal(t, 3, b.DiffCount)
	assert.Equal(t, int64(2), b.Revision)
	require.NotNil(t, b.DeletedAt)
}

func TestScanReconBatch_NoDeletedAt(t *testing.T) {
	ex := &mockScannerEx{values: []interface{}{
		"rb-wx-20251001", "org-1", "wechat", time.Now(), "pending",
		0, 50, 50,
		time.Now(), time.Now(), nil, int64(1),
	}}
	b, err := scanReconBatch(ex)
	require.NoError(t, err)
	assert.Nil(t, b.DeletedAt)
}

// ── scanReconItem tests ──────────────────────────────────────────────

func TestScanReconItem_AllFields(t *testing.T) {
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

	ex := &mockScannerEx{values: []interface{}{
		"ri-1", "rb-test-20251001", "amount_mismatch", &orderNo, &payID,
		&chAmt, &la, &chSt, &ls,
		true, "approved", &approvedBy, &approvedAt, &resolvedBy, &resolvedAt, &note, now,
	}}
	i, err := scanReconItem(ex)
	require.NoError(t, err)
	assert.Equal(t, "ri-1", i.ID)
	assert.Equal(t, "amount_mismatch", i.DiffType)
	assert.True(t, i.Resolved)
	require.NotNil(t, i.ResolvedBy)
	assert.Equal(t, "admin", *i.ResolvedBy)
	require.NotNil(t, i.Note)
	assert.Equal(t, "amount mismatch reviewed", *i.Note)
}

func TestScanReconItem_Unresolved(t *testing.T) {
	now := time.Now().UTC()
	ex := &mockScannerEx{values: []interface{}{
		"ri-2", "rb-test-20251001", "missing_channel", nil, nil,
		nil, nil, nil, nil,
		false, "pending", nil, nil, nil, nil, nil, now,
	}}
	i, err := scanReconItem(ex)
	require.NoError(t, err)
	assert.False(t, i.Resolved)
	assert.Nil(t, i.ChannelOrderNo)
	assert.Nil(t, i.ResolvedBy)
}

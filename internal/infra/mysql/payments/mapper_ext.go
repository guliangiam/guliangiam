package payments

import (
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─── scannable 是 sql.Row 和 sql.Rows 的公共接口 ──────────────────────────
// mysql/payments/repository.go 已经定义了 scannable，这里复用。

// scanPaymentExtended 扫描带 6 个增量列的 Payment（channel_order_no / attempt_count / refunded_amount / refunding_amount / last_error_code / last_error_message）。
// SELECT 列顺序：与 queryGetByChannelOrderNo / queryGetByOutTradeNo 严格一致（38 列）。
func scanPaymentExtended(sc scannable) (*domain.Payment, error) {
	var p domain.Payment
	var productsStr string
	var idempotencyKey sql.NullString
	err := sc.Scan(
		&p.ID, &p.OrganizationID, &p.Name, &p.DisplayName, &p.Provider, &p.Type,
		&productsStr, &p.ProductName, &p.ProductDisplayName, &p.Detail,
		&p.Currency, &p.Price, &p.User, &p.PersonName, &p.PersonIdCard,
		&p.PersonEmail, &p.PersonPhone, &p.InvoiceType, &p.InvoiceTitle,
		&p.InvoiceTaxId, &p.InvoiceRemark, &p.InvoiceUrl, &p.Order,
		&p.OutOrderId, &p.PayUrl, &p.SuccessUrl, &p.State, &p.Message,
		&idempotencyKey,
		&p.ChannelOrderNo, &p.AttemptCount, &p.RefundedAmount, &p.RefundingAmount,
		&p.LastErrorCode, &p.LastErrorMessage,
		&p.CreatedAt, &p.UpdatedAt, &p.Revision,
	)
	if err != nil {
		return nil, err
	}
	if idempotencyKey.Valid {
		p.IdempotencyKey = idempotencyKey.String
	}
	if productsStr != "" && productsStr != jsonNullLiteral {
		if err := json.Unmarshal([]byte(productsStr), &p.Products); err != nil {
			return nil, apperrors.NewInternal("unmarshal products")
		}
	}
	if p.Products == nil {
		p.Products = []string{}
	}
	return &p, nil
}

// scanCallbackReceipt 扫描 pay_callback_receipts 行。
// SELECT 列顺序：id, organization_id, channel, payload_sha256, payload,
// headers, received_at, client_ip, process_state, event_key, out_trade_no,
// raw_event, error_message, created_at, updated_at, revision（16 列）。
// MySQL nullable 列用 sql.NullString / *string，JSON 列存储为 TEXT 字符串。
func scanCallbackReceipt(sc scannable) (*domain.CallbackReceipt, error) {
	var rec domain.CallbackReceipt
	var payload []byte
	var headersStr sql.NullString
	var clientIP sql.NullString
	var eventKey sql.NullString
	var outTradeNo sql.NullString
	var rawEventStr sql.NullString
	var errorMessage sql.NullString
	err := sc.Scan(
		&rec.ID, &rec.OrganizationID, &rec.Channel, &rec.PayloadSHA256,
		&payload,
		&headersStr, &rec.ReceivedAt, &clientIP,
		&rec.ProcessState, &eventKey, &outTradeNo, &rawEventStr,
		&errorMessage, &rec.CreatedAt, &rec.UpdatedAt, &rec.Revision,
	)
	if err != nil {
		return nil, err
	}
	rec.Payload = payload
	if clientIP.Valid {
		s := clientIP.String
		rec.ClientIP = &s
	}
	if eventKey.Valid {
		s := eventKey.String
		rec.EventKey = &s
	}
	if outTradeNo.Valid {
		s := outTradeNo.String
		rec.OutTradeNo = &s
	}
	if headersStr.Valid && headersStr.String != "" && headersStr.String != jsonNullLiteral {
		_ = json.Unmarshal([]byte(headersStr.String), &rec.Headers)
	}
	if rawEventStr.Valid && rawEventStr.String != "" && rawEventStr.String != jsonNullLiteral {
		_ = json.Unmarshal([]byte(rawEventStr.String), &rec.RawEvent)
	}
	if errorMessage.Valid {
		s := errorMessage.String
		rec.ErrorMessage = &s
	}
	return &rec, nil
}

// scanRefund 扫描 pay_refunds 行。
// SELECT 列顺序：id, organization_id, payment_id, out_refund_no, channel,
// amount, currency, reason, state, channel_refund_no, error_code, error_message,
// requested_at, completed_at, created_at, updated_at, revision（17 列）。
func scanRefund(sc scannable) (*domain.Refund, error) {
	var ref domain.Refund
	var channelRefundNo sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var completedAt sql.NullTime
	err := sc.Scan(
		&ref.ID, &ref.OrganizationID, &ref.PaymentID, &ref.OutRefundNo,
		&ref.Channel, &ref.Amount, &ref.Currency, &ref.Reason,
		&ref.State, &channelRefundNo, &errorCode, &errorMessage,
		&ref.RequestedAt, &completedAt, &ref.CreatedAt, &ref.UpdatedAt, &ref.Revision,
	)
	if err != nil {
		return nil, err
	}
	if channelRefundNo.Valid {
		ref.ChannelRefundNo = channelRefundNo.String
	}
	if errorCode.Valid {
		s := errorCode.String
		ref.ErrorCode = &s
	}
	if errorMessage.Valid {
		s := errorMessage.String
		ref.ErrorMessage = &s
	}
	if completedAt.Valid {
		t := completedAt.Time
		ref.CompletedAt = &t
	}
	return &ref, nil
}

// scanPayAttempt 扫描 pay_attempts 行。
// SELECT 列顺序：id, organization_id, payment_id, refund_id, channel,
// attempt_type, attempt_no, channel_order_no, channel_request_id, state,
// amount, currency, error_code, error_message, latency_ms, occurred_at, raw_response（17 列）。
// MySQL nullable 列用 sql.NullString / sql.NullFloat64 / sql.NullInt64。
func scanPayAttempt(sc scannable) (*domain.PayAttempt, error) {
	var a domain.PayAttempt
	var refundID sql.NullString
	var channelOrderNo sql.NullString
	var channelRequestID sql.NullString
	var amount sql.NullFloat64
	var currency sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var latencyMs sql.NullInt64
	var rawResponseStr sql.NullString
	var stateStr string
	err := sc.Scan(
		&a.ID, &a.OrganizationID, &a.PaymentID, &refundID, &a.Channel,
		&a.AttemptType, &a.AttemptNo, &channelOrderNo, &channelRequestID,
		&stateStr, &amount, &currency, &errorCode, &errorMessage, &latencyMs,
		&a.OccurredAt, &rawResponseStr,
	)
	if err != nil {
		return nil, err
	}
	a.State = domain.AttemptState(stateStr)
	if refundID.Valid {
		s := refundID.String
		a.RefundID = &s
	}
	if channelOrderNo.Valid {
		s := channelOrderNo.String
		a.ChannelOrderNo = &s
	}
	if channelRequestID.Valid {
		s := channelRequestID.String
		a.ChannelRequestID = &s
	}
	if amount.Valid {
		v := amount.Float64
		a.Amount = &v
	}
	if currency.Valid {
		s := currency.String
		a.Currency = &s
	}
	if errorCode.Valid {
		s := errorCode.String
		a.ErrorCode = &s
	}
	if errorMessage.Valid {
		s := errorMessage.String
		a.ErrorMessage = &s
	}
	if latencyMs.Valid {
		v := int(latencyMs.Int64)
		a.LatencyMs = &v
	}
	if rawResponseStr.Valid && rawResponseStr.String != "" && rawResponseStr.String != jsonNullLiteral {
		if err := json.Unmarshal([]byte(rawResponseStr.String), &a.RawResponse); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal raw_response")
		}
	}
	if a.RawResponse == nil {
		a.RawResponse = map[string]any{}
	}
	return &a, nil
}

// scanChannelConfig 扫描 pay_channel_configs 行。
// SELECT 列顺序：id, organization_id, provider_id, channel, environment,
// config_sealed, config_fingerprint, config_set, version_no, status, test_passed_at,
// activated_at, disabled_at, created_at, updated_at, deleted_at, revision（17 列）。
func scanChannelConfig(sc scannable) (*domain.PayChannelConfig, error) {
	var c domain.PayChannelConfig
	var providerID sql.NullString
	var configSetStr sql.NullString
	var testPassedAt sql.NullTime
	var activatedAt sql.NullTime
	var disabledAt sql.NullTime
	var deletedAt sql.NullTime
	err := sc.Scan(
		&c.ID, &c.OrganizationID, &providerID, &c.Channel, &c.Environment,
		&c.ConfigSealed, &c.ConfigFingerprint, &configSetStr,
		&c.VersionNo, &c.Status,
		&testPassedAt, &activatedAt, &disabledAt,
		&c.CreatedAt, &c.UpdatedAt, &deletedAt,
		&c.Revision,
	)
	if err != nil {
		return nil, err
	}
	c.ProviderID = providerID.String
	if testPassedAt.Valid {
		t := testPassedAt.Time
		c.TestPassedAt = &t
	}
	if activatedAt.Valid {
		t := activatedAt.Time
		c.ActivatedAt = &t
	}
	if disabledAt.Valid {
		t := disabledAt.Time
		c.DisabledAt = &t
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		c.DeletedAt = &t
	}
	if configSetStr.Valid && configSetStr.String != "" && configSetStr.String != jsonNullLiteral && configSetStr.String != "[]" {
		if err := json.Unmarshal([]byte(configSetStr.String), &c.ConfigSet); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal config_set")
		}
	}
	if c.ConfigSet == nil {
		c.ConfigSet = []string{}
	}
	return &c, nil
}

// scanReconBatch 扫描 pay_recon_batches 行。
// SELECT 列顺序：id, organization_id, channel, batch_date, status,
// diff_count, total_local, total_channel, created_at, updated_at, deleted_at, revision（12 列）。
func scanReconBatch(sc scannable) (*domain.ReconBatch, error) {
	var b domain.ReconBatch
	var deletedAt sql.NullTime
	err := sc.Scan(
		&b.ID, &b.OrganizationID, &b.Channel, &b.BatchDate, &b.Status,
		&b.DiffCount, &b.TotalLocal, &b.TotalChannel,
		&b.CreatedAt, &b.UpdatedAt, &deletedAt, &b.Revision,
	)
	if err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		b.DeletedAt = &t
	}
	return &b, nil
}

// scanReconItem 扫描 pay_recon_items 行。
// SELECT 列顺序：id, batch_id, diff_type, channel_order_no, local_payment_id,
// channel_amount, local_amount, channel_state, local_state, resolved,
// resolved_by, resolved_at, note, created_at（14 列）。
func scanReconItem(sc scannable) (*domain.ReconItem, error) {
	var i domain.ReconItem
	var channelOrderNo sql.NullString
	var localPaymentID sql.NullString
	var channelAmount sql.NullFloat64
	var localAmount sql.NullFloat64
	var channelState sql.NullString
	var localState sql.NullString
	var approvedBy sql.NullString
	var approvedAt sql.NullTime
	var resolvedBy sql.NullString
	var resolvedAt sql.NullTime
	var note sql.NullString
	err := sc.Scan(
		&i.ID, &i.BatchID, &i.DiffType, &channelOrderNo, &localPaymentID,
		&channelAmount, &localAmount, &channelState, &localState,
		&i.Resolved, &i.ResolutionState, &approvedBy, &approvedAt, &resolvedBy, &resolvedAt, &note, &i.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if channelOrderNo.Valid {
		s := channelOrderNo.String
		i.ChannelOrderNo = &s
	}
	if localPaymentID.Valid {
		s := localPaymentID.String
		i.LocalPaymentID = &s
	}
	if channelAmount.Valid {
		v := channelAmount.Float64
		i.ChannelAmount = &v
	}
	if localAmount.Valid {
		v := localAmount.Float64
		i.LocalAmount = &v
	}
	if channelState.Valid {
		s := channelState.String
		i.ChannelState = &s
	}
	if localState.Valid {
		s := localState.String
		i.LocalState = &s
	}
	if resolvedBy.Valid {
		s := resolvedBy.String
		i.ResolvedBy = &s
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		i.ResolvedAt = &t
	}
	if note.Valid {
		s := note.String
		i.Note = &s
	}
	return &i, nil
}

package payments

// ─── 表名常量 ────────────────────────────────────────────────────────────
// PG → MySQL: 去掉 schema 限定符 guliangiam.，反引号包裹

const (
	btPayCallbackReceipts = "`pay_callback_receipts`"
	btPayRefunds          = "`pay_refunds`"
	btPayAttempts         = "`pay_attempts`"
	btPayChannelConfigs   = "`pay_channel_configs`"
	btPayReconBatches     = "`pay_recon_batches`"
	btPayReconItems       = "`pay_recon_items`"
)

// ─── Payments 表增量查询 ────────────────────────────────────────────────

// queryGetByChannelOrderNo PG $1→provider, $2→channel_order_no
// MySQL 直接 ?，SELECT 列包含 6 个增量列：channel_order_no, attempt_count, refunded_amount, refunding_amount, last_error_code, last_error_message
const queryGetByChannelOrderNo = `SELECT id, organization_id, name, display_name, provider, type, products, product_name, product_display_name, detail, currency, price, user_id, person_name, person_id_card, person_email, person_phone, invoice_type, invoice_title, invoice_tax_id, invoice_remark, invoice_url, order_id, out_order_id, pay_url, success_url, status, message, idempotency_key, channel_order_no, attempt_count, refunded_amount, refunding_amount, last_error_code, last_error_message, created_at, updated_at, revision FROM ` + btPayments + ` WHERE provider = ? AND channel_order_no = ?`

const queryGetByOutTradeNo = `SELECT id, organization_id, name, display_name, provider, type, products, product_name, product_display_name, detail, currency, price, user_id, person_name, person_id_card, person_email, person_phone, invoice_type, invoice_title, invoice_tax_id, invoice_remark, invoice_url, order_id, out_order_id, pay_url, success_url, status, message, idempotency_key, channel_order_no, attempt_count, refunded_amount, refunding_amount, last_error_code, last_error_message, created_at, updated_at, revision FROM ` + btPayments + ` WHERE provider = ? AND out_order_id = ?`

// ─── pay_callback_receipts ──────────────────────────────────────────────
// DDL 真实列：id, organization_id, channel, payload_sha256, payload, headers,
// received_at, client_ip, process_state, event_key, out_trade_no, raw_event,
// error_message, created_at, updated_at, revision（共 16 列）。

const (
	receiptInsert             = `INSERT INTO ` + btPayCallbackReceipts + ` (id, organization_id, channel, payload_sha256, payload, headers, received_at, client_ip, process_state, event_key, out_trade_no, raw_event, error_message, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	receiptGetByPayloadSHA256 = `SELECT id, organization_id, channel, payload_sha256, payload, headers, received_at, client_ip, process_state, event_key, out_trade_no, raw_event, error_message, created_at, updated_at, revision FROM ` + btPayCallbackReceipts + ` WHERE channel = ? AND payload_sha256 = ?`
	receiptGetByEventKey      = `SELECT id, organization_id, channel, payload_sha256, payload, headers, received_at, client_ip, process_state, event_key, out_trade_no, raw_event, error_message, created_at, updated_at, revision FROM ` + btPayCallbackReceipts + ` WHERE channel = ? AND event_key = ?`
	receiptUpdate             = `UPDATE ` + btPayCallbackReceipts + ` SET process_state=?, event_key=?, out_trade_no=?, raw_event=?, error_message=?, updated_at=?, revision=revision+1 WHERE id=?`
	receiptList               = `SELECT id, organization_id, channel, payload_sha256, payload, headers, received_at, client_ip, process_state, event_key, out_trade_no, raw_event, error_message, created_at, updated_at, revision FROM ` + btPayCallbackReceipts + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) ORDER BY received_at DESC LIMIT ? OFFSET ?`
	receiptCount              = `SELECT COUNT(*) FROM ` + btPayCallbackReceipts + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)`
)

// ─── pay_refunds ────────────────────────────────────────────────────────

const (
	refundInsert                       = `INSERT INTO ` + btPayRefunds + ` (id, organization_id, payment_id, out_refund_no, channel, amount, currency, reason, state, channel_refund_no, error_code, error_message, requested_at, completed_at, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	refundGetByID                      = `SELECT id, organization_id, payment_id, out_refund_no, channel, amount, currency, reason, state, channel_refund_no, error_code, error_message, requested_at, completed_at, created_at, updated_at, revision FROM ` + btPayRefunds + ` WHERE id = ?`
	refundGetByIDForOrganization       = `SELECT id, organization_id, payment_id, out_refund_no, channel, amount, currency, reason, state, channel_refund_no, error_code, error_message, requested_at, completed_at, created_at, updated_at, revision FROM ` + btPayRefunds + ` WHERE id = ? AND organization_id = ?`
	refundGetByOutRefundNo             = `SELECT id, organization_id, payment_id, out_refund_no, channel, amount, currency, reason, state, channel_refund_no, error_code, error_message, requested_at, completed_at, created_at, updated_at, revision FROM ` + btPayRefunds + ` WHERE channel = ? AND out_refund_no = ?`
	refundUpdate                       = `UPDATE ` + btPayRefunds + ` SET channel_refund_no=?, state=?, error_code=?, error_message=?, completed_at=?, updated_at=?, revision=revision+1 WHERE id=?`
	refundListByPayment                = `SELECT id, organization_id, payment_id, out_refund_no, channel, amount, currency, reason, state, channel_refund_no, error_code, error_message, requested_at, completed_at, created_at, updated_at, revision FROM ` + btPayRefunds + ` WHERE payment_id = ? ORDER BY requested_at DESC`
	refundListByPaymentForOrganization = `SELECT id, organization_id, payment_id, out_refund_no, channel, amount, currency, reason, state, channel_refund_no, error_code, error_message, requested_at, completed_at, created_at, updated_at, revision FROM ` + btPayRefunds + ` WHERE payment_id = ? AND organization_id = ? ORDER BY requested_at DESC`
	refundList                         = `SELECT id, organization_id, payment_id, out_refund_no, channel, amount, currency, reason, state, channel_refund_no, error_code, error_message, requested_at, completed_at, created_at, updated_at, revision FROM ` + btPayRefunds + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) ORDER BY requested_at DESC LIMIT ? OFFSET ?`
	refundCount                        = `SELECT COUNT(*) FROM ` + btPayRefunds + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)`
	refundLockPayment                  = `SELECT COALESCE((SELECT SUM(r.amount) FROM ` + btPayRefunds + ` r WHERE r.payment_id = p.id AND r.state = 'succeeded'), 0), COALESCE((SELECT SUM(r.amount) FROM ` + btPayRefunds + ` r WHERE r.payment_id = p.id AND r.state IN ('requested', 'processing')), 0), p.price, p.status, p.provider, p.out_order_id, p.currency FROM ` + btPayments + ` p WHERE p.id = ? AND p.status IN ('succeeded', 'partially_refunded') FOR UPDATE`
)

// ─── pay_attempts ──────────────────────────────────────────────────────

const (
	attemptGetByID                      = `SELECT id, organization_id, payment_id, refund_id, channel, attempt_type, attempt_no, channel_order_no, channel_request_id, state, amount, currency, error_code, error_message, latency_ms, occurred_at, raw_response FROM ` + btPayAttempts + ` WHERE id = ?`
	attemptGetByIDForOrganization       = `SELECT id, organization_id, payment_id, refund_id, channel, attempt_type, attempt_no, channel_order_no, channel_request_id, state, amount, currency, error_code, error_message, latency_ms, occurred_at, raw_response FROM ` + btPayAttempts + ` WHERE id = ? AND organization_id = ?`
	attemptListByPayment                = `SELECT id, organization_id, payment_id, refund_id, channel, attempt_type, attempt_no, channel_order_no, channel_request_id, state, amount, currency, error_code, error_message, latency_ms, occurred_at, raw_response FROM ` + btPayAttempts + ` WHERE payment_id = ? ORDER BY occurred_at DESC`
	attemptListByPaymentForOrganization = `SELECT id, organization_id, payment_id, refund_id, channel, attempt_type, attempt_no, channel_order_no, channel_request_id, state, amount, currency, error_code, error_message, latency_ms, occurred_at, raw_response FROM ` + btPayAttempts + ` WHERE payment_id = ? AND organization_id = ? ORDER BY occurred_at DESC`
	attemptListByRefund                 = `SELECT id, organization_id, payment_id, refund_id, channel, attempt_type, attempt_no, channel_order_no, channel_request_id, state, amount, currency, error_code, error_message, latency_ms, occurred_at, raw_response FROM ` + btPayAttempts + ` WHERE refund_id = ? ORDER BY occurred_at DESC`
	attemptInsert                       = `INSERT INTO ` + btPayAttempts + ` (id, organization_id, payment_id, refund_id, channel, attempt_type, attempt_no, channel_order_no, channel_request_id, state, amount, currency, error_code, error_message, latency_ms, occurred_at, raw_response) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

// ─── pay_channel_configs ────────────────────────────────────────────────

const (
	queryChannelConfigGetByID                = `SELECT id, organization_id, provider_id, channel, environment, config_sealed, config_fingerprint, config_set, version_no, status, test_passed_at, activated_at, disabled_at, created_at, updated_at, deleted_at, revision FROM ` + btPayChannelConfigs + ` WHERE id = ?`
	queryChannelConfigGetByIDForOrganization = `SELECT id, organization_id, provider_id, channel, environment, config_sealed, config_fingerprint, config_set, version_no, status, test_passed_at, activated_at, disabled_at, created_at, updated_at, deleted_at, revision FROM ` + btPayChannelConfigs + ` WHERE id = ? AND organization_id = ?`
	queryChannelConfigGetActive              = `SELECT id, organization_id, provider_id, channel, environment, config_sealed, config_fingerprint, config_set, version_no, status, test_passed_at, activated_at, disabled_at, created_at, updated_at, deleted_at, revision FROM ` + btPayChannelConfigs + ` WHERE organization_id = ? AND channel = ? AND environment = ? AND status = 'active' AND deleted_at IS NULL ORDER BY version_no DESC LIMIT 1`
	queryChannelConfigList                   = `SELECT id, organization_id, provider_id, channel, environment, config_sealed, config_fingerprint, config_set, version_no, status, test_passed_at, activated_at, disabled_at, created_at, updated_at, deleted_at, revision FROM ` + btPayChannelConfigs + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND deleted_at IS NULL ORDER BY channel ASC, version_no DESC`
	queryChannelConfigInsert                 = `INSERT INTO ` + btPayChannelConfigs + ` (id, organization_id, provider_id, channel, environment, config_sealed, config_fingerprint, config_set, version_no, status, test_passed_at, activated_at, disabled_at, created_at, updated_at, deleted_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	queryChannelConfigUpdate                 = `UPDATE ` + btPayChannelConfigs + ` SET status=?, test_passed_at=?, activated_at=?, disabled_at=?, updated_at=?, revision=? WHERE id=? AND revision=? AND organization_id=?`
)

// ─── pay_recon_batches + pay_recon_items ─────────────────────────────────

const (
	reconBatchGetByID                = `SELECT id, organization_id, channel, batch_date, status, diff_count, total_local, total_channel, created_at, updated_at, deleted_at, revision FROM ` + btPayReconBatches + ` WHERE id = ?`
	reconBatchGetByIDForOrganization = `SELECT id, organization_id, channel, batch_date, status, diff_count, total_local, total_channel, created_at, updated_at, deleted_at, revision FROM ` + btPayReconBatches + ` WHERE id = ? AND organization_id = ?`
	reconBatchList                   = `SELECT id, organization_id, channel, batch_date, status, diff_count, total_local, total_channel, created_at, updated_at, deleted_at, revision FROM ` + btPayReconBatches + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND deleted_at IS NULL ORDER BY batch_date DESC, channel ASC LIMIT ? OFFSET ?`
	reconBatchCount                  = `SELECT COUNT(*) FROM ` + btPayReconBatches + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND deleted_at IS NULL`
	reconBatchInsert                 = `INSERT INTO ` + btPayReconBatches + ` (id, organization_id, channel, batch_date, status, diff_count, total_local, total_channel, created_at, updated_at, deleted_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	reconBatchUpdate                 = `UPDATE ` + btPayReconBatches + ` SET status=?, diff_count=?, total_local=?, total_channel=?, updated_at=?, revision=? WHERE id=? AND revision=?`

	reconItemList                   = `SELECT id, batch_id, diff_type, channel_order_no, local_payment_id, channel_amount, local_amount, channel_state, local_state, resolved, resolved_by, resolved_at, note, created_at FROM ` + btPayReconItems + ` WHERE batch_id = ? ORDER BY diff_type, id`
	reconItemListForOrganization    = `SELECT i.id, i.batch_id, i.diff_type, i.channel_order_no, i.local_payment_id, i.channel_amount, i.local_amount, i.channel_state, i.local_state, i.resolved, i.resolution_state, i.approved_by, i.approved_at, i.resolved_by, i.resolved_at, i.note, i.created_at FROM ` + btPayReconItems + ` i WHERE i.batch_id = ? AND EXISTS (SELECT 1 FROM ` + btPayReconBatches + ` b WHERE b.id = i.batch_id AND b.organization_id = ?) ORDER BY i.diff_type, i.id`
	reconItemInsert                 = `INSERT INTO ` + btPayReconItems + ` (id, batch_id, diff_type, channel_order_no, local_payment_id, channel_amount, local_amount, channel_state, local_state, resolved, resolved_by, resolved_at, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	reconItemResolve                = `UPDATE ` + btPayReconItems + ` SET resolved=TRUE, resolution_state='resolved', resolved_by=?, resolved_at=?, note=? WHERE id=? AND resolved=FALSE`
	reconItemResolveForOrganization = `UPDATE ` + btPayReconItems + ` i SET resolved=TRUE, resolution_state='resolved', resolved_by=?, resolved_at=?, note=? WHERE i.id=? AND i.resolved=FALSE AND EXISTS (SELECT 1 FROM ` + btPayReconBatches + ` b WHERE b.id = i.batch_id AND b.organization_id = ?)`
	reconItemResolvedBy             = `SELECT resolved_by FROM ` + btPayReconItems + ` WHERE id = ?`
	reconItemApproveForOrganization = `UPDATE ` + btPayReconItems + ` i SET approved_by=?, approved_at=?, resolution_state='approved' WHERE i.id=? AND i.resolved=TRUE AND i.approved_by IS NULL AND i.resolved_by <> ? AND EXISTS (SELECT 1 FROM ` + btPayReconBatches + ` b WHERE b.id = i.batch_id AND b.organization_id = ?)`
)

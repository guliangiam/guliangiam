package payments

// ─── Payments 表增量查询 ────────────────────────────────────────────────
// gen_schema.py diff 会生成 ALTER TABLE，假设 6 个增量列已存在：
// channel_order_no / attempt_count / refunded_amount / refunding_amount /
// last_error_code / last_error_message

const (
	QueryGetByChannelOrderNo = `
		SELECT id, organization_id, name, display_name, provider, type,
		       products, product_name, product_display_name, detail,
		       currency, price, user_id, person_name, person_id_card,
		       person_email, person_phone, invoice_type, invoice_title,
		       invoice_tax_id, invoice_remark, invoice_url, order_id,
		       out_order_id, pay_url, success_url, status, message,
		       idempotency_key, channel_order_no, attempt_count,
		       refunded_amount, refunding_amount, last_error_code,
		       last_error_message, created_at, updated_at, revision
		  FROM "guliangiam".payments
		 WHERE provider = $1 AND channel_order_no = $2
	`
	QueryGetByOutTradeNo = `
		SELECT id, organization_id, name, display_name, provider, type,
		       products, product_name, product_display_name, detail,
		       currency, price, user_id, person_name, person_id_card,
		       person_email, person_phone, invoice_type, invoice_title,
		       invoice_tax_id, invoice_remark, invoice_url, order_id,
		       out_order_id, pay_url, success_url, status, message,
		       idempotency_key, channel_order_no, attempt_count,
		       refunded_amount, refunding_amount, last_error_code,
		       last_error_message, created_at, updated_at, revision
		  FROM "guliangiam".payments
		 WHERE provider = $1 AND out_order_id = $2
	`
)

// ─── pay_callback_receipts ──────────────────────────────────────────────
// 与 domain.CallbackReceipt 真实字段对齐：
//   ID / OrganizationID / Channel / PayloadSHA256 / Payload /
//   Headers / ReceivedAt / ClientIP / ProcessState / EventKey /
//   OutTradeNo / RawEvent / ErrorMessage / CreatedAt / UpdatedAt / Revision

const (
	ReceiptInsert = `
		INSERT INTO "guliangiam".pay_callback_receipts
			(id, organization_id, channel, payload_sha256, payload,
			 headers, received_at, client_ip, process_state,
			 event_key, out_trade_no, raw_event, error_message,
			 created_at, updated_at, revision)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::bytea,
		        $6::jsonb, $7, $8, $9,
		        $10, $11, $12::jsonb, $13,
		        $14, $15, $16)
	`
	ReceiptGetByPayloadSHA256 = `
		SELECT id, organization_id, channel, payload_sha256, payload,
		       headers, received_at, client_ip, process_state,
		       event_key, out_trade_no, raw_event, error_message,
		       created_at, updated_at, revision
		  FROM "guliangiam".pay_callback_receipts
		 WHERE channel = $1 AND payload_sha256 = $2
	`
	ReceiptGetByEventKey = `
		SELECT id, organization_id, channel, payload_sha256, payload,
		       headers, received_at, client_ip, process_state,
		       event_key, out_trade_no, raw_event, error_message,
		       created_at, updated_at, revision
		  FROM "guliangiam".pay_callback_receipts
		 WHERE channel = $1 AND event_key = $2
	`
	ReceiptUpdate = `
		UPDATE "guliangiam".pay_callback_receipts
		   SET process_state=$1, event_key=$2, out_trade_no=$3,
		       raw_event=$4::jsonb, error_message=$5,
		       updated_at=$6, revision=revision+1
		 WHERE id=$7
	`
	ReceiptList = `
		SELECT id, organization_id, channel, payload_sha256, payload,
		       headers, received_at, client_ip, process_state,
		       event_key, out_trade_no, raw_event, error_message,
		       created_at, updated_at, revision
		  FROM "guliangiam".pay_callback_receipts
		 WHERE organization_id = guliangiam.resolve_org_id($1)
		 ORDER BY received_at DESC LIMIT $2 OFFSET $3
	`
	ReceiptCount = `
		SELECT COUNT(*)
		  FROM "guliangiam".pay_callback_receipts
		 WHERE organization_id = guliangiam.resolve_org_id($1)
	`
)

// ─── pay_refunds ────────────────────────────────────────────────────────
// Domain Refund 真实字段（无 AttemptID — Schema 列用 NULL）：
//   ID / OrganizationID / PaymentID / OutRefundNo / Channel / Amount /
//   Currency / State / Reason / ChannelRefundNo / ErrorCode / ErrorMessage /
//   RequestedAt / CompletedAt / CreatedAt / UpdatedAt / DeletedAt / Revision

const (
	RefundInsert = `
		INSERT INTO "guliangiam".pay_refunds
			(id, organization_id, payment_id, out_refund_no,
			 channel, amount, currency, reason, state,
			 channel_refund_no, error_code, error_message,
			 requested_at, completed_at, created_at, updated_at, revision)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4,
		        $5, $6, $7, $8, $9,
		        $10, $11, $12,
		        $13, $14, $15, $16, $17)
	`
	RefundGetByID = `
		SELECT id, organization_id, payment_id, out_refund_no,
		       channel, amount, currency, reason, state,
		       channel_refund_no, error_code, error_message,
		       requested_at, completed_at, created_at, updated_at, revision
		  FROM "guliangiam".pay_refunds
		 WHERE id = $1::uuid
	`
	RefundGetByIDForOrganization = `
		SELECT id, organization_id, payment_id, out_refund_no,
		       channel, amount, currency, reason, state,
		       channel_refund_no, error_code, error_message,
		       requested_at, completed_at, created_at, updated_at, revision
		  FROM "guliangiam".pay_refunds
		 WHERE id = $1::uuid
		   AND organization_id = guliangiam.resolve_org_id($2)
	`
	RefundGetByOutRefundNo = `
		SELECT id, organization_id, payment_id, out_refund_no,
		       channel, amount, currency, reason, state,
		       channel_refund_no, error_code, error_message,
		       requested_at, completed_at, created_at, updated_at, revision
		  FROM "guliangiam".pay_refunds
		 WHERE channel = $1 AND out_refund_no = $2
	`
	RefundUpdate = `
		UPDATE "guliangiam".pay_refunds
		   SET channel_refund_no=$1, state=$2,
		       error_code=$3, error_message=$4, completed_at=$5,
		       updated_at=$6, revision=revision+1
		 WHERE id=$7
	`
	RefundListByPayment = `
		SELECT id, organization_id, payment_id, out_refund_no,
		       channel, amount, currency, reason, state,
		       channel_refund_no, error_code, error_message,
		       requested_at, completed_at, created_at, updated_at, revision
		  FROM "guliangiam".pay_refunds
		 WHERE payment_id = $1::uuid
		 ORDER BY requested_at DESC
	`
	RefundListByPaymentForOrganization = `
		SELECT id, organization_id, payment_id, out_refund_no,
		       channel, amount, currency, reason, state,
		       channel_refund_no, error_code, error_message,
		       requested_at, completed_at, created_at, updated_at, revision
		  FROM "guliangiam".pay_refunds
		 WHERE payment_id = $1::uuid
		   AND organization_id = guliangiam.resolve_org_id($2)
		 ORDER BY requested_at DESC
	`
	RefundLockPayment = `
		SELECT
		    COALESCE((SELECT SUM(r.amount) FROM "guliangiam".pay_refunds r
		               WHERE r.payment_id = p.id AND r.state = 'succeeded'), 0) AS refunded_amount,
		    COALESCE((SELECT SUM(r.amount) FROM "guliangiam".pay_refunds r
		               WHERE r.payment_id = p.id AND r.state IN ('requested', 'processing')), 0) AS refunding_amount,
		    p.price, p.status, p.provider, p.out_order_id, p.currency
		  FROM "guliangiam".payments p
		 WHERE p.id = $1::uuid
		   AND p.status IN ('succeeded', 'partially_refunded')
		 FOR UPDATE
	`
)

// ─── pay_attempts (AttemptsRepo) ──────────────────────────────────────
// Domain PayAttempt 字段:
//   ID / OrganizationID / PaymentID / RefundID* / Channel / AttemptType /
//   AttemptNo / ChannelOrderNo* / ChannelRequestID* / State / Amount* /
//   Currency* / ErrorCode* / ErrorMessage* / LatencyMs* / OccurredAt /
//   RawResponse(map)

const (
	AttemptGetByID = `
		SELECT id, organization_id, payment_id, refund_id, channel,
		       attempt_type, attempt_no, channel_order_no, channel_request_id,
		       state, amount, currency, error_code, error_message, latency_ms,
		       occurred_at, raw_response
		  FROM "guliangiam".pay_attempts
		 WHERE id = $1::uuid
	`
	AttemptGetByIDForOrganization = `
		SELECT id, organization_id, payment_id, refund_id, channel,
		       attempt_type, attempt_no, channel_order_no, channel_request_id,
		       state, amount, currency, error_code, error_message, latency_ms,
		       occurred_at, raw_response
		  FROM "guliangiam".pay_attempts
		 WHERE id = $1::uuid
		   AND organization_id = guliangiam.resolve_org_id($2)
	`
	AttemptListByPayment = `
		SELECT id, organization_id, payment_id, refund_id, channel,
		       attempt_type, attempt_no, channel_order_no, channel_request_id,
		       state, amount, currency, error_code, error_message, latency_ms,
		       occurred_at, raw_response
		  FROM "guliangiam".pay_attempts
		 WHERE payment_id = $1::uuid
		 ORDER BY occurred_at DESC
	`
	AttemptListByPaymentForOrganization = `
		SELECT id, organization_id, payment_id, refund_id, channel,
		       attempt_type, attempt_no, channel_order_no, channel_request_id,
		       state, amount, currency, error_code, error_message, latency_ms,
		       occurred_at, raw_response
		  FROM "guliangiam".pay_attempts
		 WHERE payment_id = $1::uuid
		   AND organization_id = guliangiam.resolve_org_id($2)
		 ORDER BY occurred_at DESC
	`
	AttemptListByRefund = `
		SELECT id, organization_id, payment_id, refund_id, channel,
		       attempt_type, attempt_no, channel_order_no, channel_request_id,
		       state, amount, currency, error_code, error_message, latency_ms,
		       occurred_at, raw_response
		  FROM "guliangiam".pay_attempts
		 WHERE refund_id = $1::uuid
		 ORDER BY occurred_at DESC
	`
	AttemptInsert = `
		INSERT INTO "guliangiam".pay_attempts
			(id, organization_id, payment_id, refund_id, channel,
			 attempt_type, attempt_no, channel_order_no, channel_request_id,
			 state, amount, currency, error_code, error_message, latency_ms,
			 occurred_at, raw_response)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5,
		        $6, $7, $8, $9,
		        $10, $11, $12, $13, $14, $15,
		        $16, $17::jsonb)
	`
)

// ─── pay_recon_batches + pay_recon_items (ReconRepo) ────────────────────

const (
	ReconBatchColumns = `id, organization_id, channel, batch_date, status,
       diff_count, total_local, total_channel,
       created_at, updated_at, deleted_at, revision`

	ReconBatchGetByID = `SELECT ` + ReconBatchColumns + ` FROM "guliangiam".pay_recon_batches WHERE id = $1::uuid`

	ReconBatchGetByIDForOrganization = `SELECT ` + ReconBatchColumns + ` FROM "guliangiam".pay_recon_batches WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	ReconBatchList = `
		SELECT ` + ReconBatchColumns + `
		  FROM "guliangiam".pay_recon_batches
		 WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
		   AND deleted_at IS NULL
		 ORDER BY batch_date DESC, channel ASC
		 LIMIT $2 OFFSET $3
	`
	ReconBatchCount = `
		SELECT COUNT(*)
		  FROM "guliangiam".pay_recon_batches
		 WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
		   AND deleted_at IS NULL
	`
	ReconBatchInsert = `
		INSERT INTO "guliangiam".pay_recon_batches
			(id, organization_id, channel, batch_date, status,
			 diff_count, total_local, total_channel,
			 created_at, updated_at, deleted_at, revision)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5,
		        $6, $7, $8,
		        $9, $10, $11, $12)
	`
	ReconBatchUpdate = `
		UPDATE "guliangiam".pay_recon_batches
		   SET status=$1, diff_count=$2, total_local=$3, total_channel=$4,
		       updated_at=$5, revision=$6
		 WHERE id=$7::uuid AND revision=$8
	`

	ReconItemColumns = `id, batch_id, diff_type, channel_order_no, local_payment_id,
       channel_amount, local_amount, channel_state, local_state,
       resolved, resolution_state, approved_by, approved_at,
       resolved_by, resolved_at, note, created_at`

	ReconItemList = `
		SELECT ` + ReconItemColumns + `
		  FROM "guliangiam".pay_recon_items
		 WHERE batch_id = $1::uuid
		 ORDER BY diff_type, id
	`
	ReconItemListForOrganization = `
		SELECT ` + ReconItemColumns + `
		  FROM "guliangiam".pay_recon_items i
		 WHERE i.batch_id = $1::uuid
		   AND EXISTS (
		       SELECT 1 FROM "guliangiam".pay_recon_batches b
		        WHERE b.id = i.batch_id
		          AND b.organization_id = guliangiam.resolve_org_id($2)
		   )
		 ORDER BY i.diff_type, i.id
	`
	ReconItemInsert = `
		INSERT INTO "guliangiam".pay_recon_items
			(id, batch_id, diff_type, channel_order_no, local_payment_id,
			 channel_amount, local_amount, channel_state, local_state,
			 resolved, resolved_by, resolved_at, note, created_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid,
		        $6, $7, $8, $9,
		        $10, $11, $12, $13, $14)
	`
	ReconItemResolve = `
		UPDATE "guliangiam".pay_recon_items
		   SET resolved=TRUE, resolution_state='resolved', resolved_by=$1, resolved_at=$2, note=$3
		 WHERE id=$4::uuid AND resolved=FALSE
	`
	ReconItemResolveForOrganization = `
		UPDATE "guliangiam".pay_recon_items
		   SET resolved=TRUE, resolution_state='resolved', resolved_by=$1, resolved_at=$2, note=$3
		 WHERE id=$4::uuid AND resolved=FALSE
		   AND EXISTS (
		       SELECT 1 FROM "guliangiam".pay_recon_batches b
		        WHERE b.id = pay_recon_items.batch_id
		          AND b.organization_id = guliangiam.resolve_org_id($5)
		   )
	`
	ReconItemApproveForOrganization = `
		UPDATE "guliangiam".pay_recon_items
		   SET approved_by=$1::uuid, approved_at=$2, resolution_state='approved'
		 WHERE id=$3::uuid AND resolved=TRUE AND approved_by IS NULL
		   AND resolved_by <> ($1::uuid)::text
		   AND EXISTS (
		       SELECT 1 FROM "guliangiam".pay_recon_batches b
		        WHERE b.id = pay_recon_items.batch_id
		          AND b.organization_id = guliangiam.resolve_org_id($4)
		   )
		RETURNING resolved_by
	`
)

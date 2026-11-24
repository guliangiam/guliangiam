package payments

// QueryPaymentColumns is the single full-row projection shared by every payment
// read. The six settlement columns (channel_order_no / attempt_count /
// refunded_amount / refunding_amount / last_error_code / last_error_message)
// are included so a read-modify-write cycle never zeroes them (BE-C-08/10).
const QueryPaymentColumns = `id, organization_id, name, display_name, provider, type, products, product_name, product_display_name, detail, currency, price, user_id, person_name, person_id_card, person_email, person_phone, invoice_type, invoice_title, invoice_tax_id, invoice_remark, invoice_url, order_id, out_order_id, pay_url, success_url, status, message, idempotency_key, channel_order_no, attempt_count, refunded_amount, refunding_amount, last_error_code, last_error_message, created_at, updated_at, revision`

const (
	QueryGetByID                = `SELECT ` + QueryPaymentColumns + ` FROM "guliangiam".payments WHERE id = $1::uuid`
	QueryGetByIDForOrganization = `SELECT ` + QueryPaymentColumns + ` FROM "guliangiam".payments WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
	QueryList                   = `SELECT ` + QueryPaymentColumns + ` FROM "guliangiam".payments WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND (CASE WHEN $3 = '' THEN TRUE ELSE user_id = $3::uuid END) AND ($4::text IS NULL OR status = $4) ORDER BY created_at DESC LIMIT $5 OFFSET $6`
	QueryCount                  = `SELECT COUNT(*) FROM "guliangiam".payments WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%') AND (CASE WHEN $3 = '' THEN TRUE ELSE user_id = $3::uuid END) AND ($4::text IS NULL OR status = $4)`
	QueryInsert                 = `INSERT INTO "guliangiam".payments (id, organization_id, name, display_name, provider, type, products, product_name, product_display_name, detail, currency, price, user_id, person_name, person_id_card, person_email, person_phone, invoice_type, invoice_title, invoice_tax_id, invoice_remark, invoice_url, order_id, out_order_id, pay_url, success_url, status, message, idempotency_key, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13::uuid, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23::uuid, $24, $25, $26, $27, $28, $29, $30, $31, $32)`
	// QueryUpdate persists the settlement columns the charge/callback paths
	// mutate (BE-C-08/10). refunded_amount/refunding_amount stay derived from
	// pay_refunds (RefundLockPayment) and are deliberately not written here.
	QueryUpdate = `UPDATE "guliangiam".payments SET display_name=$1, provider=$2, type=$3, status=$4, message=$5, pay_url=$6, success_url=$7, out_order_id=$8, invoice_url=$9, idempotency_key=$10, updated_at=$11, revision=$12, channel_order_no=$15, attempt_count=$16, last_error_code=$17, last_error_message=$18 WHERE id=$13 AND organization_id = guliangiam.resolve_org_id($14)`
	QueryDelete = `DELETE FROM "guliangiam".payments WHERE id=$1 AND organization_id = guliangiam.resolve_org_id($2)`
)

// ─── Channel Configs (G6) ─────────────────────────────────────────────
// SELECT 列顺序必须与 scanPayChannelConfig 参数顺序严格一致。

const (
	QueryChannelConfigColumns = `id, organization_id, provider_id, channel, environment, config_sealed, config_fingerprint, config_set, version_no, status, test_passed_at, activated_at, disabled_at, created_at, updated_at, deleted_at, revision`

	QueryChannelConfigGetByID                = `SELECT ` + QueryChannelConfigColumns + ` FROM "guliangiam".pay_channel_configs WHERE id = $1::uuid AND deleted_at IS NULL`
	QueryChannelConfigGetByIDForOrganization = `SELECT ` + QueryChannelConfigColumns + ` FROM "guliangiam".pay_channel_configs WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`

	QueryChannelConfigGetActive = `SELECT ` + QueryChannelConfigColumns + ` FROM "guliangiam".pay_channel_configs WHERE organization_id = guliangiam.resolve_org_id($1) AND channel = $2 AND environment = $3 AND status = 'active' AND deleted_at IS NULL ORDER BY version_no DESC LIMIT 1`

	QueryChannelConfigList = `SELECT ` + QueryChannelConfigColumns + ` FROM "guliangiam".pay_channel_configs WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END) AND deleted_at IS NULL ORDER BY channel, version_no DESC`

	QueryChannelConfigInsert = `INSERT INTO "guliangiam".pay_channel_configs (id, organization_id, provider_id, channel, environment, config_sealed, config_fingerprint, config_set, version_no, status, test_passed_at, activated_at, disabled_at, created_at, updated_at, deleted_at, revision) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16, $17)`

	QueryChannelConfigUpdate = `UPDATE "guliangiam".pay_channel_configs SET status=$1, test_passed_at=$2, activated_at=$3, disabled_at=$4, updated_at=$5, revision=$6 WHERE id=$7::uuid AND revision=$8 AND organization_id = guliangiam.resolve_org_id($9)`
)

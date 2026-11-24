package privacy

const (
	queryInsertDsarRequest = `
INSERT INTO guliangiam.dsar_requests
    (id, organization_id, subject_type, subject_id, request_type, state, sla_due_at, closed_at, cooling_until, rejection_reason, created_at, updated_at, created_by, revision)
VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5, $6, $7, $8, $9, $10, $11, $12, $13::uuid, $14)`

	queryInsertDsarEvent = `
INSERT INTO guliangiam.dsar_events (id, organization_id, dsar_id, action, actor_id, payload, created_at)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6, $7)`

	queryGetDsarRequestForOrganization = `
SELECT id, organization_id, subject_type, subject_id, request_type, state, sla_due_at, closed_at, cooling_until, rejection_reason, created_by, created_at, updated_at, revision
FROM guliangiam.dsar_requests
WHERE id = $1::uuid AND organization_id = $2::uuid`

	queryUpdateDsarRequest = `
UPDATE guliangiam.dsar_requests
SET state = $1,
    sla_due_at = $2,
    closed_at = $3,
    cooling_until = $4,
    rejection_reason = $5,
    updated_at = $6,
    revision = revision + 1
WHERE id = $7::uuid AND organization_id = $8::uuid AND revision = $9`

	queryCountDsarRequestsForOrganization = `
SELECT COUNT(*)
FROM guliangiam.dsar_requests
WHERE organization_id = $1::uuid`

	queryListDsarRequestsForOrganization = `
SELECT id, organization_id, subject_type, subject_id, request_type, state, sla_due_at, closed_at, cooling_until, rejection_reason, created_by, created_at, updated_at, revision
FROM guliangiam.dsar_requests
WHERE organization_id = $1::uuid
ORDER BY created_at DESC
LIMIT $2 OFFSET $3`

	queryListOverdueDsarRequests = `
SELECT id, organization_id, subject_type, subject_id, request_type, state, sla_due_at, closed_at, cooling_until, rejection_reason, created_by, created_at, updated_at, revision
FROM guliangiam.dsar_requests
WHERE state NOT IN ('fulfilled', 'rejected') AND sla_due_at < $1
ORDER BY sla_due_at ASC
LIMIT $2`
)

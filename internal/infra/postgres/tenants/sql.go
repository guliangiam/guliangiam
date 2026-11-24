package tenants

const (
	queryInsertIdentity = `
INSERT INTO guliangiam.tenant_identities (
    organization_id, status, isolation_tier, is_built_in, last_error,
    provisioned_at, decommissioned_at, created_at, updated_at, revision
) VALUES (
    $1::uuid, $2, $3, $4, $5,
    $6, $7, $8, $9, $10
)`

	queryGetIdentity = `
SELECT organization_id, status, isolation_tier, is_built_in, COALESCE(last_error, ''),
       provisioned_at, decommissioned_at, created_at, updated_at, revision
FROM guliangiam.tenant_identities
WHERE organization_id = $1::uuid`

	queryDecommissionIdentity = `
UPDATE guliangiam.tenant_identities
SET status = 'decommissioned',
    decommissioned_at = $2,
    updated_at = $2,
    revision = revision + 1
WHERE organization_id = $1::uuid AND status <> 'decommissioned'`

	queryInsertTopology = `
INSERT INTO guliangiam.tenant_instances (
    id, organization_id, region_id, region_code, is_primary, status,
    placement_kind, storage_boundary, backup_boundary, data_residency_country,
    capacity_quota_units, provisioned_at, effective_from, created_at, updated_at, revision
) VALUES (
    $1::uuid, $2::uuid, $3::uuid, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16
)`

	queryGetPrimaryTopology = `
SELECT id, organization_id, region_id, region_code, is_primary, status,
       placement_kind, storage_boundary, backup_boundary, data_residency_country,
       capacity_quota_units, provisioned_at, effective_from, created_at, updated_at, revision
FROM guliangiam.tenant_instances
WHERE organization_id = $1::uuid AND is_primary AND status <> 'decommissioned'`

	queryDecommissionTopologies = `
UPDATE guliangiam.tenant_instances
SET status = 'decommissioned',
    updated_at = $2,
    revision = revision + 1
WHERE organization_id = $1::uuid AND status <> 'decommissioned'`

	// 默认区域来自部署区域字典本身，按 is_primary + region_code 稳定排序，
	// 不读取任何环境变量（M6 裁决）。
	queryPrimaryRegion = `
SELECT id, region_code, country_code
FROM guliangiam.deployment_regions
WHERE is_primary
ORDER BY region_code ASC, id ASC
LIMIT 1`
)

package applications

// Auto-split from sql.go — domain: provider

const (
	queryGetApplicationProviderBindingsByApplicationID = `
SELECT application_id, provider_id
FROM guliangiam.application_provider_bindings
WHERE application_id = $1::uuid
ORDER BY provider_id ASC`

	queryBatchGetApplicationProviderBindingsByApplicationIDs = `
SELECT application_id, provider_id
FROM guliangiam.application_provider_bindings
WHERE application_id = ANY($1::uuid[])
ORDER BY application_id, provider_id ASC`
)

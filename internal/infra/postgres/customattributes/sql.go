package customattributes

const (
	queryListByOrganization = "SELECT id, organization_id, key, data_type, required, options, writable, visibility, description, enabled, created_at, updated_at, revision " +
		"FROM guliangiam.custom_attribute_definitions WHERE organization_id = $1::uuid AND ($2 OR enabled = true) ORDER BY key"
	queryGetByOrganizationAndKey = "SELECT id, organization_id, key, data_type, required, options, writable, visibility, description, enabled, created_at, updated_at, revision " +
		"FROM guliangiam.custom_attribute_definitions WHERE organization_id = $1::uuid AND key = $2"
	queryInsert = "INSERT INTO guliangiam.custom_attribute_definitions " +
		"(id, organization_id, key, data_type, required, options, writable, visibility, description, enabled, created_at, updated_at, revision) " +
		"VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)"
	queryUpdate = "UPDATE guliangiam.custom_attribute_definitions SET key = $3, data_type = $4, required = $5, options = $6, " +
		"writable = $7, visibility = $8, description = $9, enabled = $10, updated_at = $11, revision = revision + 1 " +
		"WHERE id = $1::uuid AND organization_id = $2::uuid AND revision = $12"
	queryDelete = "DELETE FROM guliangiam.custom_attribute_definitions WHERE id = $1::uuid AND organization_id = $2::uuid"
)

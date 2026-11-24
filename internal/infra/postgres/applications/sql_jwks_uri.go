package applications

const (
	queryUpsertClientJWKSURI = "INSERT INTO guliangiam.app_client_jwks_uris (application_id, organization_id, jwks_uri, created_at, updated_at) " +
		"VALUES ($1::uuid, $2::uuid, $3, $4, $4) " +
		"ON CONFLICT (application_id) DO UPDATE SET jwks_uri = EXCLUDED.jwks_uri, updated_at = EXCLUDED.updated_at"
	queryGetClientJWKSURI = "SELECT application_id, organization_id, jwks_uri, created_at, updated_at " +
		"FROM guliangiam.app_client_jwks_uris WHERE organization_id = $1::uuid AND application_id = $2::uuid"
	queryDeleteClientJWKSURI = "DELETE FROM guliangiam.app_client_jwks_uris WHERE organization_id = $1::uuid AND application_id = $2::uuid"
)

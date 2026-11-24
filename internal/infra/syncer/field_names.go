package syncer

// LDAP 属性名与 URL scheme 前缀。属性名同时用于 LDAP 请求属性列表、字段映射与
// 结果 map，必须与目录端属性逐字一致。
const (
	dirAttrGivenName      = "givenName"
	dirAttrEmployeeNumber = "employeeNumber"
	dirAttrTitle          = "title"
	urlSchemeHTTPS        = "https://"

	// AWS Signature V4 的固定 service 名（IAM API）。
	awsIAMServiceName = "iam"
)

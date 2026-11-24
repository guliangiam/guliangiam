package scim

// SCIM 协议属性名、资源类型与 schema 属性类型（RFC 7643/7644 的线上名称）。
// 这些值同时出现在 filter 解析与 PATCH path 分派里，必须逐字一致。
const (
	attrUserName     = "userName"
	attrDisplayName  = "displayName"
	attrName         = "name"
	attrEmail        = "email"
	attrPhone        = "phone"
	attrActive       = "active"
	attrEmails       = "emails"
	attrPhoneNumbers = "phoneNumbers"
	attrMembers      = "members"
	attrGroups       = "groups"

	resourceTypeUser  = "User"
	resourceTypeGroup = "Group"

	attrTypeWork = "work"
)

// users.status 列的取值。它与 SCIM 属性 "active" 同名但不是同一语义：
// 前者是本地用户状态机，后者是 SCIM 布尔属性。枚举事实源：
// contracts/registries/state-machines.yaml §users_status（ck_users_status）。
const (
	userStatusActive   = "active"
	userStatusDisabled = "disabled"
)

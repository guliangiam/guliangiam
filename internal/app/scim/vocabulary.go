package scim

// SCIM 协议词汇（RFC 7643/7644）。资源类型、属性名与 schema 的 returned 取值
// 都是线上契约：IdP 按这些名字做 discovery、filter 与 PATCH 分派，必须逐字一致。
const (
	resourceTypeUser  = "User"
	resourceTypeGroup = "Group"

	scimAttrUserName = "userName"
	scimAttrEmails   = "emails"
	scimAttrActive   = "active"
	scimAttrMembers  = "members"

	// PATCH/filter 路径在分派前统一 ToLower，因此是小写形式。
	pathUsername          = "username"
	pathDisplayName       = "displayname"
	pathPhoneNumbers      = "phonenumbers"
	pathEmailsValue       = "emails.value"
	pathPhoneNumbersValue = "phonenumbers.value"
	pathNameGivenName     = "name.givenname"
	pathNameFamilyName    = "name.familyname"

	returnedDefault = "default"

	// discovery 里 schema 属性的 mutability 与 type 取值。
	mutabilityReadWrite = "readWrite"
	schemaTypeString    = "string"
)

// users.status 列的取值。与 SCIM 属性 active 同名但不是同一语义：前者是本地
// 用户状态机（事实源 contracts/registries/state-machines.yaml §users_status：
// active/disabled/locked/pending_verification），后者是 SCIM 布尔属性。
// SCIM active=false 的列表过滤映射到管理面的停用态 disabled。
const (
	userStatusActive   = "active"
	userStatusDisabled = "disabled"
)

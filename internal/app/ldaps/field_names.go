package ldaps

const (
	// statusDisabled 是 LDAP 同步用户的禁用状态值。
	// 枚举事实源：contracts/registries/state-machines.yaml §users_status（ck_users_status）。
	statusDisabled = "disabled"
	// identityTypeLDAP 是身份记录/目录提供方的类型标识。
	identityTypeLDAP = "ldap"
)

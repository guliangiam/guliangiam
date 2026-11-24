package middleware

// 认证相关端点路径：缓存策略（写操作前缀）与按 IP 限流的默认表共用。
const (
	pathAuthLogin  = "/api/v1/auth/login"
	pathAuthSignup = "/api/v1/auth/signup"
	pathAuthLogout = "/api/v1/auth/logout"
)

// riskLevelFor 使用的 HTTP 方法（小写，与 strings.ToLower(method) 对齐）。
const (
	methodGet     = "get"
	methodHead    = "head"
	methodOptions = "options"
	methodDelete  = "delete"
	methodPost    = "post"
	methodPut     = "put"
	methodPatch   = "patch"
)

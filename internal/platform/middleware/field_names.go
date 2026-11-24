package middleware

// HTTP 访问日志、Prometheus 标签与错误响应共用的字段名。日志检索、指标查询与
// 前端错误解析都按这些名字取值，改名会让看板与告警规则断链。
const (
	fieldMethod  = "method"
	fieldPath    = "path"
	fieldSurface = "surface"
	fieldStatus  = "status"
)

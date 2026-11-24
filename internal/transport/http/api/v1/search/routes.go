package search

import (
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// Routes 返回本模块全部路由声明，供 OpenAPI 生成器与契约校验消费。
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/search",
			OperationID: "GlobalSearch",
			Summary:     "按关键词聚合搜索核心实体",
			Permission:  "search:read",
			Surface:     route.SurfaceManagement,
		},
	}
}

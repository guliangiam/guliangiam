package search

import (
	"context"

	appsearch "github.com/guliangiam/guliangiam/internal/app/search"
)

// Handler 是全局聚合搜索的 transport 入口。
type Handler struct {
	Service appsearch.Service
}

// NewHandler 构造 Handler。
func NewHandler(service appsearch.Service) Handler {
	return Handler{Service: service}
}

// Search 执行聚合搜索并返回按实体分组的结果。
func (h Handler) Search(ctx context.Context, request SearchRequest) (*appsearch.Result, error) {
	return h.Service.Search(ctx, appsearch.Query{
		Keyword:        request.Keyword,
		OrganizationID: request.OrganizationID,
		Limit:          request.Limit,
	})
}

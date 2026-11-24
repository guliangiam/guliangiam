package search

// SearchRequest 是一次全局聚合搜索的已校验输入。
type SearchRequest struct {
	// Keyword 是已 trim 的非空搜索词。
	Keyword string
	// OrganizationID 是可信租户边界；空值仅平台管理员合法（跨组织聚合）。
	OrganizationID string
	// Limit 是总结果上限（<=0 由用例层取默认值）。
	Limit int
}

// Package util provides shared HTTP utility functions for the transport layer.
package util

import (
	nethttp "net/http"

	platformhttp "github.com/guliangiam/guliangiam/internal/platform/http"
)

// ClientIP —— Type-Alias Bridge。
// 单一事实数据源在 platform/http（消除 platform/middleware 反向依赖 transport）。
// 本包仅保留 transport 层的向后兼容入口。
func ClientIP(r *nethttp.Request) string { return platformhttp.ClientIP(r) }

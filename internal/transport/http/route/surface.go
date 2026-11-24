// Package route hosts the HTTP routing contract shared by every API module.
package route

import registry "github.com/guliangiam/guliangiam/internal/platform/registry"

// Surface = registry.Surface —— Type-Alias Bridge 模式。
// 本包是 transport 层 API 门面；单一事实数据源位于 platform/registry，
// middleware / platform 层直接依赖那里，避免 platform 反向依赖 transport。
type Surface = registry.Surface

const (
	SurfaceManagement     = registry.SurfaceManagement
	SurfaceAuthentication = registry.SurfaceAuthentication
	SurfaceProtocol       = registry.SurfaceProtocol
)

var (
	AllSurfaces = registry.AllSurfaces
	SurfaceFor  = registry.SurfaceFor
)

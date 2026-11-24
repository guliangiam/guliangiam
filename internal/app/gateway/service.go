package gateway

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.UnitOfWork
	MCP   domain.MCPProxy
}

// 租户作用域说明：本包不做 RLS 提升。servers/sites 的 organizationId 由 transport
// 层用 TrustedOrganizationID 收敛（query 读与 body 写）；平台管理员跨组织访问由
// ScopeAuthorizer 放行，service 不自行越权。
type CreateServerCommand struct {
	OrganizationID string
	Name           string
	Endpoint       string
	Protocol       string
	Status         string
	Metadata       map[string]any
}

type UpdateServerCommand struct {
	ID       string
	Name     string
	Endpoint string
	Protocol string
	Status   string
	Metadata map[string]any
	Revision int64
}

type CreateSiteCommand struct {
	OrganizationID      string
	Name                string
	Host                string
	Status              string
	SslMode             string
	CertID              string
	ForceHttps          bool
	EnableHealthCheck   bool
	HealthCheckInterval int
	EnableAlert         bool
	AlertInterval       int
	AlertTryTimes       int
	AlertProviderIDs    []string
	Metadata            map[string]any
}

type UpdateSiteCommand struct {
	ID                  string
	Name                string
	Host                string
	Status              string
	SslMode             string
	CertID              string
	ForceHttps          bool
	EnableHealthCheck   bool
	HealthCheckInterval int
	EnableAlert         bool
	AlertInterval       int
	AlertTryTimes       int
	AlertProviderIDs    []string
	Metadata            map[string]any
	Revision            int64
}

package postgres

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// 租户作用域原语定义在 internal/platform/tenantctx（跨层共享的纯上下文值，
// 不依赖数据库驱动）。这里以别名/函数变量重导出，保持既有仓储的 import 与调用点
// 不变，同时让 platform 包不再反向依赖 infra/（分层门禁 Rule 7）。
type TenantFilterMode = tenantctx.TenantFilterMode

const (
	TenantFilterStrict     = tenantctx.TenantFilterStrict
	TenantFilterOptional   = tenantctx.TenantFilterOptional
	TenantFilterSuperAdmin = tenantctx.TenantFilterSuperAdmin
	TenantFilterSystem     = tenantctx.TenantFilterSystem
	TenantFilterCross      = tenantctx.TenantFilterCross
)

type TenantScope = tenantctx.TenantScope

var (
	WithTenantScope        = tenantctx.WithTenantScope
	TenantScopeFromContext = tenantctx.TenantScopeFromContext
	TenantWhereClause      = tenantctx.TenantWhereClause
	TenantAndClause        = tenantctx.TenantAndClause
)

var _ = context.Background // 保持 context 导入用于别名可读性（删除会触发 unused）

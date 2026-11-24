package platformadmins

import (
	appplatformadmins "github.com/guliangiam/guliangiam/internal/app/platform_admins"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// 响应投影全部复用 app 层 DTO（transport 不 import domain）。
type (
	// PlatformAdminResponse 单个平台管理员详情。
	PlatformAdminResponse = appplatformadmins.PlatformAdminDetail
	// ScopeResponse 单个作用域详情。
	ScopeResponse = appplatformadmins.ScopeDetail
	// ListPlatformAdminsResponse 分页列表信封。
	ListPlatformAdminsResponse = presenter.ListResponse[appplatformadmins.PlatformAdminDetail]
	// ListScopesResponse 作用域列表信封（无分页参数，page 元数据按全量口径填充）。
	ListScopesResponse = presenter.ListResponse[appplatformadmins.ScopeDetail]
)

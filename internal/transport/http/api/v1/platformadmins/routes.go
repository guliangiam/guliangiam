package platformadmins

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes 声明平台管理员管理面的路由契约（BE-A-05）。
// Permission 与 PEP 运行期派生一致：路径首段 platform-admins /
// platform-admin-scopes 即 Casbin 资源，GET→read、POST/PATCH→write、
// DELETE→delete（auth_rbac_models.go resourceFromPath + casbinActionCandidates）。
// 嵌套 /platform-admins/{id}/scopes 的声明资源与派生宿主资源一致，不触发
// NESTED 失配；:set-active / :approve 自定义动作沿用 payments 的 colon-verb 先例。
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/platform-admins",
			OperationID: "ListPlatformAdmins",
			Summary:     "List platform admins",
			Permission:  "platform-admins:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:         "POST",
			Pattern:        "/api/v1/platform-admins",
			OperationID:    "CreatePlatformAdmin",
			Summary:        "Register a user as platform admin",
			Permission:     "platform-admins:write",
			Surface:        route.SurfaceManagement,
			SuccessStatus:  201,
			ErrorResponses: []int{400, 401, 403, 409},
		},
		{
			Method:         "GET",
			Pattern:        "/api/v1/platform-admins/{id}",
			OperationID:    "GetPlatformAdmin",
			Summary:        "Get a platform admin by ID",
			Permission:     "platform-admins:read",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{400, 404},
		},
		{
			Method:         "PATCH",
			Pattern:        "/api/v1/platform-admins/{id}",
			OperationID:    "PatchPlatformAdmin",
			Summary:        "Update platform admin audit fields (operator pinned)",
			Permission:     "platform-admins:write",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{400, 401, 403, 404},
		},
		{
			Method:         "DELETE",
			Pattern:        "/api/v1/platform-admins/{id}",
			OperationID:    "DeletePlatformAdmin",
			Summary:        "Delete a platform admin (irreversible, revokes sessions)",
			Permission:     "platform-admins:delete",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{400},
		},
		{
			Method:         "POST",
			Pattern:        "/api/v1/platform-admins/{id}:set-active",
			OperationID:    "SetPlatformAdminActive",
			Summary:        "Activate or deactivate a platform admin",
			Permission:     "platform-admins:write",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{400, 404},
		},
		{
			Method:         "GET",
			Pattern:        "/api/v1/platform-admins/{id}/scopes",
			OperationID:    "ListPlatformAdminScopes",
			Summary:        "List tenant scopes of a platform admin",
			Permission:     "platform-admins:read",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{400},
		},
		{
			Method:         "POST",
			Pattern:        "/api/v1/platform-admins/{id}/scopes",
			OperationID:    "CreatePlatformAdminScope",
			Summary:        "Create a tenant scope for a platform admin (approver pinned to operator)",
			Permission:     "platform-admins:write",
			Surface:        route.SurfaceManagement,
			SuccessStatus:  201,
			ErrorResponses: []int{400, 401, 403},
		},
		{
			Method:         "GET",
			Pattern:        "/api/v1/platform-admin-scopes/{scopeId}",
			OperationID:    "GetPlatformAdminScope",
			Summary:        "Get a platform admin scope by ID",
			Permission:     "platform-admin-scopes:read",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{400, 404},
		},
		{
			Method:         "DELETE",
			Pattern:        "/api/v1/platform-admin-scopes/{scopeId}",
			OperationID:    "DeletePlatformAdminScope",
			Summary:        "Delete a platform admin scope",
			Permission:     "platform-admin-scopes:delete",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{400},
		},
		{
			Method:         "POST",
			Pattern:        "/api/v1/platform-admin-scopes/{scopeId}:approve",
			OperationID:    "ApprovePlatformAdminScope",
			Summary:        "Approve a pending platform admin scope (approver pinned to operator)",
			Permission:     "platform-admin-scopes:write",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{400, 401, 403},
		},
	}
}

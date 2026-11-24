package b2b

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// B2B 域名治理 API — functional.md §15 MODULE-B2B。
//
// RESTful 设计：
//
//	POST   /api/v1/organizations/{orgId}/domains          认领域名（ClaimDomain）
//	GET    /api/v1/organizations/{orgId}/domains          列出已认领域名
//	DELETE /api/v1/organizations/{orgId}/domains/{domainId} 撤销认领
//	POST   /api/v1/organizations/{orgId}/domains/{domainId}/verify  手动触发 DNS 验证
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "POST",
			Pattern:     "/api/v1/organizations/{orgId}/domains",
			OperationID: "ClaimOrganizationDomain",
			Summary:     "Claim B2B domain for DNS TXT verification",
			Permission:  "b2b:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/organizations/{orgId}/domains",
			OperationID: "ListOrganizationDomains",
			Summary:     "List claimed domains for organization",
			Permission:  "b2b:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/organizations/{orgId}/domains/{domainId}",
			OperationID: "DeleteOrganizationDomain",
			Summary:     "Delete domain claim",
			Permission:  "b2b:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 204,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/organizations/{orgId}/domains/{domainId}/verify",
			OperationID: "VerifyOrganizationDomain",
			Summary:     "Manually trigger DNS TXT verification",
			Permission:  "b2b:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 202,
		},
	}
}

package ldaps

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/ldaps",
			OperationID: "ListLdaps",
			Summary:     "List LDAP configurations",
			Permission:  "ldaps:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/ldaps",
			OperationID: "CreateLdap",
			Summary:     "Create LDAP configuration",
			Permission:  "ldaps:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/ldaps/{id}",
			OperationID: "GetLdap",
			Summary:     "Get LDAP configuration detail",
			Permission:  "ldaps:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/ldaps/{id}",
			OperationID: "PatchLdap",
			Summary:     "Patch LDAP configuration",
			Permission:  "ldaps:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/ldaps/{id}",
			OperationID: "DeleteLdap",
			Summary:     "Delete LDAP configuration",
			Permission:  "ldaps:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/ldaps/{id}/sync",
			OperationID: "SyncLdapUsers",
			Summary:     "Sync LDAP users",
			Permission:  "ldaps:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/ldaps/{id}/preview-users",
			OperationID: "PreviewLdapUsers",
			Summary:     "Preview LDAP users before sync",
			Permission:  "ldaps:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/ldaps/{id}/test-connection",
			OperationID: "TestLdapConnection",
			Summary:     "Test LDAP server connection",
			Permission:  "ldaps:write",
			Surface:     route.SurfaceManagement,
		},
	}
}

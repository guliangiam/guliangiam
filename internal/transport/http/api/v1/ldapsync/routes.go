package ldapsync

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the Casbin permission definitions for LDAP sync configuration
// endpoints. The runtime registration lives in handler.go (RegisterRoutes);
// these declarations are the contract consumed by the route-contract gate and
// the OpenAPI generator, so the two must stay in sync.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/ldap-sync-configs",
			OperationID: "ListLdapSyncConfigs",
			Summary:     "List LDAP sync configurations",
			Permission:  "ldaps:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/ldap-sync-configs",
			OperationID: "CreateLdapSyncConfig",
			Summary:     "Create LDAP sync configuration",
			Permission:  "ldaps:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/ldap-sync-configs/{id}",
			OperationID: "GetLdapSyncConfig",
			Summary:     "Get LDAP sync configuration detail",
			Permission:  "ldaps:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/ldap-sync-configs/{id}",
			OperationID: "UpdateLdapSyncConfig",
			Summary:     "Update LDAP sync configuration",
			Permission:  "ldaps:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/ldap-sync-configs/{id}",
			OperationID: "DeleteLdapSyncConfig",
			Summary:     "Delete LDAP sync configuration",
			Permission:  "ldaps:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 204,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/ldap-sync-configs/{id}/sync",
			OperationID: "TriggerLdapSync",
			Summary:     "Trigger an immediate LDAP sync",
			Permission:  "ldaps:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/ldap-sync-configs/{id}/logs",
			OperationID: "ListLdapSyncLogs",
			Summary:     "List sync task logs for an LDAP sync configuration",
			Permission:  "ldaps:read",
			Surface:     route.SurfaceManagement,
		},
	}
}

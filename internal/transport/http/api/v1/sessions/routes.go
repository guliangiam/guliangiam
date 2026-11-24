package sessions

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/sessions/current",
			OperationID: "GetCurrentSession",
			Summary:     "Get current session detail",
			Permission:  "sessions:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/sessions/current/revoke-others",
			OperationID: "PostCurrentSessionRevokeOthers",
			Summary:     "Revoke other sessions for current user",
			Permission:  "sessions:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/sessions",
			OperationID: "ListSessions",
			Summary:     "List sessions",
			Permission:  "sessions:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/sessions/{id}",
			OperationID: "GetSession",
			Summary:     "Get session detail",
			Permission:  "sessions:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/sessions/{id}",
			OperationID: "DeleteSession",
			Summary:     "Delete session",
			Permission:  "sessions:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/sessions/{id}/revoke",
			OperationID: "RevokeSession",
			Summary:     "Revoke session",
			Permission:  "sessions:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/sessions/{id}",
			OperationID: "PatchSession",
			Summary:     "Update session fields such as state",
			Permission:  "sessions:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/sessions/is-duplicated",
			OperationID: "CheckSessionDuplicated",
			Summary:     "Check if a user has duplicate active sessions",
			Permission:  "sessions:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/sessions/force-logout",
			OperationID: "ForceLogoutUser",
			Summary:     "Force-logout a user by revoking all their active sessions",
			Permission:  "sessions:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/sessions/force-logout-organization",
			OperationID: "ForceLogoutOrganization",
			Summary:     "Force-logout all sessions in an organization (emergency lockdown)",
			Permission:  "sessions:write",
			Surface:     route.SurfaceManagement,
		},
	}
}

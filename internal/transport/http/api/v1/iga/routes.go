package iga

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/iga/access-campaigns",
			OperationID: "ListAccessCampaigns",
			Summary:     "List access review campaigns",
			Permission:  "iga:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/iga/access-campaigns",
			OperationID: "CreateAccessCampaign",
			Summary:     "Create an access review campaign",
			Permission:  "iga:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/iga/access-campaigns/{id}",
			OperationID: "GetAccessCampaign",
			Summary:     "Get an access campaign by ID",
			Permission:  "iga:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/iga/access-campaigns/{id}/status",
			OperationID: "UpdateCampaignStatus",
			Summary:     "Transition campaign lifecycle status",
			Permission:  "iga:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/iga/sod-rules",
			OperationID: "ListSODRules",
			Summary:     "List Segregation of Duties rules",
			Permission:  "iga:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/iga/sod-rules",
			OperationID: "CreateSODRule",
			Summary:     "Create an SOD rule",
			Permission:  "iga:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/iga/sod-rules/{id}",
			OperationID: "GetSODRule",
			Summary:     "Get an SOD rule by ID",
			Permission:  "iga:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/iga/access-lifecycle",
			OperationID: "ListAccessLifecycle",
			Summary:     "List access lifecycle events",
			Permission:  "iga:read",
			Surface:     route.SurfaceManagement,
		},
		// Missing CRUD endpoints added for completeness (opt-backend-api)
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/iga/access-campaigns/{id}",
			OperationID: "DeleteAccessCampaign",
			Summary:     "Delete an access review campaign",
			Permission:  "iga:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 204,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/iga/sod-rules/{id}",
			OperationID: "UpdateSODRule",
			Summary:     "Update an SOD rule",
			Permission:  "iga:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/iga/sod-rules/{id}",
			OperationID: "DeleteSODRule",
			Summary:     "Delete an SOD rule",
			Permission:  "iga:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 204,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/iga/access-lifecycle",
			OperationID: "CreateAccessLifecycle",
			Summary:     "Grant access (create lifecycle event)",
			Permission:  "iga:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/iga/access-lifecycle/{id}",
			OperationID: "GetAccessLifecycle",
			Summary:     "Get an access lifecycle event by ID",
			Permission:  "iga:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/iga/access-lifecycle/{id}/status",
			OperationID: "UpdateAccessLifecycleStatus",
			Summary:     "Revoke or expire an access grant",
			Permission:  "iga:write",
			Surface:     route.SurfaceManagement,
		},
		{Method: "GET", Pattern: "/api/v1/iga/access-requests", OperationID: "ListAccessRequests", Summary: "List IGA access requests", Permission: "iga:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/iga/access-requests", OperationID: "CreateAccessRequest", Summary: "Create an IGA access request", Permission: "iga:write", Surface: route.SurfaceManagement, SuccessStatus: 201, ResponseHeaders: []string{"Location"}},
		{Method: "GET", Pattern: "/api/v1/iga/access-requests/{id}", OperationID: "GetAccessRequest", Summary: "Get an IGA access request and approval transcript", Permission: "iga:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/iga/access-requests/{id}/submit", OperationID: "SubmitAccessRequest", Summary: "Submit an access request for manager approval", Permission: "iga:write", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/iga/access-requests/{id}/approve", OperationID: "ApproveAccessRequest", Summary: "Approve the next access request stage", Permission: "iga:write", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/iga/access-requests/{id}/reject", OperationID: "RejectAccessRequest", Summary: "Reject the current access request stage", Permission: "iga:write", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/iga/access-requests/{id}/withdraw", OperationID: "WithdrawAccessRequest", Summary: "Withdraw an access request", Permission: "iga:write", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/iga/access-requests/{id}/provision", OperationID: "ProvisionAccessRequest", Summary: "Provision an approved access request", Permission: "iga:write", Surface: route.SurfaceManagement},
		{Method: "GET", Pattern: "/api/v1/iga/privileged-sessions", OperationID: "ListPrivilegedSessions", Summary: "List privileged sessions for the current user", Permission: "iga:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/iga/privileged-sessions", OperationID: "StartPrivilegedSession", Summary: "Elevate the current session using a provisioned request", Permission: "iga:write", Surface: route.SurfaceManagement, SuccessStatus: 201},
	}
}

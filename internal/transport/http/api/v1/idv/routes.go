package idv

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/idv/verifications",
			OperationID: "ListIDVVerifications",
			Summary:     "List identity-verification records across providers",
			Permission:  "idv:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/idv/verifications/{id}",
			OperationID: "GetIDVVerification",
			Summary:     "Get an identity-verification record by ID",
			Permission:  "idv:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/idv/verifications/{id}",
			OperationID: "DeleteIDVVerification",
			Summary:     "Delete an identity-verification record",
			Permission:  "idv:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 204,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/idv/callback/{provider}",
			OperationID: "HandleIDVCallback",
			Summary:     "Handle a third-party identity-verification callback",
			// Public: providers authenticate with a webhook signature, not a
			// session. The route is registered on the public face and must stay
			// in the runtime public whitelist (auth_paths.go).
			Permission: "public",
			Surface:    route.SurfaceManagement,
		},
	}
}

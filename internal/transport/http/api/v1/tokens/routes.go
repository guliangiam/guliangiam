package tokens

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/tokens",
			OperationID: "ListTokens",
			Summary:     "List access tokens",
			Permission:  "tokens:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/tokens",
			OperationID: "CreateToken",
			Summary:     "Create access token",
			Permission:  "tokens:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/tokens/revoke",
			OperationID: "RevokeTokens",
			Summary:     "Revoke access tokens by filter",
			Permission:  "tokens:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/tokens/introspect",
			OperationID: "IntrospectToken",
			Summary:     "Introspect access token",
			Permission:  "tokens:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/tokens/refresh",
			OperationID: "RefreshToken",
			Summary:     "Refresh access token",
			Permission:  "tokens:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/tokens/{id}",
			OperationID: "GetToken",
			Summary:     "Get access token detail",
			Permission:  "tokens:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/tokens/{id}/revoke",
			OperationID: "RevokeToken",
			Summary:     "Revoke access token",
			Permission:  "tokens:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/tokens/{id}",
			OperationID: "DeleteToken",
			Summary:     "Delete access token",
			Permission:  "tokens:write",
			Surface:     route.SurfaceManagement,
		},
	}
}

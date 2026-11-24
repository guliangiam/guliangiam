package dcr

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "POST",
			Pattern:     "/oauth2/register",
			OperationID: "RegisterOAuthClient",
			Summary:     "Register OAuth2 client (RFC 7591)",
			Permission:  "dcr:write",
			Surface:     route.SurfaceProtocol,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/oauth2/register/{clientID}",
			OperationID: "GetOAuthClient",
			Summary:     "Get OAuth2 client configuration",
			Permission:  "dcr:read",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "PUT",
			Pattern:     "/oauth2/register/{clientID}",
			OperationID: "UpdateOAuthClient",
			Summary:     "Update OAuth2 client configuration",
			Permission:  "dcr:write",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "DELETE",
			Pattern:     "/oauth2/register/{clientID}",
			OperationID: "DeleteOAuthClient",
			Summary:     "Delete OAuth2 client",
			Permission:  "dcr:write",
			Surface:     route.SurfaceProtocol,

			SuccessStatus: 204,
		},
	}
}

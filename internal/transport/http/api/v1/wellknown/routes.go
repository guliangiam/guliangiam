package wellknown

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/.well-known/openid-configuration",
			OperationID: "GetOpenIDConfiguration",
			Summary:     "OpenID Connect Discovery",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/{application}/openid-configuration",
			OperationID: "GetOpenIDConfigurationByApplication",
			Summary:     "OpenID Connect Discovery (per application)",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/jwks.json",
			OperationID: "GetJWKS",
			Summary:     "JSON Web Key Set",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/jwks-gm.json",
			OperationID: "GetGMJWKS",
			Summary:     "JSON Web Key Set (SM2-SM3)",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/{application}/jwks.json",
			OperationID: "GetJWKSByApplication",
			Summary:     "JSON Web Key Set (per application)",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/webfinger",
			OperationID: "GetWebFinger",
			Summary:     "WebFinger discovery",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/oauth-authorization-server",
			OperationID: "GetOAuthAuthorizationServer",
			Summary:     "OAuth 2.0 Authorization Server Metadata (RFC 8414)",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/oauth-protected-resource",
			OperationID: "GetOAuthProtectedResource",
			Summary:     "OAuth 2.0 Protected Resource Metadata (RFC 9728)",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/{application}/oauth-protected-resource",
			OperationID: "GetOAuthProtectedResourceByApplication",
			Summary:     "OAuth 2.0 Protected Resource Metadata (per application, RFC 9728)",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/saml-metadata",
			OperationID: "GetSAMLMetadata",
			Summary:     "SAML metadata discovery",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/.well-known/scim-configuration",
			OperationID: "GetSCIMConfiguration",
			Summary:     "SCIM service provider configuration discovery",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/oauth2/userinfo",
			OperationID: "GetUserInfo",
			Summary:     "OpenID Connect UserInfo endpoint",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,
		},
	}
}

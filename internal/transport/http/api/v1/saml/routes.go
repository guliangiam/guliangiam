package saml

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		// SP → IdP：为外部 SP 生成 AuthnRequest（OPEN-FE-07 补全）。
		// 允许控制台 / 外部 SP POST organizationSlug + applicationSlug + spEntityId + spAcsUrl，
		// 后端返回 Base64 AuthnRequest + IdP SSO URL，由前端完成重定向。
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/saml/authn-request/{organizationSlug}/{applicationSlug}",
			OperationID: "BuildSamlAuthnRequest",
			Summary:     "Build SAML AuthnRequest for external SP-initiated SSO",
			Permission:  "auth.saml",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/auth/saml/redirect/{organizationSlug}/{applicationSlug}",
			OperationID: "SamlRedirect",
			Summary:     "Redirect to SAML IdP",
			Permission:  "auth.saml",
			Surface:     route.SurfaceProtocol,

			SuccessStatus:   303,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/auth/saml/metadata/{organizationSlug}/{applicationId}",
			OperationID: "GetSamlAuthMetadata",
			Summary:     "Get SAML authentication metadata",
			Permission:  "auth.saml",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/saml/acs",
			OperationID: "SamlAcs",
			Summary:     "SAML Assertion Consumer Service",
			Permission:  "auth.saml",
			Surface:     route.SurfaceProtocol,
		},
		// SP role: browser ACS for an external IdP. Public (root group): no
		// session exists yet and the IdP POST is cross-site.
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/saml/acs/{organizationSlug}/{providerId}",
			OperationID: "SamlSpAcs",
			Summary:     "SAML SP browser Assertion Consumer Service",
			Permission:  "public",
			Surface:     route.SurfaceProtocol,

			SuccessStatus:   303,
			ResponseHeaders: []string{"Location", "Set-Cookie"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/saml/metadata/{applicationId}",
			OperationID: "GetSamlMetadata",
			Summary:     "Get SAML SP metadata",
			Permission:  "saml:read",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/saml/sso/{applicationId}",
			OperationID: "SamlSsoPost",
			Summary:     "SAML SSO (POST binding)",
			Permission:  "saml:read",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/saml/sso/{applicationId}",
			OperationID: "SamlSsoGet",
			Summary:     "SAML SSO (GET binding)",
			Permission:  "saml:read",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/saml/slo/{applicationId}",
			OperationID: "SamlSloGet",
			Summary:     "SAML SLO (GET binding)",
			Permission:  "saml:read",
			Surface:     route.SurfaceProtocol,

			SuccessStatus:   302,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/saml/slo",
			OperationID: "SamlSloPost",
			Summary:     "SAML SLO (POST binding)",
			Permission:  "saml:read",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/saml/slo",
			OperationID: "SamlSloRedirect",
			Summary:     "SAML SLO redirect",
			Permission:  "saml:read",
			Surface:     route.SurfaceProtocol,
		},
	}
}

package webauthn

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/auth/webauthn/credentials",
			OperationID: "ListWebAuthnCredentials",
			Summary:     "List the authenticated user's WebAuthn credentials",
			Permission:  "webauthn:read",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:        "DELETE",
			Pattern:       "/api/v1/auth/webauthn/credentials/{id}",
			OperationID:   "RevokeWebAuthnCredential",
			Summary:       "Revoke one of the authenticated user's WebAuthn credentials",
			Permission:    "webauthn:write",
			Surface:       route.SurfaceProtocol,
			SuccessStatus: 204,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/webauthn/registration/begin",
			OperationID: "WebAuthnRegistrationBegin",
			Summary:     "Begin WebAuthn registration ceremony",
			Permission:  "webauthn:write",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/webauthn/registration/finish",
			OperationID: "WebAuthnRegistrationFinish",
			Summary:     "Finish WebAuthn registration ceremony",
			Permission:  "webauthn:write",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/webauthn/authentication/begin",
			OperationID: "WebAuthnAuthenticationBegin",
			Summary:     "Begin WebAuthn authentication ceremony",
			Permission:  "webauthn.auth",
			Surface:     route.SurfaceProtocol,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/webauthn/authentication/finish",
			OperationID: "WebAuthnAuthenticationFinish",
			Summary:     "Finish WebAuthn authentication ceremony",
			Permission:  "webauthn.auth",
			Surface:     route.SurfaceProtocol,
		},
	}
}

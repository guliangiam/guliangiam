package certs

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the Casbin permission definitions for certificate endpoints.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/certs",
			OperationID: "ListCerts",
			Summary:     "List certificates",
			Permission:  "certs:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/certs",
			OperationID: "CreateCert",
			Summary:     "Create a certificate",
			Permission:  "certs:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/certs/{id}",
			OperationID: "GetCert",
			Summary:     "Get a certificate by ID",
			Permission:  "certs:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/certs/{id}",
			OperationID: "PatchCert",
			Summary:     "Patch a certificate",
			Permission:  "certs:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/certs/{id}",
			OperationID: "DeleteCert",
			Summary:     "Delete a certificate",
			Permission:  "certs:admin",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/certs/{id}/update-domain-expire",
			OperationID: "UpdateCertDomainExpire",
			Summary:     "Update domain expiration time for a certificate",
			Permission:  "certs:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/certs/{id}/check-domain-expiration",
			OperationID: "CheckCertDomainExpiration",
			Summary:     "Check domain expiration for a certificate",
			Permission:  "certs:write",
			Surface:     route.SurfaceManagement,
		},
	}
}

// GlobalRoutes returns the Casbin permission definitions for global certificate endpoints.
func GlobalRoutes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/global/certs",
			OperationID: "ListGlobalCerts",
			Summary:     "List all certificates across organizations",
			Permission:  "certs:read",
			Surface:     route.SurfaceManagement,
		},
	}
}

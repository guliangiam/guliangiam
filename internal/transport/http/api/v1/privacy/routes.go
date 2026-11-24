package privacy

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the permission/contract definitions of the DSAR endpoints.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/privacy/dsar",
			OperationID: "ListDsarRequests",
			Summary:     "List data subject access requests",
			Permission:  "privacy:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:        "POST",
			Pattern:       "/api/v1/privacy/dsar",
			OperationID:   "CreateDsarRequest",
			Summary:       "Create a data subject access request",
			Permission:    "privacy:write",
			Surface:       route.SurfaceManagement,
			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/privacy/dsar/overdue",
			OperationID: "ListOverdueDsarRequests",
			Summary:     "List overdue DSAR requests",
			Permission:  "privacy:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/privacy/dsar/{id}",
			OperationID: "GetDsarRequest",
			Summary:     "Get a data subject access request",
			Permission:  "privacy:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:          "GET",
			Pattern:         "/api/v1/privacy/exports/{id}",
			OperationID:     "ExportPrivacyEvidence",
			Summary:         "Download a DSAR evidence package as JSON Lines",
			Permission:      "privacy:read",
			Surface:         route.SurfaceManagement,
			ResponseHeaders: []string{"Content-Disposition", "X-Checksum-Sha256", "X-Record-Count", "X-Region"},
		},
		{
			Method:         "POST",
			Pattern:        "/api/v1/privacy/dsar/{id}/erase-review",
			OperationID:    "ReviewDsarErasure",
			Summary:        "Review the two-track erasure scope of an erase DSAR",
			Permission:     "privacy:erase",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{409, 422},
		},
		{
			Method:         "POST",
			Pattern:        "/api/v1/privacy/dsar/{id}/transition",
			OperationID:    "TransitionDsarRequest",
			Summary:        "Advance a DSAR request state",
			Permission:     "privacy:write",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{409, 422},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/privacy/consents",
			OperationID: "ListConsentRecords",
			Summary:     "List the consent version chain of a data subject",
			Permission:  "privacy:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:        "POST",
			Pattern:       "/api/v1/privacy/consents",
			OperationID:   "GrantConsentRecord",
			Summary:       "Grant consent (append a new version)",
			Permission:    "privacy:write",
			Surface:       route.SurfaceManagement,
			SuccessStatus: 201,
		},
		{
			Method:         "POST",
			Pattern:        "/api/v1/privacy/consents/{purpose}/withdraw",
			OperationID:    "WithdrawConsentRecord",
			Summary:        "Withdraw consent by stamping the latest version",
			Permission:     "privacy:write",
			Surface:        route.SurfaceManagement,
			ErrorResponses: []int{404},
		},
	}
}

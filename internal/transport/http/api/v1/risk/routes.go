package risk

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/risk/signals",
			OperationID: "ListRiskSignals",
			Summary:     "List risk signals",
			Permission:  "risk:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/risk/decisions",
			OperationID: "ListRiskDecisions",
			Summary:     "List risk decisions",
			Permission:  "risk:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/risk/decisions/{id}/review",
			OperationID: "ReviewRiskDecision",
			Summary:     "Review / adjust a risk decision",
			Permission:  "risk:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/risk/decisions/{id}/appeal",
			OperationID: "SubmitRiskDecisionAppeal",
			Summary:     "Submit a false-positive appeal for a risk decision",
			Permission:  "risk:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/risk/decisions/{id}/appeal/resolution",
			OperationID: "ResolveRiskDecisionAppeal",
			Summary:     "Resolve a pending risk decision appeal",
			Permission:  "risk:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/risk/session-governance",
			OperationID: "ListSessionGovernance",
			Summary:     "List session governance (lockout / step-up)",
			Permission:  "risk:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/risk/session-governance/{id}/transition",
			OperationID: "TransitionSessionGovernance",
			Summary:     "Append a legal session governance transition",
			Permission:  "risk:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/risk/audit-integrity",
			OperationID: "VerifyRiskAuditIntegrity",
			Summary:     "Verify the tenant audit hash chain read-only",
			Permission:  "risk:read",
			Surface:     route.SurfaceManagement,
		},
		// Missing detail endpoints added for completeness (opt-backend-api)
		{
			Method:      "GET",
			Pattern:     "/api/v1/risk/signals/{id}",
			OperationID: "GetRiskSignal",
			Summary:     "Get a risk signal by ID",
			Permission:  "risk:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/risk/signals",
			OperationID: "ReportRiskSignal",
			Summary:     "Manually report a risk signal",
			Permission:  "risk:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/risk/decisions/{id}",
			OperationID: "GetRiskDecision",
			Summary:     "Get a risk decision by ID",
			Permission:  "risk:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/risk/session-governance/{id}",
			OperationID: "GetSessionGovernance",
			Summary:     "Get a session governance record by ID",
			Permission:  "risk:read",
			Surface:     route.SurfaceManagement,
		},
	}
}

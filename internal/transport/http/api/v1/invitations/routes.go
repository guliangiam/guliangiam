package invitations

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/invitations",
			OperationID: "ListInvitations",
			Summary:     "List invitations",
			Permission:  "invitations:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/invitations",
			OperationID: "CreateInvitation",
			Summary:     "Create invitation",
			Permission:  "invitations:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/invitations/{id}",
			OperationID: "GetInvitation",
			Summary:     "Get invitation detail",
			Permission:  "invitations:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/invitations/{id}/revoke",
			OperationID: "RevokeInvitation",
			Summary:     "Revoke invitation",
			Permission:  "invitations:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/invitations/{id}/resend",
			OperationID: "ResendInvitation",
			Summary:     "Resend invitation",
			Permission:  "invitations:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/invitations/{id}/send",
			OperationID: "SendInvitation",
			Summary:     "Send invitation email",
			Permission:  "invitations:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/invitations/verify",
			OperationID: "VerifyInvitation",
			Summary:     "Verify invitation code (public)",
			Permission:  "public",
			Surface:     route.SurfaceManagement,
		},
	}
}

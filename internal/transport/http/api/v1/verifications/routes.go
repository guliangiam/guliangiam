package verifications

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/verifications",
			OperationID: "ListVerifications",
			Summary:     "List verifications",
			Permission:  "verification:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/verifications/{id}",
			OperationID: "GetVerification",
			Summary:     "Get verification detail",
			Permission:  "verification:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/verification-code/send",
			OperationID: "SendVerificationCode",
			Summary:     "Send a verification code via email or SMS",
			Permission:  "public",
			Surface:     route.SurfaceAuthentication,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/auth/verification-code/verify",
			OperationID: "VerifyVerificationCode",
			Summary:     "Verify a verification code",
			Permission:  "public",
			Surface:     route.SurfaceAuthentication,
		},
	}
}

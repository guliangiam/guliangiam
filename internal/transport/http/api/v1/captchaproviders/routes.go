package captchaproviders

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/captcha-providers",
			OperationID: "ListCaptchaProviders",
			Summary:     "List captcha providers",
			Permission:  "captcha-providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/captcha-providers",
			OperationID: "CreateCaptchaProvider",
			Summary:     "Create captcha provider",
			Permission:  "captcha-providers:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/captcha-providers/{id}",
			OperationID: "GetCaptchaProvider",
			Summary:     "Get captcha provider detail",
			Permission:  "captcha-providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/captcha-providers/{id}",
			OperationID: "PatchCaptchaProvider",
			Summary:     "Patch captcha provider",
			Permission:  "captcha-providers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/captcha-providers/{id}",
			OperationID: "DeleteCaptchaProvider",
			Summary:     "Delete captcha provider",
			Permission:  "captcha-providers:write",
			Surface:     route.SurfaceManagement,
		},
	}
}

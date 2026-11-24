package coupons

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/coupons",
			OperationID: "ListCoupons",
			Summary:     "List coupons",
			Permission:  "coupons:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/coupons",
			OperationID: "CreateCoupon",
			Summary:     "Create coupon",
			Permission:  "coupons:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/coupons/{id}",
			OperationID: "GetCoupon",
			Summary:     "Get coupon detail",
			Permission:  "coupons:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/coupons/{id}",
			OperationID: "PatchCoupon",
			Summary:     "Patch coupon",
			Permission:  "coupons:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/coupons/{id}",
			OperationID: "DeleteCoupon",
			Summary:     "Delete coupon",
			Permission:  "coupons:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/coupons/validate",
			OperationID: "ValidateCoupon",
			Summary:     "Validate a coupon code",
			Permission:  "coupons:read",
			Surface:     route.SurfaceManagement,
		},
	}
}

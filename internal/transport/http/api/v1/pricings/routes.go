package pricings

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{Method: "GET", Pattern: "/api/v1/pricings", OperationID: "ListPricings", Summary: "List pricings", Permission: "pricings:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/pricings", OperationID: "CreatePricing", Summary: "Create a pricing", Permission: "pricings:write", Surface: route.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/pricings/{id}", OperationID: "GetPricing", Summary: "Get a pricing by ID", Permission: "pricings:read", Surface: route.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/pricings/{id}", OperationID: "PatchPricing", Summary: "Patch a pricing", Permission: "pricings:write", Surface: route.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/pricings/{id}", OperationID: "DeletePricing", Summary: "Delete a pricing", Permission: "pricings:admin", Surface: route.SurfaceManagement},
	}
}

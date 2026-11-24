package products

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the Casbin permission definitions for product endpoints.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/products",
			OperationID: "ListProducts",
			Summary:     "List products",
			Permission:  "products:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/products",
			OperationID: "CreateProduct",
			Summary:     "Create a product",
			Permission:  "products:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/products/{id}",
			OperationID: "GetProduct",
			Summary:     "Get a product by ID",
			Permission:  "products:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/products/{id}",
			OperationID: "PatchProduct",
			Summary:     "Patch a product",
			Permission:  "products:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/products/{id}",
			OperationID: "DeleteProduct",
			Summary:     "Delete a product",
			Permission:  "products:admin",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/products/{id}/buy",
			OperationID: "BuyProduct",
			Summary:     "Buy a product",
			Permission:  "products:buy",
			Surface:     route.SurfaceManagement,
		},
	}
}

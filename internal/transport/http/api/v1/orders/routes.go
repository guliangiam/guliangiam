package orders

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes returns the Casbin permission definitions for order endpoints.
func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/orders",
			OperationID: "ListOrders",
			Summary:     "List orders",
			Permission:  "orders:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/orders",
			OperationID: "CreateOrder",
			Summary:     "Create an order",
			Permission:  "orders:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/orders/{id}",
			OperationID: "GetOrder",
			Summary:     "Get an order by ID",
			Permission:  "orders:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/orders/{id}",
			OperationID: "PatchOrder",
			Summary:     "Patch an order",
			Permission:  "orders:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/orders/{id}",
			OperationID: "DeleteOrder",
			Summary:     "Delete an order",
			Permission:  "orders:admin",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/orders/{id}/pay",
			OperationID: "PayOrder",
			Summary:     "Pay for an order",
			Permission:  "orders:pay",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/orders/{id}/cancel",
			OperationID: "CancelOrder",
			Summary:     "Cancel an order",
			Permission:  "orders:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/orders/place",
			OperationID: "PlaceOrder",
			Summary:     "Place a new order",
			Permission:  "orders:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus:   201,
			ResponseHeaders: []string{"Location"},
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/orders/user-orders",
			OperationID: "GetUserOrders",
			Summary:     "List orders for the current user",
			Permission:  "orders:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/orders/{id}/pay-url",
			OperationID: "GetOrderPayUrl",
			Summary:     "Get payment URL for an order",
			Permission:  "orders:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/orders/{id}/notify-paid",
			OperationID: "NotifyOrderPaid",
			Summary:     "Notify that an order has been paid",
			Permission:  "orders:write",
			Surface:     route.SurfaceManagement,
		},
	}
}

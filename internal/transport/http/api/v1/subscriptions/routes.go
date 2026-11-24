package subscriptions

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{Method: "GET", Pattern: "/api/v1/subscriptions", OperationID: "ListSubscriptions", Summary: "List subscriptions", Permission: "subscriptions:read", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/subscriptions", OperationID: "CreateSubscription", Summary: "Create a subscription", Permission: "subscriptions:write", Surface: route.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/subscriptions/{id}", OperationID: "GetSubscription", Summary: "Get a subscription by ID", Permission: "subscriptions:read", Surface: route.SurfaceManagement},
		{Method: "PATCH", Pattern: "/api/v1/subscriptions/{id}", OperationID: "PatchSubscription", Summary: "Patch a subscription", Permission: "subscriptions:write", Surface: route.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/subscriptions/{id}", OperationID: "DeleteSubscription", Summary: "Delete a subscription", Permission: "subscriptions:admin", Surface: route.SurfaceManagement},
	}
}

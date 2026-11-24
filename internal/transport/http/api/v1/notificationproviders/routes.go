package notificationproviders

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/notification-providers",
			OperationID: "ListNotificationProviders",
			Summary:     "List notification providers",
			Permission:  "notification-providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/notification-providers",
			OperationID: "CreateNotificationProvider",
			Summary:     "Create notification provider",
			Permission:  "notification-providers:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/notification-providers/{id}",
			OperationID: "GetNotificationProvider",
			Summary:     "Get notification provider detail",
			Permission:  "notification-providers:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PATCH",
			Pattern:     "/api/v1/notification-providers/{id}",
			OperationID: "PatchNotificationProvider",
			Summary:     "Patch notification provider",
			Permission:  "notification-providers:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "DELETE",
			Pattern:     "/api/v1/notification-providers/{id}",
			OperationID: "DeleteNotificationProvider",
			Summary:     "Delete notification provider",
			Permission:  "notification-providers:write",
			Surface:     route.SurfaceManagement,
		},
	}
}

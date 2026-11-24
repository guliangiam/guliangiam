package notifications

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/notifications",
			OperationID: "ListNotifications",
			Summary:     "List notifications",
			Permission:  "notifications:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/notifications",
			OperationID: "CreateNotification",
			Summary:     "Create notification",
			Permission:  "notifications:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/notifications/broadcast",
			OperationID: "BroadcastNotification",
			Summary:     "Send an administrator notification to multiple users",
			Permission:  "notifications:write",
			Surface:     route.SurfaceManagement,

			SuccessStatus: 201,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/notifications/{id}/mark-read",
			OperationID: "MarkNotificationRead",
			Summary:     "Mark notification as read",
			Permission:  "notifications:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/notifications/mark-all-read",
			OperationID: "MarkAllNotificationsRead",
			Summary:     "Mark all notifications as read",
			Permission:  "notifications:write",
			Surface:     route.SurfaceManagement,
		},
	}
}

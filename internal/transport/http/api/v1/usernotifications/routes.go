package usernotifications

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

// Routes exposes the authenticated user's own notification center.
// Permission is "public" because authorization is the session principal itself:
// every handler requires a session user and scopes all reads/writes to that
// user. Organization isolation is resolved through TrustedOrganizationID.
func Routes() []route.Definition {
	return []route.Definition{
		{Method: "GET", Pattern: "/api/v1/me/notifications", OperationID: "ListMyNotifications", Summary: "List the authenticated user's notifications", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "GET", Pattern: "/api/v1/me/notifications/unread-count", OperationID: "GetMyUnreadNotificationCount", Summary: "Count unread notifications", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "GET", Pattern: "/api/v1/me/notifications/{id}", OperationID: "GetMyNotification", Summary: "Get one notification", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/me/notifications/{id}/read", OperationID: "MarkMyNotificationRead", Summary: "Mark one notification as read", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/me/notifications/{id}/unread", OperationID: "MarkMyNotificationUnread", Summary: "Mark one notification as unread", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/me/notifications/read-all", OperationID: "MarkAllMyNotificationsRead", Summary: "Mark all notifications as read", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/me/notifications/{id}", OperationID: "DeleteMyNotification", Summary: "Delete one notification", Permission: "public", Surface: route.SurfaceManagement, SuccessStatus: 204},
		{Method: "POST", Pattern: "/api/v1/me/notifications/batch-delete", OperationID: "BatchDeleteMyNotifications", Summary: "Delete multiple notifications", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "GET", Pattern: "/api/v1/me/notifications/{id}/replies", OperationID: "ListMyNotificationReplies", Summary: "List replies on a notification", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "POST", Pattern: "/api/v1/me/notifications/{id}/replies", OperationID: "CreateMyNotificationReply", Summary: "Reply to a notification", Permission: "public", Surface: route.SurfaceManagement, SuccessStatus: 201},
		{Method: "GET", Pattern: "/api/v1/me/notification-preferences", OperationID: "ListMyNotificationPreferences", Summary: "List notification channel preferences", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "PUT", Pattern: "/api/v1/me/notification-preferences", OperationID: "UpdateMyNotificationPreferences", Summary: "Update notification channel preferences", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "GET", Pattern: "/api/v1/me/notification-channel-bindings", OperationID: "ListMyNotificationChannelBindings", Summary: "List external channel bindings", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "PUT", Pattern: "/api/v1/me/notification-channel-bindings", OperationID: "UpsertMyNotificationChannelBinding", Summary: "Bind an external channel identity", Permission: "public", Surface: route.SurfaceManagement},
		{Method: "DELETE", Pattern: "/api/v1/me/notification-channel-bindings/{id}", OperationID: "DeleteMyNotificationChannelBinding", Summary: "Remove an external channel binding", Permission: "public", Surface: route.SurfaceManagement, SuccessStatus: 204},
	}
}

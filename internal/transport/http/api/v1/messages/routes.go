package messages

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "POST",
			Pattern:     "/api/v1/messages/email",
			OperationID: "SendEmail",
			Summary:     "Send email message",
			Permission:  "messages.email",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/messages/sms",
			OperationID: "SendSms",
			Summary:     "Send SMS message",
			Permission:  "messages.sms",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "POST",
			Pattern:     "/api/v1/messages/notification",
			OperationID: "SendNotification",
			Summary:     "Send push notification",
			Permission:  "messages.notification",
			Surface:     route.SurfaceManagement,
		},
	}
}

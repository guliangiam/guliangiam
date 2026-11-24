package notifications

import (
	appnotifications "github.com/guliangiam/guliangiam/internal/app/notifications"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type NotificationResponse = appnotifications.NotificationDTO

type ListNotificationsResponse = presenter.ListResponse[*appnotifications.NotificationDTO]

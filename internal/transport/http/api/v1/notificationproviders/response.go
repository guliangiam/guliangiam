package notificationproviders

import (
	appnotificationproviders "github.com/guliangiam/guliangiam/internal/app/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListNotificationProvidersResponse = presenter.ListResponse[appnotificationproviders.NotificationProviderListItem]
type NotificationProviderResponse = appnotificationproviders.NotificationProviderDetail

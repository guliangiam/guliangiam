package notificationproviders

import appnotificationproviders "github.com/guliangiam/guliangiam/internal/app/notificationproviders"

func NewHandler(service appnotificationproviders.Service) Handler {
	return Handler{Service: service}
}

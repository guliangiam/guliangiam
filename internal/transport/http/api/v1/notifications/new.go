package notifications

import (
	appnotifications "github.com/guliangiam/guliangiam/internal/app/notifications"
	appusernotifications "github.com/guliangiam/guliangiam/internal/app/usernotifications"
)

type Handler struct {
	Service appnotifications.Service
	Inbox   appusernotifications.Service
}

func NewHandler(service appnotifications.Service, inbox appusernotifications.Service) Handler {
	return Handler{Service: service, Inbox: inbox}
}

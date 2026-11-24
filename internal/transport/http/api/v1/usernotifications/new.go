package usernotifications

import appusernotifications "github.com/guliangiam/guliangiam/internal/app/usernotifications"

type Handler struct {
	Service appusernotifications.Service
}

func NewHandler(service appusernotifications.Service) Handler {
	return Handler{Service: service}
}

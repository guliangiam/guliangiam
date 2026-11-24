package sessions

import appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"

func NewHandler(service appsessions.Service) Handler {
	return Handler{Service: service}
}

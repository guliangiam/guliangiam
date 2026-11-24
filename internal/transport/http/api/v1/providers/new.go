package providers

import appproviders "github.com/guliangiam/guliangiam/internal/app/providers"

func NewHandler(service appproviders.Service) Handler {
	return Handler{Service: service}
}

package faceidproviders

import appfaceidproviders "github.com/guliangiam/guliangiam/internal/app/faceidproviders"

func NewHandler(service appfaceidproviders.Service) Handler {
	return Handler{Service: service}
}

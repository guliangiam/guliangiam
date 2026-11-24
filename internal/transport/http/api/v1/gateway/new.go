package gateway

import appgateway "github.com/guliangiam/guliangiam/internal/app/gateway"

func NewHandler(service appgateway.Service) Handler {
	return Handler{
		Service: service,
	}
}

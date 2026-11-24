package radiusconfigs

import appradiusconfigs "github.com/guliangiam/guliangiam/internal/app/radiusconfigs"

func NewHandler(service appradiusconfigs.Service) Handler {
	return Handler{Service: service}
}

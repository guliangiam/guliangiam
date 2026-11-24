package permissions

import apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"

func NewHandler(service apppermissions.Service) Handler {
	return Handler{Service: service}
}

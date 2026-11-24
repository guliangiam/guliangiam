package groups

import (
	appgroups "github.com/guliangiam/guliangiam/internal/app/groups"
)

func NewHandler(service appgroups.Service) Handler {
	return Handler{
		Service: service,
	}
}

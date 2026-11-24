package roles

import approles "github.com/guliangiam/guliangiam/internal/app/roles"

func NewHandler(service approles.Service) Handler {
	return Handler{Service: service}
}

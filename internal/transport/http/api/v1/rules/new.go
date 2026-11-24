package rules

import approles "github.com/guliangiam/guliangiam/internal/app/rules"

func NewHandler(service approles.Service) Handler {
	return Handler{Service: service}
}

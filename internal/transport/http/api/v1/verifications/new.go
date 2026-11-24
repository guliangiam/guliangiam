package verifications

import appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"

func NewHandler(service appverifications.Service) Handler {
	return Handler{
		Service: service,
	}
}

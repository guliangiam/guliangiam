package records

import apprecords "github.com/guliangiam/guliangiam/internal/app/records"

func NewHandler(service apprecords.Service) Handler {
	return Handler{
		Service: service,
	}
}

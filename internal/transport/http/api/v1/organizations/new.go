package organizations

import (
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	apporganizations "github.com/guliangiam/guliangiam/internal/app/organizations"
)

func NewHandler(service apporganizations.Service, applicationService appapplications.Service) Handler {
	return Handler{Service: service, ApplicationService: applicationService}
}

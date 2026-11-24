package organizations

import (
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	apporganizations "github.com/guliangiam/guliangiam/internal/app/organizations"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListOrganizationsResponse = presenter.ListResponse[apporganizations.OrganizationListItem]
type OrganizationResponse = apporganizations.OrganizationDetail
type ListOrganizationApplicationsResponse = presenter.ListResponse[appapplications.ApplicationListItem]

type CreateOrganizationResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
